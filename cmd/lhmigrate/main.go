package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"

	"github.com/go-oidfed/lighthouse/storage"
)

func usage() {
	_, _ = fmt.Fprintf(os.Stderr, "lhmigrate: migrate legacy data and keys to new formats\n")
	_, _ = fmt.Fprintf(os.Stderr, "\n")
	_, _ = fmt.Fprintf(os.Stderr, "Subcommands:\n")
	_, _ = fmt.Fprintf(
		os.Stderr, "  all       Run all migration steps in sequence (config, keys, config2db, db, cleanup)\n",
	)
	_, _ = fmt.Fprintf(os.Stderr, "  keys      Migrate signing keys (subcommands: public, kms) [alias: signing]\n")
	_, _ = fmt.Fprintf(os.Stderr, "  config2db Migrate config file values to database\n")
	_, _ = fmt.Fprintf(os.Stderr, "  db        Migrate legacy storage data (JSON/Badger) to GORM-based database\n")
	_, _ = fmt.Fprintf(os.Stderr, "  config    Migrate or update configuration to new format\n")
	_, _ = fmt.Fprintf(os.Stderr, "\n")
	_, _ = fmt.Fprintf(os.Stderr, "Use 'lhmigrate <subcommand> -h' for help on a subcommand.\n")
}

func publicCmd(args []string) int {
	fs := flag.NewFlagSet("public", flag.ExitOnError)
	var (
		src     string
		dst     string
		typeID  string
		destDB  string
		destDSN string
		dbDebug bool
		verbose bool
	)
	// --source / -s
	fs.StringVar(&src, "source", "", "Path to legacy public key storage directory")
	fs.StringVar(&src, "s", "", "Path to legacy public key storage directory (shorthand)")
	// --dest / -d
	fs.StringVar(&dst, "dest", "", "Destination for migrated public key storage (dir for fs, or DB file for sqlite)")
	fs.StringVar(&dst, "d", "", "Destination for migrated public key storage (shorthand)")
	// --type / -t
	fs.StringVar(&typeID, "type", "federation", "Key type identifier (e.g., 'federation')")
	fs.StringVar(&typeID, "t", "federation", "Key type identifier (shorthand)")
	// --db-type
	fs.StringVar(
		&destDB, "db-type", "",
		"Destination database type: sqlite|mysql|postgres (optional; defaults to filesystem if empty)",
	)
	// --db-dsn
	fs.StringVar(&destDSN, "db-dsn", "", "Destination DSN for mysql/postgres (ignored for sqlite)")
	// --db-debug
	fs.BoolVar(&dbDebug, "db-debug", false, "Enable GORM debug logging for DB migration")
	// --verbose / -v
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"Usage: lhmigrate keys public --source <legacy_dir> --dest <dest> --type <typeID> [--db-type=<sqlite|mysql|postgres>] [--db-dsn=<dsn>] [--db-debug]\n",
		)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if verbose {
		log.SetLevel(log.DebugLevel)
	}
	if src == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--source is required")
		fs.Usage()
		return 2
	}
	if dst == "" {
		dst = src
	}
	if typeID == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--type is required")
		fs.Usage()
		return 2
	}
	log.WithFields(
		log.Fields{
			"source": src,
			"dest":   dst,
			"type":   typeID,
		},
	).Info("migrating public key storage")

	// Build source legacy storage wrapper
	legacy := &public.LegacyPublicKeyStorage{
		Dir:    src,
		TypeID: typeID,
	}
	if err := legacy.Load(); err != nil {
		log.WithError(err).Error("failed to load legacy public key storage")
		return 1
	}

	// Destination selection: filesystem (default) or DB
	if strings.TrimSpace(destDB) == "" {
		// Filesystem destination
		if _, err := public.NewFilesystemPublicKeyStorageFromStorage(dst, typeID, legacy); err != nil {
			log.WithError(err).Error("public key migration failed")
			return 1
		}
	} else {
		// Database destination
		var driver storage.DriverType
		switch strings.ToLower(strings.TrimSpace(destDB)) {
		case string(storage.DriverSQLite):
			driver = storage.DriverSQLite
		case string(storage.DriverMySQL):
			driver = storage.DriverMySQL
		case string(storage.DriverPostgres):
			driver = storage.DriverPostgres
		default:
			_, _ = fmt.Fprintf(os.Stderr, "invalid --db-type: %s\n", destDB)
			return 2
		}
		cfg := storage.Config{
			Driver:  driver,
			DSN:     destDSN,
			DataDir: dst,
			Debug:   dbDebug,
		}
		db, err := storage.Connect(cfg)
		if err != nil {
			log.WithError(err).Error("failed to connect to destination database")
			return 1
		}
		if _, err = storage.NewDBPublicKeyStorageFromStorage(db, typeID, legacy); err != nil {
			log.WithError(err).Error("public key migration to DB failed")
			return 1
		}
	}
	log.Info("public key migration completed")
	return 0
}

func parseAlgs(list string) ([]jwa.SignatureAlgorithm, error) {
	if strings.TrimSpace(list) == "" {
		return nil, nil
	}
	parts := strings.Split(list, ",")
	out := make([]jwa.SignatureAlgorithm, 0, len(parts))
	for _, p := range parts {
		a := strings.TrimSpace(p)
		if a == "" {
			continue
		}
		alg, found := jwa.LookupSignatureAlgorithm(a)
		if !found {
			return nil, errors.Errorf("invalid algorithm '%s'", a)
		}
		out = append(out, alg)
	}
	return out, nil
}

func kmsCmd(args []string) int {
	fs := flag.NewFlagSet("kms", flag.ExitOnError)
	var (
		src      string
		dst      string
		typeID   string
		algsStr  string
		defAlg   string
		generate bool
		rsaLen   int
		pksType  string
		destDB   string
		destDSN  string
		dbDebug  bool
		verbose  bool
	)
	// --source / -s
	fs.StringVar(&src, "source", "", "Path to legacy key files directory (containing <type>_<alg>.pem)")
	fs.StringVar(&src, "s", "", "Path to legacy key files directory (shorthand)")
	// --dest / -d
	fs.StringVar(&dst, "dest", "", "Destination directory for filesystem KMS (and public storage if --pks-type=fs)")
	fs.StringVar(&dst, "d", "", "Destination directory (shorthand)")
	// --type / -t
	fs.StringVar(&typeID, "type", "federation", "Key type identifier (e.g., 'federation')")
	fs.StringVar(&typeID, "t", "federation", "Key type identifier (shorthand)")
	// --algs / -a
	fs.StringVar(&algsStr, "algs", "", "Comma-separated list of algorithms to migrate (e.g., ES256,RS256)")
	fs.StringVar(&algsStr, "a", "", "Comma-separated list of algorithms (shorthand)")
	// --default
	fs.StringVar(&defAlg, "default", "", "Default algorithm (optional)")
	// --generate-missing / -g
	fs.BoolVar(&generate, "generate-missing", false, "Generate missing keys in destination if not present")
	fs.BoolVar(&generate, "g", false, "Generate missing keys (shorthand)")
	// --rsa-len
	fs.IntVar(&rsaLen, "rsa-len", 4096, "RSA key length when generating (if enabled)")
	// --pks-type (required)
	fs.StringVar(&pksType, "pks-type", "", "Public key storage type: fs (filesystem) or db (database) [required]")
	// --db-type
	fs.StringVar(
		&destDB, "db-type", "",
		"Database type for public key storage: sqlite|mysql|postgres (required when --pks-type=db)",
	)
	// --db-dsn
	fs.StringVar(&destDSN, "db-dsn", "", "Database DSN for mysql/postgres (ignored for sqlite)")
	// --db-debug
	fs.BoolVar(&dbDebug, "db-debug", false, "Enable GORM debug logging for DB operations")
	// --verbose / -v
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"Usage: lhmigrate keys kms --source <legacy_dir> --dest <dest_dir> --type <typeID> --algs <list> --pks-type <fs|db> [options]\n",
		)
		_, _ = fmt.Fprintf(os.Stderr, "\nPublic key storage options:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  For filesystem:  --pks-type=fs\n")
		_, _ = fmt.Fprintf(
			os.Stderr, "  For database:    --pks-type=db --db-type=<sqlite|mysql|postgres> [--db-dsn=<dsn>]\n\n",
		)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if verbose {
		log.SetLevel(log.DebugLevel)
	}
	if src == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--source is required")
		fs.Usage()
		return 2
	}
	if dst == "" {
		dst = src
	}
	if typeID == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--type is required")
		fs.Usage()
		return 2
	}
	algs, err := parseAlgs(algsStr)
	if err != nil || len(algs) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "--algs is required and must be a comma-separated list (e.g., ES256,RS256)")
		fs.Usage()
		return 2
	}
	// Validate --pks-type
	pksType = strings.ToLower(strings.TrimSpace(pksType))
	if pksType == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--pks-type is required (fs or db)")
		fs.Usage()
		return 2
	}
	if pksType != "fs" && pksType != "db" {
		_, _ = fmt.Fprintf(os.Stderr, "invalid --pks-type: %s (must be 'fs' or 'db')\n", pksType)
		fs.Usage()
		return 2
	}
	// Validate DB options when --pks-type=db
	var dbDriver storage.DriverType
	if pksType == "db" {
		destDB = strings.ToLower(strings.TrimSpace(destDB))
		if destDB == "" {
			_, _ = fmt.Fprintln(os.Stderr, "--db-type is required when --pks-type=db")
			fs.Usage()
			return 2
		}
		dbDriver, err = storage.ParseDriverType(destDB)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "invalid --db-type: %s\n", err)
			return 2
		}
		if err := validateDBFlags(dbDriver, dst, destDSN); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	var defaultAlg jwa.SignatureAlgorithm
	if a := strings.TrimSpace(defAlg); a != "" {
		alg, found := jwa.LookupSignatureAlgorithm(a)
		if !found {
			_, _ = fmt.Fprintf(os.Stderr, "invalid --default algorithm: %s\n", a)
			return 2
		}
		defaultAlg = alg
	}
	log.WithFields(
		log.Fields{
			"source":   src,
			"dest":     dst,
			"type":     typeID,
			"algs":     algsStr,
			"default":  defaultAlg.String(),
			"generate": generate,
			"pks-type": pksType,
		},
	).Info("migrating KMS")

	// Prepare legacy KMS source
	legacyKMS := &kms.LegacyFilesystemKMS{
		Dir:    src,
		TypeID: typeID,
		Algs:   algs,
	}
	if err = legacyKMS.Load(); err != nil {
		log.WithError(err).Error("failed to load legacy KMS")
		return 1
	}

	// Prepare legacy public key storage source
	legacyPKS := &public.LegacyPublicKeyStorage{
		Dir:    src,
		TypeID: typeID,
	}

	// Prepare destination public key storage based on --pks-type
	var dstPKS public.PublicKeyStorage
	if pksType == "fs" {
		dstPKS, err = public.NewFilesystemPublicKeyStorageFromStorage(dst, typeID, legacyPKS)
		if err != nil {
			log.WithError(err).Error("failed to migrate public key storage for KMS (filesystem)")
			return 1
		}
	} else {
		// pksType == "db"
		dbCfg := storage.Config{
			Driver:  dbDriver,
			DSN:     destDSN,
			DataDir: dst,
			Debug:   dbDebug,
		}
		db, dbErr := storage.Connect(dbCfg)
		if dbErr != nil {
			log.WithError(dbErr).Error("failed to connect to destination database for public key storage")
			return 1
		}
		dstPKS, err = storage.NewDBPublicKeyStorageFromStorage(db, typeID, legacyPKS)
		if err != nil {
			log.WithError(err).Error("failed to migrate public key storage for KMS (database)")
			return 1
		}
	}

	// Configure destination filesystem KMS
	cfg := kms.FilesystemKMSConfig{
		KMSConfig: kms.KMSConfig{
			GenerateKeys: generate,
			Algs:         algs,
			DefaultAlg:   defaultAlg,
			RSAKeyLen:    rsaLen,
			// KeyRotation not needed for migration
		},
		Dir:    dst,
		TypeID: typeID,
	}

	if _, err = kms.NewFilesystemKMSFromBasic(legacyKMS, cfg, dstPKS); err != nil {
		log.WithError(err).Error("KMS migration failed")
		return 1
	}
	log.Info("KMS migration completed")
	return 0
}

// keysCmd dispatches to key-related subcommands (public, kms).
func keysCmd(args []string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lhmigrate keys <public|kms> [options]\n")
		_, _ = fmt.Fprintf(
			os.Stderr,
			"\nSubcommands:\n  public   Migrate legacy public key storage (keys.jwks + history) to filesystem or DB\n  kms      Migrate legacy private key files (<type>_<alg>.pem) to filesystem KMS\n",
		)
		return 2
	}
	sub := args[0]
	switch sub {
	case "public":
		return publicCmd(args[1:])
	case "kms":
		return kmsCmd(args[1:])
	case "-h", "--help", "help":
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lhmigrate keys <public|kms> [options]\n")
		_, _ = fmt.Fprintf(
			os.Stderr,
			"\nSubcommands:\n  public   Migrate legacy public key storage (keys.jwks + history) to filesystem or DB\n  kms      Migrate legacy private key files (<type>_<alg>.pem) to filesystem KMS\n",
		)
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown keys subcommand: %s\n", sub)
		_, _ = fmt.Fprintf(os.Stderr, "Use 'lhmigrate keys <public|kms> -h' for help.\n")
		return 2
	}
}

// dbCmd delegates to the database migration implementation in db.go
func dbCmd(args []string) int {
	return runDBMigration(args)
}

// configCmd delegates to the config transformation implementation in config.go
func configCmd(args []string) int {
	return runConfigMigration(args)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	sub := os.Args[1]
	var code int
	switch sub {
	case "all":
		code = allCmd(os.Args[2:])
	case "keys", "signing":
		code = keysCmd(os.Args[2:])
	case "config2db":
		code = config2dbCmd(os.Args[2:])
	case "db":
		code = dbCmd(os.Args[2:])
	case "config":
		code = configCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		code = 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", sub)
		usage()
		code = 2
	}
	os.Exit(code)
}
