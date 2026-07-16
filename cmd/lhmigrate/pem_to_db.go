package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// pemToDBCmd migrates filesystem KMS private keys to database storage.
func pemToDBCmd(args []string) int {
	fs := flag.NewFlagSet("pem-to-db", flag.ExitOnError)
	var (
		source  string
		typeID  string
		destDSN string
		dbType  string
		verbose bool
	)

	fs.StringVar(&source, "source", "", "Path to legacy filesystem KMS directory")
	fs.StringVar(&source, "s", "", "Path to legacy filesystem KMS directory (shorthand)")
	fs.StringVar(&typeID, "type", "federation", "Key type identifier (e.g., 'federation')")
	fs.StringVar(&typeID, "t", "federation", "Key type identifier (shorthand)")
	fs.StringVar(&dbType, "db-type", "sqlite", "Database type: sqlite|mysql|postgres")
	fs.StringVar(&destDSN, "db-dsn", "", "Database DSN (file path for sqlite, connection string for mysql/postgres)")
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"Usage: lhmigrate pem-to-db --source <kms_dir> --type <typeID> --db-dsn <dsn> [--db-type=<type>]\n",
		)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if source == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--source is required")
		fs.Usage()
		return 2
	}

	if destDSN == "" {
		_, _ = fmt.Fprintln(os.Stderr, "--db-dsn is required")
		fs.Usage()
		return 2
	}

	log.Info().
		Str("source", source).
		Str("type", typeID).
		Str("db", dbType).
		Str("dsn", destDSN).
		Msg("migrating PEM private keys to database")

	// Initialize database connection
	driver, err := storage.ParseDriverType(dbType)
	if err != nil {
		log.Error().Err(err).Msg("invalid database type")
		return 1
	}

	cfg := storage.Config{
		Driver: driver,
		DSN:    destDSN,
		Debug:  verbose,
	}
	db, err := storage.Connect(cfg)
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize database")
		return 1
	}

	// Initialize DBPEMStorer and ensure table exists
	pemStorer := storage.NewDBPEMStorer(db, typeID)
	if err = pemStorer.Load(); err != nil {
		log.Error().Err(err).Msg("failed to initialize private_keys table")
		return 1
	}

	// Initialize KV storage for state migration
	kvStorage := &keyValueStorageForMigration{db: db}

	// Scan source directory for PEM files
	pemFiles, err := filepath.Glob(filepath.Join(source, "*.pem"))
	if err != nil {
		log.Error().Err(err).Msg("failed to scan source directory")
		return 1
	}

	if len(pemFiles) == 0 {
		log.Warn().Msg("no PEM files found in source directory")
	}

	migrated := 0
	for _, pemFile := range pemFiles {
		filename := filepath.Base(pemFile)
		kid := strings.TrimSuffix(filename, ".pem")

		pemData, err := os.ReadFile(pemFile)
		if err != nil {
			log.Warn().Err(err).Str("file", pemFile).Msg("failed to read PEM file")
			continue
		}

		if err = pemStorer.WritePEM(kid, pemData); err != nil {
			log.Error().Err(err).Str("kid", kid).Msg("failed to write PEM to database")
			continue
		}

		log.Info().Str("kid", kid).Msg("migrated private key")
		migrated++
	}

	// Migrate KMS state file if it exists
	stateFile := filepath.Join(source, "kms_state.json")
	if _, err = os.Stat(stateFile); err == nil {
		log.Info().Msg("migrating KMS state file")
		stateData, err := os.ReadFile(stateFile)
		if err != nil {
			log.Error().Err(err).Msg("failed to read KMS state file")
			return 1
		}

		var state kms.ScheduledState
		if err = json.Unmarshal(stateData, &state); err != nil {
			log.Error().Err(err).Msg("failed to parse KMS state file")
			return 1
		}

		stateStorer := storage.NewDBStateStorer(kvStorage, typeID)
		if err = stateStorer.SaveScheduledState(state); err != nil {
			log.Error().Err(err).Msg("failed to save KMS state to database")
			return 1
		}

		log.Info().Msg("migrated KMS state")
	} else if !os.IsNotExist(err) {
		log.Warn().Err(err).Msg("error checking for KMS state file")
	}

	log.Info().Int("migrated", migrated).Msg("migration completed: private keys migrated")
	log.Info().Msg("\nTo use the database-backed KMS, update your configuration:")
	log.Info().Msg("  signing:")
	log.Info().Msg("    kms: db")
	log.Info().Msg("    pk_backend: db")
	log.Info().Msg("    auto_generate_keys: true")

	return 0
}

// keyValueStorageForMigration is a simple wrapper for migration purposes
type keyValueStorageForMigration struct {
	db *gorm.DB
}

func (k *keyValueStorageForMigration) Get(scope, key string) (datatypes.JSON, error) {
	var raw []byte
	row := k.db.Model(&model.KeyValue{}).
		Select("value").
		Where(
			&model.KeyValue{
				Scope: scope,
				Key:   key,
			},
		).
		Row()
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return raw, nil
}

func (k *keyValueStorageForMigration) GetAs(scope, key string, out any) (bool, error) {
	val, err := k.Get(scope, key)
	if err != nil || val == nil {
		return false, err
	}
	err = json.Unmarshal(val, out)
	return err == nil && len(val) > 0, err
}

func (k *keyValueStorageForMigration) Set(scope, key string, value datatypes.JSON) error {
	kv := model.KeyValue{
		Scope: scope,
		Key:   key,
		Value: value,
	}
	return k.db.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "scope"},
				{Name: "key"},
			},
			DoUpdates: clause.AssignmentColumns(
				[]string{
					"value",
					"updated_at",
				},
			),
		},
	).Create(&kv).Error
}

func (k *keyValueStorageForMigration) SetAny(scope, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return k.Set(scope, key, data)
}

func (k *keyValueStorageForMigration) Delete(scope, key string) error {
	return k.db.Where(
		&model.KeyValue{
			Scope: scope,
			Key:   key,
		},
	).Delete(&model.KeyValue{}).Error
}

// runPEMMigration is the entry point for PEM-to-DB migration from main.go
func runPEMMigration(args []string) int {
	return pemToDBCmd(args)
}
