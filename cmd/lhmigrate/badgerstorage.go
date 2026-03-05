package main

// Deprecated: BadgerStorage is deprecated and will be removed in a future release.
// Use the GORM storage backend instead. See the README.md file for migration instructions.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// NewBadgerStorage creates a new BadgerStorage at the passed storage location
//
// Deprecated: Use the GORM storage backend instead. See the README.md file for migration instructions.
func NewBadgerStorage(path string) (*BadgerStorage, error) {
	storage := &BadgerStorage{Path: path}
	err := storage.Load()
	return storage, err
}

// BadgerStorage is a type for a simple database storage backend -
type BadgerStorage struct {
	*badger.DB
	Path   string
	loaded bool
}

// SubordinateStorage gives a SubordinateBadgerStorage
func (store *BadgerStorage) SubordinateStorage() loadLegacySubordinateInfos {
	return func() (infos []legacySubordinateInfo, err error) {
		s := &BadgerSubStorage{
			db:     store,
			subKey: "subordinates",
		}
		err = s.Load()
		if err != nil {
			return nil, err
		}
		err = s.ReadIterator(
			func(_, v []byte) error {
				var info legacySubordinateInfo
				if err = json.Unmarshal(v, &info); err != nil {
					return err
				}
				infos = append(infos, info)
				return nil
			},
		)
		return
	}
}

// TrustMarkedEntitiesStorage gives a TrustMarkedEntitiesBadgerStorage
func (store *BadgerStorage) TrustMarkedEntitiesStorage() *TrustMarkedEntitiesBadgerStorage {
	return &TrustMarkedEntitiesBadgerStorage{
		store: &BadgerSubStorage{
			db:     store,
			subKey: "subordinates",
		},
	}
}

// Write writes a value to the database
func (store *BadgerStorage) Write(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = store.Update(
		func(txn *badger.Txn) error {
			return txn.Set([]byte(key), data)
		},
	)
	return err
}

// Delete deletes the value associated with the given key from the database
func (store *BadgerStorage) Delete(key string) error {
	return store.Update(
		func(txn *badger.Txn) error {
			return txn.Delete([]byte(key))
		},
	)
}

// Read reads the value for a given key into target
func (store *BadgerStorage) Read(key string, target any) (bool, error) {
	var notFound bool
	err := store.View(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				notFound = true
				return fmt.Errorf("'%s' not found", key)
			}

			return item.Value(
				func(val []byte) error {
					return json.Unmarshal(val, target)
				},
			)
		},
	)
	return !notFound, err
}

// BadgerSubStorage is a type for a sub-storage of a BadgerStorage
type BadgerSubStorage struct {
	db     *BadgerStorage
	subKey string
}

// Load loads the database
func (store *BadgerSubStorage) Load() error {
	return store.db.Load()
}
func (store *BadgerSubStorage) key(key string) string {
	return store.subKey + ":" + key
}

// Write writes a value to the sub-database
func (store *BadgerSubStorage) Write(key string, value any) error {
	return store.db.Write(store.key(key), value)
}

// Delete deletes the value associated with the given key from the sub-database
func (store *BadgerSubStorage) Delete(key string) error {
	return store.db.Delete(store.key(key))
}

// Read reads the value for a given key into target
func (store *BadgerSubStorage) Read(key string, target any) (bool, error) {
	return store.db.Read(store.key(key), target)
}

// ReadIterator uses the passed iterator function do iterate over all the key-value-pairs in this sub storage
func (store *BadgerSubStorage) ReadIterator(do func(k, v []byte) error, prefix ...string) error {
	var prfx string
	if len(prefix) > 0 {
		prfx = prefix[0]
	}
	return store.db.View(
		func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			scanPrefix := []byte(store.subKey + ":" + prfx)
			for it.Seek(scanPrefix); it.ValidForPrefix(scanPrefix); it.Next() {
				item := it.Item()
				k := item.Key()
				err := item.Value(
					func(v []byte) error {
						return do(k, v)
					},
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}

// Load loads the database
func (store *BadgerStorage) Load() error {
	if store.loaded {
		return nil
	}
	db, err := badger.Open(badger.DefaultOptions(store.Path))
	if err != nil {
		return err
	}
	store.DB = db

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
		again:
			err := db.RunValueLogGC(0.7)
			if err == nil {
				goto again
			}
		}
	}()
	store.loaded = true
	return nil
}

// TrustMarkedEntitiesBadgerStorage is a type implementing the TrustMarkedEntitiesStorageBackend interface
type TrustMarkedEntitiesBadgerStorage struct {
	store *BadgerSubStorage
}

// Block implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Block(trustMarkType, entityID string) error {
	return store.write(trustMarkType, entityID, model.StatusBlocked)
}

// Approve implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Approve(trustMarkType, entityID string) error {
	return store.write(trustMarkType, entityID, model.StatusActive)
}

// Request implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Request(trustMarkType, entityID string) error {
	return store.write(trustMarkType, entityID, model.StatusPending)
}

// TrustMarkedStatus implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) TrustMarkedStatus(trustMarkType, entityID string) (model.Status, error) {
	var status model.Status
	var id string
	k := store.key(trustMarkType, entityID)
	found, err := store.store.Read(k, &status)
	if !found {
		return model.StatusInactive, nil
	}
	if err != nil {
		found, e := store.store.Read(k, &id)
		if e == nil && found {
			return model.StatusActive, nil
		}
		return -1, err
	}
	if !found {
		return model.StatusInactive, nil
	}
	return status, nil
}

// Active implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Active(trustMarkType string) ([]string, error) {
	return store.trustMarkedEntities(trustMarkType, model.StatusActive)
}

// Blocked implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Blocked(trustMarkType string) ([]string, error) {
	return store.trustMarkedEntities(trustMarkType, model.StatusBlocked)
}

// Pending implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Pending(trustMarkType string) ([]string, error) {
	return store.trustMarkedEntities(trustMarkType, model.StatusPending)
}

func (*TrustMarkedEntitiesBadgerStorage) key(trustMarkType, entityID string) string {
	return fmt.Sprintf("%s|%s", trustMarkType, entityID)
}

// Load implements the LegacySubordinateStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Load() error {
	return store.store.Load()
}

// Write implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) write(trustMarkType, entityID string, status model.Status) error {
	return store.store.Write(store.key(trustMarkType, entityID), status)
}

// Delete implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) Delete(trustMarkType, entityID string) error {
	return store.store.Delete(store.key(trustMarkType, entityID))
}

func (store *TrustMarkedEntitiesBadgerStorage) trustMarkedEntities(
	trustMarkType string, status model.Status,
) (entityIDs []string, err error) {
	err = store.store.ReadIterator(
		func(k, v []byte) error {
			var id string
			var s model.Status
			if err = json.Unmarshal(v, &s); err != nil {
				// try legacy storage format
				if e := json.Unmarshal(v, &id); e != nil {
					return err
				}
				s = model.StatusActive
			} else {
				id = strings.TrimPrefix(string(k), fmt.Sprintf("%s|", trustMarkType))
			}
			if s == status {
				entityIDs = append(entityIDs, id)
			}
			return nil
		},
		trustMarkType,
	)
	return
}

// HasTrustMark implements the TrustMarkedEntitiesStorageBackend interface
func (store *TrustMarkedEntitiesBadgerStorage) HasTrustMark(trustMarkType, entityID string) (bool, error) {
	status, err := store.TrustMarkedStatus(trustMarkType, entityID)
	return status == model.StatusActive, err
}
