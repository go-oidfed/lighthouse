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
)

func usage() {
	_, _ = fmt.Fprintf(os.Stderr, "lhmigrate: migrate legacy data and keys to new formats\n")
	_, _ = fmt.Fprintf(os.Stderr, "\n")
	_, _ = fmt.Fprintf(os.Stderr, "Subcommands:\n")
	_, _ = fmt.Fprintf(os.Stderr, "  keys     Migrate signing keys (subcommands: public, kms) [alias: signing]\n")
	_, _ = fmt.Fprintf(os.Stderr, "  db       Migrate legacy storage data to GORM-based database (NOT IMPLEMENTED)\n")
	_, _ = fmt.Fprintf(os.Stderr, "  config   Migrate or update configuration to new format (NOT IMPLEMENTED)\n")
	_, _ = fmt.Fprintf(os.Stderr, "\n")
	_, _ = fmt.Fprintf(os.Stderr, "Use 'lhmigrate <subcommand> -h' for help on a subcommand.\n")
}

func publicCmd(args []string) int {
	fs := flag.NewFlagSet("public", flag.ExitOnError)
	var (
		src    = fs.String("src", "", "Path to legacy public key storage directory")
		dst    = fs.String("dst", "", "Destination directory for filesystem public key storage")
		typeID = fs.String(
			"type", "federation", "Key type identifier (e.g., "+
				"'federation')",
		)
		v = fs.Bool("v", false, "Verbose logging")
	)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lhmigrate keys public -src <legacy_dir> -dst <dest_dir> -type <typeID>\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *v {
		log.SetLevel(log.DebugLevel)
	}
	if *src == "" {
		_, _ = fmt.Fprintln(os.Stderr, "-src is required")
		fs.Usage()
		return 2
	}
	if *dst == "" {
		*dst = *src
	}
	if *typeID == "" {
		_, _ = fmt.Fprintln(os.Stderr, "-type is required")
		fs.Usage()
		return 2
	}
	log.WithFields(
		log.Fields{
			"src":  *src,
			"dst":  *dst,
			"type": *typeID,
		},
	).Info("migrating public key storage")

	// Build source legacy storage wrapper
	legacy := &public.LegacyPublicKeyStorage{
		Dir:    *src,
		TypeID: *typeID,
	}
	if err := legacy.Load(); err != nil {
		log.WithError(err).Error("failed to load legacy public key storage")
		return 1
	}

	// Create destination filesystem storage and populate from legacy
	if _, err := public.NewFilesystemPublicKeyStorageFromStorage(*dst, *typeID, legacy); err != nil {
		log.WithError(err).Error("public key migration failed")
		return 1
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
		src      = fs.String("src", "", "Path to legacy key files directory (containing <type>_<alg>.pem)")
		dst      = fs.String("dst", "", "Destination directory for filesystem KMS and public storage")
		typeID   = fs.String("type", "federation", "Key type identifier (e.g., 'federation')")
		algsStr  = fs.String("algs", "", "Comma-separated list of algorithms to migrate (e.g., ES256,RS256)")
		defAlg   = fs.String("default", "", "Default algorithm (optional)")
		generate = fs.Bool("generate-missing", false, "Generate missing keys in destination if not present")
		rsaLen   = fs.Int("rsa-len", 4096, "RSA key length when generating (if enabled)")
		v        = fs.Bool("v", false, "Verbose logging")
	)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(
			os.Stderr, "Usage: lhmigrate keys kms -src <legacy_dir> -dst <dest_dir> -type <typeID> -algs <list> [options]\n",
		)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *v {
		log.SetLevel(log.DebugLevel)
	}
	if *src == "" {
		_, _ = fmt.Fprintln(os.Stderr, "-src is required")
		fs.Usage()
		return 2
	}
	if *dst == "" {
		*dst = *src
	}
	if *typeID == "" {
		_, _ = fmt.Fprintln(os.Stderr, "-type is required")
		fs.Usage()
		return 2
	}
	algs, err := parseAlgs(*algsStr)
	if err != nil || len(algs) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "-algs is required and must be a comma-separated list (e.g., ES256,RS256)")
		fs.Usage()
		return 2
	}
	var defaultAlg jwa.SignatureAlgorithm
	if a := strings.TrimSpace(*defAlg); a != "" {
		alg, found := jwa.LookupSignatureAlgorithm(a)
		if !found {
			_, _ = fmt.Fprintf(os.Stderr, "invalid -default algorithm: %s\n", a)
			return 2
		}
		defaultAlg = alg
	}
	log.WithFields(
		log.Fields{
			"src":      *src,
			"dst":      *dst,
			"type":     *typeID,
			"algs":     *algsStr,
			"default":  defaultAlg.String(),
			"generate": *generate,
		},
	).Info("migrating KMS")

	// Prepare legacy KMS source
	legacyKMS := &kms.LegacyFilesystemKMS{
		Dir:    *src,
		TypeID: *typeID,
		Algs:   algs,
	}
	if err = legacyKMS.Load(); err != nil {
		log.WithError(err).Error("failed to load legacy KMS")
		return 1
	}

	// Prepare destination public key storage (migrated from legacy public store at src)
	dstPKS, err := public.NewFilesystemPublicKeyStorageFromStorage(
		*dst, *typeID, &public.LegacyPublicKeyStorage{
			Dir:    *src,
			TypeID: *typeID,
		},
	)
	if err != nil {
		log.WithError(err).Error("failed to migrate public key storage for KMS")
		return 1
	}

	// Configure destination filesystem KMS
	cfg := kms.FilesystemKMSConfig{
		KMSConfig: kms.KMSConfig{
			GenerateKeys: *generate,
			Algs:         algs,
			DefaultAlg:   defaultAlg,
			RSAKeyLen:    *rsaLen,
			// KeyRotation not needed for migration
		},
		Dir:    *dst,
		TypeID: *typeID,
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
		_, _ = fmt.Fprintf(os.Stderr, "\nSubcommands:\n  public   Migrate legacy public key storage (keys.jwks + history)\n  kms      Migrate legacy private key files (<type>_<alg>.pem) to filesystem KMS\n")
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
		_, _ = fmt.Fprintf(os.Stderr, "\nSubcommands:\n  public   Migrate legacy public key storage (keys.jwks + history)\n  kms      Migrate legacy private key files (<type>_<alg>.pem) to filesystem KMS\n")
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown keys subcommand: %s\n", sub)
		_, _ = fmt.Fprintf(os.Stderr, "Use 'lhmigrate keys <public|kms> -h' for help.\n")
		return 2
	}
}

// dbCmd is a stub for future database migration support.
// It currently parses common flags and reports that the feature is not implemented.
func dbCmd(args []string) int {
	fs := flag.NewFlagSet("db", flag.ExitOnError)
	var (
		srcType  = fs.String("source-type", "", "Source storage type (e.g., json, badger)")
		srcDir   = fs.String("source-dir", "", "Source data directory")
		destType = fs.String("dest-type", "", "Destination database type (e.g., sqlite, mysql, postgres)")
		destDir  = fs.String("dest-dir", "", "Destination data directory (for sqlite)")
		destDSN  = fs.String("dest-dsn", "", "Destination DSN (for mysql/postgres)")
		dryRun   = fs.Bool("dry-run", false, "Perform a dry run without writing to destination")
		v        = fs.Bool("v", false, "Verbose logging")
	)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lhmigrate db --source-type=<json|badger> --source-dir=<dir> --dest-type=<sqlite|mysql|postgres> [--dest-dir=<dir>|--dest-dsn=<dsn>] [--dry-run] [--v]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *v {
		log.SetLevel(log.DebugLevel)
	}
	// Minimal validation to help users before we return the placeholder error.
	if *srcType == "" || *srcDir == "" || *destType == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--source-type, --source-dir, and --dest-type are required")
		fs.Usage()
		return 2
	}
	// Log intent and return not implemented.
	log.WithFields(log.Fields{
		"source-type": *srcType,
		"source-dir":  *srcDir,
		"dest-type":   *destType,
		"dest-dir":    *destDir,
		"dest-dsn":    *destDSN,
		"dry-run":     *dryRun,
	}).Info("db migration requested")
	_, _ = fmt.Fprintln(os.Stderr, "db migration is not implemented yet")
	return 3
}

// configCmd is a stub for future configuration migration support.
// It currently parses common flags and reports that the feature is not implemented.
func configCmd(args []string) int {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	var (
		in  = fs.String("in", "", "Path to existing configuration file")
		out = fs.String("out", "", "Path to write updated configuration")
		v   = fs.Bool("v", false, "Verbose logging")
	)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lhmigrate config --in=<config.yaml> [--out=<updated.yaml>] [--v]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *v {
		log.SetLevel(log.DebugLevel)
	}
	if *in == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--in is required")
		fs.Usage()
		return 2
	}
	log.WithFields(log.Fields{
		"in":  *in,
		"out": *out,
	}).Info("config migration requested")
	_, _ = fmt.Fprintln(os.Stderr, "config migration is not implemented yet")
	return 3
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	sub := os.Args[1]
	var code int
	switch sub {
	case "keys", "signing":
		code = keysCmd(os.Args[2:])
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
