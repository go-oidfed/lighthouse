package config

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

type storageConf struct {
	BackendType backendType        `yaml:"backend"`
	Driver      storage.DriverType `yaml:"driver"`
	DataDir     string             `yaml:"data_dir"`
	DSN         string             `yaml:"dsn"`
	storage.DSNConf
	Debug bool `yaml:"debug"`
}

func (c *storageConf) validate() error {
	if c.BackendType != "" {
		return errors.New("backend types have been deprecated; please migrate to gorm")
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

var defaultStorageConf = storageConf{
	Driver: storage.DriverSQLite,
	DSNConf: storage.DSNConf{
		User: "lighthouse",
		Host: "localhost",
		DB:   "lighthouse",
	},
	Debug: false,
}

type backendType string

const (
	BackendTypeJSON   backendType = "json"
	BackendTypeBadger backendType = "badger"
	BackendTypeGorm   backendType = "gorm"
)

// LoadStorageBackends loads and returns the storage backends for the passed Config
func LoadStorageBackends(c storageConf) (model.Backends, error) {
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
