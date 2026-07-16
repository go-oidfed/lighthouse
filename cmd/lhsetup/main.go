package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/internal/migration"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

var configFile string
var dbType string
var dbDSN string
var dbDir string
var dbDebug bool
var onlyFlag string
var skipFlag string

var backends model.Backends
var migrationCfg *migration.Config

var rootCmd = &cobra.Command{
	Use:   "lhsetup",
	Short: "Interactive setup utility for Lighthouse DB-managed configuration",
	Long: `lhsetup interactively prompts for all DB-managed configuration options
and writes them to the database. It can also display current values.

If a config file is provided, values from the config file are used to
prepopulate prompts. Existing DB values always take precedence as defaults.`,
	RunE: rootRunE,
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display all current DB-managed configuration values (read-only)",
	RunE:  showRunE,
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "path to config file (optional; auto-discovered if not specified)")
	rootCmd.Flags().StringVar(&dbType, "db-type", "", "override database type: sqlite|mysql|postgres")
	rootCmd.Flags().StringVar(&dbDSN, "db-dsn", "", "override database DSN (for mysql/postgres)")
	rootCmd.Flags().StringVar(&dbDir, "db-dir", "", "override data directory (for sqlite)")
	rootCmd.Flags().BoolVar(&dbDebug, "db-debug", false, "enable GORM debug logging")
	rootCmd.Flags().StringVar(&onlyFlag, "only", "", "comma-separated list of sections to configure (default: all)")
	rootCmd.Flags().StringVar(&skipFlag, "skip", "", "comma-separated list of sections to skip")

	showCmd.Flags().StringVarP(&configFile, "config", "c", "", "path to config file (optional)")
	showCmd.Flags().StringVar(&dbType, "db-type", "", "override database type: sqlite|mysql|postgres")
	showCmd.Flags().StringVar(&dbDSN, "db-dsn", "", "override database DSN (for mysql/postgres)")
	showCmd.Flags().StringVar(&dbDir, "db-dir", "", "override data directory (for sqlite)")
	showCmd.Flags().BoolVar(&dbDebug, "db-debug", false, "enable GORM debug logging")

	rootCmd.AddCommand(showCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func rootRunE(_ *cobra.Command, _ []string) error {
	if err := loadBackends(); err != nil {
		return err
	}
	sections, err := migration.ParseSections(onlyFlag)
	if err != nil {
		return err
	}
	if skipFlag != "" {
		skip, err := migration.ParseSkipSections(skipFlag)
		if err != nil {
			return err
		}
		filtered := make([]migration.Section, 0, len(sections))
		for _, s := range sections {
			if !skip[s] {
				filtered = append(filtered, s)
			}
		}
		sections = filtered
	}
	if len(sections) == 0 {
		fmt.Println("No sections to configure.")
		return nil
	}
	runWizard(sections)
	return nil
}

func showRunE(_ *cobra.Command, _ []string) error {
	if err := loadBackends(); err != nil {
		return err
	}
	showAll()
	return nil
}

func loadBackends() error {
	// Try loading config file for DB connection and prepopulation
	if configFile != "" {
		if err := config.Load(configFile); err != nil {
			return errors.Wrap(err, "failed to load config file")
		}
		log.Info().Msg("Loaded config file")
		c := config.Get()
		// Parse the same config file with migration types for prepopulation
		if mc, err := loadMigrationConfig(configFile); err != nil {
			log.Warn().Err(err).Msg("failed to parse config file for prepopulation")
		} else {
			migrationCfg = mc
		}
		// Connect to DB using config file settings (possibly overridden by flags)
		storageConf := c.Storage
		applyDBOverrides(&storageConf)
		b, err := config.LoadStorageBackends(storageConf)
		if err != nil {
			return err
		}
		backends = b
	} else {
		// No config file — use flags or auto-discover
		if dbType != "" || dbDSN != "" || dbDir != "" {
			driver := storage.DriverSQLite
			if dbType != "" {
				d, err := storage.ParseDriverType(strings.ToLower(dbType))
				if err != nil {
					return err
				}
				driver = d
			}
			cfg := storage.Config{
				Driver:  driver,
				DSN:     dbDSN,
				DataDir: dbDir,
				Debug:   dbDebug,
			}
			b, err := storage.LoadStorageBackends(cfg)
			if err != nil {
				return err
			}
			backends = b
		} else {
			// Try auto-discovery
			if err := config.Load(""); err != nil {
				return errors.Wrap(err, "failed to load config (provide --config or --db-type/--db-dir)")
			}
			c := config.Get()
			if mc, err := loadMigrationConfigFromLocations(); err == nil {
				migrationCfg = mc
			}
			storageConf := c.Storage
			applyDBOverrides(&storageConf)
			b, err := config.LoadStorageBackends(storageConf)
			if err != nil {
				return err
			}
			backends = b
		}
	}
	log.Info().Msg("Connected to database")
	return nil
}

func applyDBOverrides(c *config.StorageConf) {
	if dbType != "" {
		d, err := storage.ParseDriverType(strings.ToLower(dbType))
		if err != nil {
			log.Fatal().Err(err).Msg("invalid --db-type")
		}
		c.Driver = d
	}
	if dbDSN != "" {
		c.DSN = dbDSN
	}
	if dbDir != "" {
		c.DataDir = dbDir
	}
	if dbDebug {
		c.Debug = true
	}
}
