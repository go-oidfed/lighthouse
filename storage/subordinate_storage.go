package storage

import (
	"github.com/go-oidfed/lighthouse/storage/model"
)

// SubordinateStorageBackend is an interface to store SubordinateInfo
type SubordinateStorageBackend interface {
	Write(entityID string, info model.SubordinateInfo) error
	Delete(entityID string) error
	Block(entityID string) error
	Approve(entityID string) error
	Subordinate(entityID string) (*model.SubordinateInfo, error)
	Active() SubordinateStorageQuery
	Blocked() SubordinateStorageQuery
	Pending() SubordinateStorageQuery
	Load() error
}

// SubordinateStorageQuery is an interface to query SubordinateInfo from storage
type SubordinateStorageQuery interface {
	Subordinates() ([]model.SubordinateInfo, error)
	EntityIDs() ([]string, error)
	AddFilter(filter SubordinateStorageQueryFilter, value any) error
}

// SubordinateStorageQueryFilter is a function to filter SubordinateInfo
type SubordinateStorageQueryFilter func(info model.SubordinateInfo, value any) bool
