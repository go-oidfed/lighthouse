package config

import (
	"os"
	"reflect"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/fileutils"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse"
)

// Config holds configuration for the entity.
//
// All configuration options can be set via environment variables with the LH_ prefix.
// Environment variables override values from the YAML config file.
//
// Special environment variables:
//   - LH_CONFIG_FILE: Path to the configuration file
//   - LH_LOG_LEVEL: Shortcut for LH_LOGGING_INTERNAL_LEVEL
//
// Environment variables (with prefix LH_):
//   - LH_ENTITY_ID: Entity identifier URL
//   - LH_SERVER_*: Server configuration (see ServerConf)
//   - LH_LOGGING_*: Logging configuration (see loggingConf)
//   - LH_STORAGE_*: Storage configuration (see storageConf)
//   - LH_CACHE_*: Caching configuration (see cachingConf)
//   - LH_SIGNING_*: Signing configuration (see SigningConf)
//   - LH_ENDPOINTS_*: Endpoints configuration (see Endpoints)
//   - LH_FEDERATION_DATA_*: Federation configuration (see federationConf)
//   - LH_API_*: API configuration (see apiConf)
//   - LH_STATS_*: Statistics configuration (see StatsConf)
type Config struct {
	// EntityID is the entity identifier URL.
	// Env: LH_ENTITY_ID
	EntityID string `yaml:"entity_id" envconfig:"ENTITY_ID"`
	// Server holds server configuration.
	// Env prefix: LH_SERVER_
	Server lighthouse.ServerConf `yaml:"server" envconfig:"SERVER"`
	// Logging holds logging configuration.
	// Env prefix: LH_LOGGING_
	Logging loggingConf `yaml:"logging" envconfig:"LOGGING"`
	// Storage holds storage configuration.
	// Env prefix: LH_STORAGE_
	Storage storageConf `yaml:"storage" envconfig:"STORAGE"`
	// Caching holds caching configuration.
	// Env prefix: LH_CACHE_
	Caching cachingConf `yaml:"cache" envconfig:"CACHE"`
	// Signing holds signing configuration.
	// Env prefix: LH_SIGNING_
	Signing SigningConf `yaml:"signing" envconfig:"SIGNING"`
	// Endpoints holds endpoints configuration.
	// Env prefix: LH_ENDPOINTS_
	Endpoints Endpoints `yaml:"endpoints" envconfig:"ENDPOINTS"`
	// API holds API configuration.
	// Env prefix: LH_API_
	API apiConf `yaml:"api" envconfig:"API"`
	// Stats holds statistics configuration.
	// Env prefix: LH_STATS_
	Stats StatsConf `yaml:"stats" envconfig:"STATS"`
}

type configValidator interface {
	validate() error
}

// Validate checks all fields of Config that implement configValidator (pointer receivers)
func (c *Config) Validate() error {
	v := reflect.ValueOf(c).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)

		// Get addressable pointer to field if possible
		if fieldVal.CanAddr() {
			ptr := fieldVal.Addr().Interface()

			if validator, ok := ptr.(configValidator); ok {
				if err := validator.validate(); err != nil {
					return errors.Errorf("validation failed for field '%s': %s", t.Field(i).Name, err.Error())
				}
			}
		}
	}
	// TODO make sure that this check is still applied,
	//  but interval will also be configurable through the api
	// if c.Signing.KeyRotation.Interval < c.Federation.ConfigurationLifetime {
	// 	c.Signing.KeyRotation.Interval = c.Federation.ConfigurationLifetime
	// }

	if c.EntityID == "" {
		return errors.New("entity_id must be specified")
	}
	return nil
}

var c = Config{
	Server:    defaultServerConf,
	Logging:   defaultLoggingConf,
	Storage:   defaultStorageConf,
	Signing:   defaultSigningConf,
	Endpoints: defaultEndpointConf,
	API:       defaultAPIConf,
	Stats:     defaultStatsConf,
}

// Get returns the Config
func Get() Config {
	return c
}

var possibleConfigLocations = []string{
	".",
	"config",
	"/config",
	"/lighthouse/config",
	"/lighthouse",
	"/data/config",
	"/data",
	"/etc/lighthouse",
}

// Load loads the config from the given file.
//
// The loading order is:
//  1. Default values (defined in defaultXxxConf variables)
//  2. YAML config file (overrides defaults)
//  3. Environment variables with LH_ prefix (overrides YAML)
//
// The config file path can be specified via:
//   - The filename parameter
//   - The LH_CONFIG_FILE environment variable
//   - Auto-discovery from possibleConfigLocations
//
// Special shortcut: LH_LOG_LEVEL is an alias for LH_LOGGING_INTERNAL_LEVEL
func Load(filename string) error {
	// Check for config file path from env var if not provided
	if filename == "" {
		filename = os.Getenv("LH_CONFIG_FILE")
	}

	var content []byte
	if filename != "" {
		var err error
		content, err = fileutils.ReadFile(filename)
		if err != nil {
			return err
		}
	} else {
		content, _ = fileutils.ReadFileFromLocations("config.yaml", possibleConfigLocations)
		if content == nil {
			return errors.Errorf("could not find config file in any of the possible locations")
		}
	}
	if err := yaml.Unmarshal(content, &c); err != nil {
		return err
	}

	// Override with environment variables
	if err := envconfig.Process("LH", &c); err != nil {
		return errors.Wrap(err, "failed to process environment variables")
	}

	// Handle LH_LOG_LEVEL shortcut
	if logLevel := os.Getenv("LH_LOG_LEVEL"); logLevel != "" {
		c.Logging.Internal.Level = logLevel
	}

	if err := c.Validate(); err != nil {
		return err
	}
	return nil
}

// MustLoad loads the config from the given file and panics on error.
// This should only be called from main() or init() functions.
func MustLoad(filename string) {
	if err := Load(filename); err != nil {
		log.Fatal(err)
	}
}
