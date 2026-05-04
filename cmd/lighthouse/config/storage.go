package config

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// StorageConf holds storage/database configuration.
//
// Environment variables (with prefix LH_STORAGE_):
//   - LH_STORAGE_DRIVER: Database driver (sqlite, mysql, postgres)
//   - LH_STORAGE_DATA_DIR: Directory for SQLite database files
//   - LH_STORAGE_DSN: Database connection string
//   - LH_STORAGE_USER: Database username (for DSN building)
//   - LH_STORAGE_PASSWORD: Database password
//   - LH_STORAGE_HOST: Database host
//   - LH_STORAGE_PORT: Database port
//   - LH_STORAGE_DB: Database name
//   - LH_STORAGE_DEBUG: Enable debug logging
type StorageConf struct {
	// Deprecated: Only used for discovering a migration need
	BackendType string `yaml:"backend" envconfig:"-"`
	// Driver is the database driver type.
	// Env: LH_STORAGE_DRIVER
	Driver storage.DriverType `yaml:"driver" envconfig:"DRIVER"`
	// DataDir is the directory for SQLite database files.
	// Env: LH_STORAGE_DATA_DIR
	DataDir string `yaml:"data_dir" envconfig:"DATA_DIR"`
	// DSN is the database connection string.
	// Env: LH_STORAGE_DSN
	DSN string `yaml:"dsn" envconfig:"DSN"`
	// DSNConf provides individual connection parameters (embedded).
	// Env: LH_STORAGE_USER, LH_STORAGE_PASSWORD, LH_STORAGE_HOST, LH_STORAGE_PORT, LH_STORAGE_DB
	storage.DSNConf
	// Debug enables debug logging.
	// Env: LH_STORAGE_DEBUG
	Debug bool `yaml:"debug" envconfig:"DEBUG"`
}

// users hashing parameters moved under api.admin.users_hash

func (c *StorageConf) validate() error {
	if c.BackendType != "" {
		return errors.New("backend types have been deprecated; please migrate")
	}

	if c.Driver == (storage.DriverSQLite) {
		if c.DataDir == "" {
			return errors.New("error in storage conf: data_dir must be specified")
		}
		return nil
	}
	var err error
	if c.DSN == "" {
		c.DSN, err = storage.DSN(c.Driver, c.DSNConf)
	}
	return err
}

var defaultStorageConf = StorageConf{
	Driver: storage.DriverSQLite,
	DSNConf: storage.DSNConf{
		User: "lighthouse",
		Host: "localhost",
		DB:   "lighthouse",
	},
	Debug: false,
}

// LoadStorageBackends loads and returns the storage backends for the passed Config
func LoadStorageBackends(c StorageConf) (model.Backends, error) {
	cfg := storage.Config{
		Driver:  c.Driver,
		DSN:     c.DSN,
		DataDir: c.DataDir,
		Debug:   c.Debug,
	}
	backs, err := storage.LoadStorageBackends(cfg)
	if err != nil {
		return model.Backends{}, err
	}
	log.Info("Loaded storage backend")
	return backs, nil
}
