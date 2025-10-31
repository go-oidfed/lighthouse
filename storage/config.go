package storage

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// DriverType represents the type of database driver
type DriverType string

const (
	// DriverSQLite is the SQLite driver
	DriverSQLite DriverType = "sqlite"
	// DriverMySQL is the MySQL driver
	DriverMySQL DriverType = "mysql"
	// DriverPostgres is the PostgreSQL driver
	DriverPostgres DriverType = "postgres"
)

var SupportedDrivers = []DriverType{
	DriverSQLite,
	DriverMySQL,
	DriverPostgres,
}

// DSN creates and returns a dsn connection string for the passed DriverType and DSNConf
func DSN(driver DriverType, conf DSNConf) (string, error) {
	switch driver {
	case DriverSQLite:
		return "", errors.Errorf("driver %s does not use dsn", driver)
	case DriverMySQL:
		if conf.Port == 0 {
			conf.Port = 3306
		}
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True", conf.User, conf.Password, conf.Host, conf.Port,
			conf.DB,
		), nil
	case DriverPostgres:
		if conf.Port == 0 {
			conf.Port = 9920
		}
		return fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d",
			conf.Host, conf.User, conf.Password, conf.DB, conf.Port,
		), nil
	default:
		return "", errors.Errorf("unsupported driver '%s'", driver)
	}
}

// DSNConf provides configuration options for database connection strings.
// It contains common connection parameters used across different database drivers
// including MySQL and PostgreSQL. When used with the DSN function, this struct
// helps generate proper connection strings based on the selected driver type.
type DSNConf struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	DB       string `yaml:"db"`
}

// Config represents the database configuration
type Config struct {
	// Driver is the database driver type
	Driver DriverType `yaml:"driver"`
	// DSN is the data source name (connection string)
	// For SQLite, this is the database file path
	// For MySQL, this is the connection string: user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	// For PostgreSQL, this is the connection string: host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable TimeZone=Asia/Shanghai
	DSN string `yaml:"dsn"`
	// DataDir is the directory where database files are stored (for SQLite)
	DataDir string `yaml:"data_dir"`
	// Debug enables debug logging
	Debug bool `yaml:"debug"`
	// UsersHash defines parameters for hashing admin user passwords
	UsersHash Argon2idParams
}

// Argon2idParams configures Argon2id hashing parameters
type Argon2idParams struct {
	Time        uint32
	MemoryKiB   uint32
	Parallelism uint8
	KeyLen      uint32
	SaltLen     uint32
}

// Connect establishes a connection to the database based on the configuration
func Connect(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case DriverSQLite:
		// If DSN is not provided, use the default database file in DataDir
		dsn := cfg.DSN
		if dsn == "" {
			dsn = filepath.Join(cfg.DataDir, "lighthouse.db")
		}
		dialector = sqlite.Open(dsn)
	case DriverMySQL:
		dialector = mysql.Open(cfg.DSN)
	case DriverPostgres:
		dialector = postgres.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	logMode := logger.Silent
	if cfg.Debug {
		logMode = logger.Info
	}

	return gorm.Open(
		dialector, &gorm.Config{
			Logger: logger.Default.LogMode(logMode),
		},
	)
}

// LoadStorageBackends initializes a warehouse and returns grouped backends.
func LoadStorageBackends(cfg Config) (model.Backends, error) {
	warehouse, err := NewStorage(cfg)
	if err != nil {
		return model.Backends{}, err
	}
	return model.Backends{
		Subordinates:     warehouse.SubordinateStorage(),
		TrustMarks:       warehouse.TrustMarkedEntitiesStorage(),
		AuthorityHints:   warehouse.AuthorityHintsStorage(),
		TrustMarkTypes:   warehouse.TrustMarkTypesStorage(),
		TrustMarkOwners:  warehouse.TrustMarkOwnersStorage(),
		TrustMarkIssuers: warehouse.TrustMarkIssuersStorage(),
		AdditionalClaims: warehouse.AdditionalClaimsStorage(),
		KV:               warehouse.KeyValue(),
		Users:            warehouse.UsersStorage(),
	}, nil
}
