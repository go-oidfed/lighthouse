package main

import (
	"os"

	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/fileutils"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/internal/migration"
)

func loadMigrationConfig(filename string) (*migration.Config, error) {
	content, err := fileutils.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}
	var cfg migration.Config
	if err = yaml.Unmarshal(content, &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to parse config file")
	}
	return &cfg, nil
}

func loadMigrationConfigFromLocations() (*migration.Config, error) {
	locations := []string{
		".",
		"config",
		"/config",
		"/lighthouse/config",
		"/lighthouse",
		"/data/config",
		"/data",
		"/etc/lighthouse",
	}
	for _, loc := range locations {
		path := loc + "/config.yaml"
		if _, err := os.Stat(path); err == nil {
			return loadMigrationConfig(path)
		}
	}
	return nil, os.ErrNotExist
}
