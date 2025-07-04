package config

import (
	"log"
	"reflect"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/internal/utils/fileutils"
)

// Config holds configuration for the entity
type Config struct {
	Server     lighthouse.ServerConf `yaml:"server"`
	Logging    loggingConf           `yaml:"logging"`
	Storage    storageConf           `yaml:"storage"`
	Signing    signingConf           `yaml:"signing"`
	Endpoints  Endpoints             `yaml:"endpoints"`
	Federation federationConf        `yaml:"federation_data"`
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
	return nil
}

var c = Config{
	Server:     defaultServerConf,
	Logging:    defaultLoggingConf,
	Storage:    defaultStorageConf,
	Signing:    defaultSigningConf,
	Endpoints:  defaultEndpointConf,
	Federation: defaultFederationConf,
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
func Load(filename string) {
	var content []byte
	var err error
	if filename != "" {
		content, err = fileutils.ReadFile(filename)
	} else {
		content, _ = mustReadConfigFile("config.yaml", possibleConfigLocations)
	}
	if err != nil {
		log.Fatal(err)
	}
	if err = yaml.Unmarshal(content, &c); err != nil {
		log.Fatal(err)
	}
	if err = c.Validate(); err != nil {
		log.Fatal(err)
	}
}
