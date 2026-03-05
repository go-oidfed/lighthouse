package config

import (
	"reflect"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/fileutils"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse"
)

// Config holds configuration for the entity
type Config struct {
	Server     lighthouse.ServerConf `yaml:"server"`
	Logging    loggingConf           `yaml:"logging"`
	Storage    storageConf           `yaml:"storage"`
	Caching    cachingConf           `yaml:"cache"`
	Signing    SigningConf           `yaml:"signing"`
	Endpoints  Endpoints             `yaml:"endpoints"`
	Federation federationConf        `yaml:"federation_data"`
	API        apiConf               `yaml:"api"`
	Stats      StatsConf             `yaml:"stats"`
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
	return nil
}

var c = Config{
	Server:     defaultServerConf,
	Logging:    defaultLoggingConf,
	Storage:    defaultStorageConf,
	Signing:    defaultSigningConf,
	Endpoints:  defaultEndpointConf,
	Federation: defaultFederationConf,
	API:        defaultAPIConf,
	Stats:      defaultStatsConf,
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

// Load loads the config from the given file
func Load(filename string) error {
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
