package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

var (
	sourceType         string
	sourceDir          string
	destType           string
	destDir            string
	destDSN            string
	verbose            bool
	dryRun             bool
	sourceSubordinates loadLegacySubordinateInfos
	sourceTrustMark    storage.TrustMarkedEntitiesStorageBackend
	destBackend        *storage.Storage
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "lhmigrate",
		Short: "Lighthouse Storage Migration Tool",
		Long:  "A tool to migrate data between different Lighthouse storage backends",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
	}

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data from one storage backend to another",
		Long:  "Migrate data from one storage backend to another (json/badger to gorm)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate source type
			if sourceType != "json" && sourceType != "badger" {
				return errors.New("source-type must be either 'json' or 'badger'")
			}

			destDriver := storage.DriverType(destType)
			// Validate destination type
			switch destDriver {
			case storage.DriverSQLite, storage.DriverMySQL, storage.DriverPostgres:
				break
			default:
				return errors.Errorf("dest-type must be one of %+v", storage.SupportedDrivers)
			}

			// Validate source directory
			if sourceDir == "" {
				return errors.New("source-dir is required")
			}

			// Validate destination directory for SQLite
			if destDriver == storage.DriverSQLite && destDir == "" && destDSN == "" {
				return errors.New("dest-dir is required for sqlite")
			}

			// Validate DSN for MySQL and PostgreSQL
			if (destDriver == storage.DriverMySQL || destDriver == storage.DriverPostgres) && destDSN == "" {
				return errors.New("dest-dsn is required for mysql and postgres")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize source storage
			var err error
			switch sourceType {
			case string(config.BackendTypeJSON):
				warehouse := NewFileStorage(sourceDir)
				sourceSubordinates = warehouse.SubordinateStorage()
				sourceTrustMark = warehouse.TrustMarkedEntitiesStorage()
			case string(config.BackendTypeBadger):
				warehouse, err := NewBadgerStorage(sourceDir)
				if err != nil {
					return errors.Wrap(err, "failed to initialize badger storage")
				}
				sourceSubordinates = warehouse.SubordinateStorage()
				sourceTrustMark = warehouse.TrustMarkedEntitiesStorage()
			}

			// Load source storage
			subordinates, err := sourceSubordinates()
			if err != nil {
				return errors.Wrap(err, "failed to load source subordinate storage")
			}
			if err = sourceTrustMark.Load(); err != nil {
				return errors.Wrap(err, "failed to load source trust mark storage")
			}

			// Initialize destination storage
			if !dryRun {
				dbConfig := storage.Config{
					Driver:  storage.DriverType(destType),
					DSN:     destDSN,
					DataDir: destDir,
					Debug:   verbose,
				}

				destBackend, err = storage.NewStorage(dbConfig)
				if err != nil {
					return errors.Wrap(err, "failed to initialize destination storage")
				}
			}

			// Perform migration
			if err = migrateSubordinates(subordinates); err != nil {
				return errors.Wrap(err, "failed to migrate subordinates")
			}

			if err = migrateTrustMarks(); err != nil {
				return errors.Wrap(err, "failed to migrate trust marks")
			}

			fmt.Println("Migration completed successfully!")
			return nil
		},
	}

	// Add flags to migrate command
	migrateCmd.Flags().StringVar(&sourceType, "source-type", "", "Source storage type (json or badger)")
	migrateCmd.Flags().StringVar(&sourceDir, "source-dir", "", "Source data directory")
	migrateCmd.Flags().StringVar(
		&destType, "dest-type", "sqlite", "Destination database type (sqlite, mysql, or postgres)",
	)
	migrateCmd.Flags().StringVar(&destDir, "dest-dir", "", "Destination data directory (for sqlite)")
	migrateCmd.Flags().StringVar(&destDSN, "dest-dsn", "", "Destination DSN (for mysql and postgres)")
	migrateCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Perform a dry run without writing to destination")

	// Mark required flags
	migrateCmd.MarkFlagRequired("source-type")
	migrateCmd.MarkFlagRequired("source-dir")

	rootCmd.AddCommand(migrateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func migrateSubordinates(subordinates []legacySubordinateInfo) error {
	fmt.Printf("Found %d subordinates\n", len(subordinates))
	for _, sub := range subordinates {
		fmt.Printf("Migrating subordinate: %s\n", sub.EntityID)
		if !dryRun {
			if err := destBackend.SubordinateStorage().Write(
				sub.EntityID, model.SubordinateInfo{
					Entity: model.Entity{
						EntityID:    sub.EntityID,
						EntityTypes: model.NewEntityTypes(sub.EntityTypes),
					},
					JWKS:               model.NewJWKS(sub.JWKS),
					Metadata:           sub.Metadata,
					MetadataPolicy:     sub.MetadataPolicy,
					Constraints:        sub.Constraints,
					CriticalExtensions: model.NewCritExtensions(sub.CriticalExtensions),
					MetadataPolicyCrit: model.NewPolicyOperators(sub.MetadataPolicyCrit),
					Extra:              sub.Extra,
					Status:             sub.Status,
				},
			); err != nil {
				return errors.Wrapf(err, "failed to write subordinate %s", sub.EntityID)
			}
		}
	}
	return nil
}

func migrateTrustMarks() error {
	// Get all trust mark types
	// This is a bit tricky since we don't have a direct way to get all trust mark types
	// We'll use a workaround by checking the source directory for JSON files
	trustMarkTypes := []string{}

	if sourceType == "json" {
		// For JSON storage, we can look at the trust_marks directory
		trustMarksDir := filepath.Join(sourceDir, "trust_marks")
		if _, err := os.Stat(trustMarksDir); err == nil {
			entries, err := os.ReadDir(trustMarksDir)
			if err != nil {
				return errors.Wrap(err, "failed to read trust_marks directory")
			}

			for _, entry := range entries {
				if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
					trustMarkType := filepath.Base(entry.Name())
					trustMarkType = trustMarkType[:len(trustMarkType)-5] // Remove .json extension
					trustMarkTypes = append(trustMarkTypes, trustMarkType)
				}
			}
		}
	} else {
		// For Badger, we'll need to use an empty string to get all trust marks
		trustMarkTypes = append(trustMarkTypes, "")
	}

	// If we couldn't find any trust mark types, use an empty string to get all
	if len(trustMarkTypes) == 0 {
		trustMarkTypes = append(trustMarkTypes, "")
	}

	// Process each trust mark type
	for _, trustMarkType := range trustMarkTypes {
		// Get active entities
		activeEntities, err := sourceTrustMark.Active(trustMarkType)
		if err != nil {
			return errors.Wrapf(err, "failed to get active entities for trust mark %s", trustMarkType)
		}

		fmt.Printf("Found %d active entities for trust mark type %s\n", len(activeEntities), trustMarkType)
		for _, entityID := range activeEntities {
			fmt.Printf("Migrating active trust mark: %s for entity %s\n", trustMarkType, entityID)
			if !dryRun {
				if err := destBackend.TrustMarkedEntitiesStorage().Approve(trustMarkType, entityID); err != nil {
					return errors.Wrapf(err, "failed to approve trust mark %s for entity %s", trustMarkType, entityID)
				}
			}
		}

		// Get blocked entities
		blockedEntities, err := sourceTrustMark.Blocked(trustMarkType)
		if err != nil {
			return errors.Wrapf(err, "failed to get blocked entities for trust mark %s", trustMarkType)
		}

		fmt.Printf("Found %d blocked entities for trust mark type %s\n", len(blockedEntities), trustMarkType)
		for _, entityID := range blockedEntities {
			fmt.Printf("Migrating blocked trust mark: %s for entity %s\n", trustMarkType, entityID)
			if !dryRun {
				if err := destBackend.TrustMarkedEntitiesStorage().Block(trustMarkType, entityID); err != nil {
					return errors.Wrapf(err, "failed to block trust mark %s for entity %s", trustMarkType, entityID)
				}
			}
		}

		// Get pending entities
		pendingEntities, err := sourceTrustMark.Pending(trustMarkType)
		if err != nil {
			return errors.Wrapf(err, "failed to get pending entities for trust mark %s", trustMarkType)
		}

		fmt.Printf("Found %d pending entities for trust mark type %s\n", len(pendingEntities), trustMarkType)
		for _, entityID := range pendingEntities {
			fmt.Printf("Migrating pending trust mark: %s for entity %s\n", trustMarkType, entityID)
			if !dryRun {
				if err := destBackend.TrustMarkedEntitiesStorage().Request(trustMarkType, entityID); err != nil {
					return errors.Wrapf(err, "failed to request trust mark %s for entity %s", trustMarkType, entityID)
				}
			}
		}
	}

	return nil
}
