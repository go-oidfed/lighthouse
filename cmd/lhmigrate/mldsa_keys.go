package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lib/jwx"

	"github.com/go-oidfed/lighthouse/storage"
)

// mldsaKeysCmd re-encodes ML-DSA private keys written by earlier versions of
// go-oidfed/lib (legacy OCTET-STRING-wrapped seed PKCS#8 encoding) in the
// current interoperable RFC 9935 seed-only PKCS#8 encoding, using
// jwx.ConvertMLDSAPEM. Keys in other formats (EC, RSA, EdDSA, ...) are not
// ML-DSA keys and are skipped. Already migrated ML-DSA keys are left
// unchanged (the conversion is idempotent).
func mldsaKeysCmd(args []string) int {
	fs := flag.NewFlagSet("mldsa", flag.ExitOnError)
	var (
		source  string
		dbType  string
		dbDSN   string
		dbDir   string
		typeID  string
		dryRun  bool
		verbose bool
	)
	fs.StringVar(&source, "source", "", "Path to filesystem KMS directory (use this or --db-type)")
	fs.StringVar(&source, "s", "", "Path to filesystem KMS directory (shorthand)")
	fs.StringVar(&dbType, "db-type", "", "Database type for database-stored keys: sqlite|mysql|postgres")
	fs.StringVar(&dbDSN, "db-dsn", "", "Database DSN for mysql/postgres (or sqlite file path)")
	fs.StringVar(&dbDir, "db-dir", "", "Data directory for sqlite database")
	fs.StringVar(&typeID, "type", "federation", "Key type identifier (database table suffix and legacy <type>_<alg>.pem naming)")
	fs.StringVar(&typeID, "t", "federation", "Key type identifier (shorthand)")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview only, don't write changes")
	fs.BoolVar(&dryRun, "n", false, "Preview only (shorthand)")
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"Usage: lhmigrate keys mldsa (--source <kms_dir> | --db-type <sqlite|mysql|postgres> [--db-dir <dir> | --db-dsn <dsn>]) [--type <typeID>] [--dry-run]\n",
		)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	switch {
	case source != "":
		if dbType != "" || dbDSN != "" || dbDir != "" {
			_, _ = fmt.Fprintln(os.Stderr, "--source cannot be combined with database flags")
			fs.Usage()
			return 2
		}
		return migrateMLDSAFilesystem(source, typeID, dryRun)
	case dbType != "":
		return migrateMLDSADatabase(dbType, dbDSN, dbDir, typeID, dryRun)
	default:
		_, _ = fmt.Fprintln(os.Stderr, "either --source or --db-type is required")
		fs.Usage()
		return 2
	}
}

// convertMLDSAPEMIfChanged converts legacy-encoded ML-DSA PEM data. Non-ML-DSA
// key material yields an error naming the source; callers skip it with a
// warning.
func convertMLDSAPEMIfChanged(source string, pemData []byte) (out []byte, err error) {
	out, err = jwx.ConvertMLDSAPEM(pemData)
	if err != nil {
		return nil, errors.Wrapf(err, "%s is not an ML-DSA private key", source)
	}
	return out, nil
}

func migrateMLDSAFilesystem(dir, typeID string, dryRun bool) int {
	info, err := os.Stat(dir)
	if err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("failed to access KMS directory")
		return 1
	}
	if !info.IsDir() {
		log.Error().Str("dir", dir).Msg("--source is not a directory")
		return 1
	}

	// Scan all PEM files in the directory; this covers both the current
	// filesystem KMS layout (<kid>.pem) and the legacy layout
	// (<type>_<alg>.pem, e.g. federation_MLDSA65.pem). Non-ML-DSA keys are
	// skipped.
	pemFiles, err := filepath.Glob(filepath.Join(dir, "*.pem"))
	if err != nil {
		log.Error().Err(err).Msg("failed to scan KMS directory")
		return 1
	}
	if len(pemFiles) == 0 {
		log.Warn().Str("dir", dir).Msg("no PEM files found in KMS directory")
		return 0
	}

	converted, skipped, errorsCount := 0, 0, 0
	for _, file := range pemFiles {
		pemData, err := os.ReadFile(file)
		if err != nil {
			log.Error().Err(err).Str("file", file).Msg("failed to read PEM file")
			errorsCount++
			continue
		}
		out, err := convertMLDSAPEMIfChanged(file, pemData)
		if err != nil {
			log.Info().Str("file", file).Msg("skipping non-ML-DSA key file")
			skipped++
			continue
		}
		if bytes.Equal(out, pemData) {
			log.Info().Str("file", file).Msg("ML-DSA key already in current format")
			continue
		}
		if dryRun {
			log.Info().Str("file", file).Msg("would convert ML-DSA key to RFC 9935 seed-only encoding")
		} else {
			if err = writeFileAtomic(file, out); err != nil {
				log.Error().Err(err).Str("file", file).Msg("failed to write converted ML-DSA key")
				errorsCount++
				continue
			}
			log.Info().Str("file", file).Msg("converted ML-DSA key to RFC 9935 seed-only encoding")
		}
		converted++
	}

	log.Info().
		Int("converted", converted).
		Int("skipped", skipped).
		Int("errors", errorsCount).
		Msg("ML-DSA key migration completed")
	if errorsCount > 0 {
		return 1
	}
	return 0
}

func migrateMLDSADatabase(dbType, dbDSN, dbDir, typeID string, dryRun bool) int {
	driver, err := storage.ParseDriverType(dbType)
	if err != nil {
		log.Error().Err(err).Msg("invalid database type")
		return 1
	}
	cfg := storage.Config{
		Driver:  driver,
		DSN:     dbDSN,
		DataDir: dbDir,
	}
	db, err := storage.Connect(cfg)
	if err != nil {
		log.Error().Err(err).Msg("failed to connect to database")
		return 1
	}
	pemStorer := storage.NewDBPEMStorer(db, typeID)
	entries, err := pemStorer.GetAll()
	if err != nil {
		log.Error().Err(err).Msg("failed to read private keys from database")
		return 1
	}
	if len(entries) == 0 {
		log.Warn().Str("type", typeID).Msg("no private keys found in database")
		return 0
	}

	converted, skipped, errorsCount := 0, 0, 0
	for _, entry := range entries {
		pemData := []byte(entry.PEMData)
		out, err := convertMLDSAPEMIfChanged("key "+entry.KID, pemData)
		if err != nil {
			log.Info().Str("kid", entry.KID).Msg("skipping non-ML-DSA key")
			skipped++
			continue
		}
		if bytes.Equal(out, pemData) {
			log.Info().Str("kid", entry.KID).Msg("ML-DSA key already in current format")
			continue
		}
		if dryRun {
			log.Info().Str("kid", entry.KID).Msg("would convert ML-DSA key to RFC 9935 seed-only encoding")
		} else {
			if err = pemStorer.WritePEM(entry.KID, out); err != nil {
				log.Error().Err(err).Str("kid", entry.KID).Msg("failed to write converted ML-DSA key")
				errorsCount++
				continue
			}
			log.Info().Str("kid", entry.KID).Msg("converted ML-DSA key to RFC 9935 seed-only encoding")
		}
		converted++
	}

	log.Info().
		Int("converted", converted).
		Int("skipped", skipped).
		Int("errors", errorsCount).
		Msg("ML-DSA key migration completed")
	if errorsCount > 0 {
		return 1
	}
	return 0
}

// writeFileAtomic writes data to path via a temp file + rename so a crash
// never leaves a truncated private key on disk.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
