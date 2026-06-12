package storage

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// JTIStorageType represents the type of JTI storage backend
type JTIStorageType string

const (
	// JTIStorageCache uses the cache backend (Redis/memory)
	JTIStorageCache JTIStorageType = "cache"
	// JTIStorageDB uses the database backend
	JTIStorageDB JTIStorageType = "db"
)

// NewJTIStorage creates a new JTI storage backend based on the specified type
func NewJTIStorage(backendType JTIStorageType, db *gorm.DB) (model.JTIStorageBackend, error) {
	switch backendType {
	case JTIStorageCache, "":
		// Default to cache backend
		return NewJTICacheStorage(), nil
	case JTIStorageDB:
		if db == nil {
			return nil, errors.New("database connection required for DB JTI storage")
		}
		return NewJTIDBStorage(db), nil
	default:
		return nil, errors.Errorf("unknown JTI storage backend type: %s", backendType)
	}
}
