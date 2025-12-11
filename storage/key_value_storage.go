package storage

import (
	"database/sql"
	"encoding/json"
	"errors"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// KeyValueStorage implements model.KeyValueStore using GORM.
type KeyValueStorage struct {
	db *gorm.DB
}

// KeyValue provides an accessor for scoped key-value storage.
func (s *Storage) KeyValue() *KeyValueStorage {
	return &KeyValueStorage{db: s.db}
}

// Get returns the JSON value for a (scope, key). If not found, returns nil, nil.
func (s *KeyValueStorage) Get(scope, key string) (datatypes.JSON, error) {
	// Read the JSON/JSONB value as raw bytes to support scalar JSON (e.g., numbers).
	var raw []byte
	row := s.db.Model(&model.KeyValue{}).
		Select("value").
		Where(
			&model.KeyValue{
				Scope: scope,
				Key:   key,
			},
		).
		Row()
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return raw, nil
}

// Set upserts the JSON value for a (scope, key).
func (s *KeyValueStorage) Set(scope, key string, value datatypes.JSON) error {
	kv := model.KeyValue{
		Scope: scope,
		Key:   key,
		Value: value,
	}
	return s.db.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "scope"},
				{Name: "key"},
			},
			DoUpdates: clause.AssignmentColumns(
				[]string{
					"value",
					"updated_at",
				},
			),
		},
	).Create(&kv).Error
}

// Delete removes a (scope, key) pair. No error if it's missing.
func (s *KeyValueStorage) Delete(scope, key string) error {
	return s.db.Where(
		&model.KeyValue{
			Scope: scope,
			Key:   key,
		},
	).Delete(&model.KeyValue{}).Error
}

// GetAs retrieves and unmarshals the value for (scope, key) into out.
// out must be a pointer to the target type. Returns (false, nil) if not found.
func (s *KeyValueStorage) GetAs(scope, key string, out any) (bool, error) {
	raw, err := s.Get(scope, key)
	if err != nil {
		return false, err
	}
	if raw == nil {
		return false, nil
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return false, err
	}
	return true, nil
}

// SetAny marshals v to JSON and stores it at (scope, key).
func (s *KeyValueStorage) SetAny(scope, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Set(scope, key, datatypes.JSON(b))
}
