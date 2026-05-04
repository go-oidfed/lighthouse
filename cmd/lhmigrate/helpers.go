package main

import (
	"fmt"

	"github.com/go-oidfed/lighthouse/storage"
)

// validateDBFlags validates the database connection flags for a given driver type.
// Returns an error if required flags are missing.
func validateDBFlags(driver storage.DriverType, dbDir, dbDSN string) error {
	switch driver {
	case storage.DriverSQLite:
		if dbDir == "" {
			return fmt.Errorf("--db-dir is required for sqlite")
		}
	case storage.DriverMySQL, storage.DriverPostgres:
		if dbDSN == "" {
			return fmt.Errorf("--db-dsn is required for %s", driver)
		}
	}
	return nil
}
