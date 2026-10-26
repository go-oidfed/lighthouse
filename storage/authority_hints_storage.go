package storage

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AuthorityHintsStorage provides CRUD access to AuthorityHint records
// implementing model.AuthorityHintStore.
type AuthorityHintsStorage struct {
	db *gorm.DB
}

func (s *AuthorityHintsStorage) List() ([]model.AuthorityHint, error) {
	var items []model.AuthorityHint
	if err := s.db.Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "authority_hints: list failed")
	}
	return items, nil
}

func (s *AuthorityHintsStorage) Create(hint model.AddAuthorityHint) (*model.AuthorityHint, error) {
	var existing model.AuthorityHint
	result := s.db.Unscoped().Where("entity_id = ?", hint.EntityID).First(&existing)
	if result.Error == nil {
		if existing.DeletedAt.Valid {
			existing.DeletedAt = gorm.DeletedAt{}
			existing.EntityID = hint.EntityID
			existing.Description = hint.Description
			existing.JWKSSync = hint.JWKSSync
			if err := s.db.Save(&existing).Error; err != nil {
				return nil, errors.Wrap(err, "authority_hints: reactivation failed")
			}
			return &existing, nil
		}
		return nil, model.AlreadyExistsError("authority hint already exists")
	}

	item := &model.AuthorityHint{
		EntityID:    hint.EntityID,
		Description: hint.Description,
		JWKSSync:    hint.JWKSSync,
	}
	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("authority hint already exists")
		}
		return nil, errors.Wrap(err, "authority_hints: create failed")
	}
	return item, nil
}

func (s *AuthorityHintsStorage) findByIdent(ident string) (*model.AuthorityHint, error) {
	var item model.AuthorityHint
	// Try numeric ID
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}
	// Fallback to entity_id match
	if err := s.db.Where("entity_id = ?", ident).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("authority hint not found")
		}
		return nil, errors.Wrap(err, "authority_hints: get failed")
	}
	return &item, nil
}

func (s *AuthorityHintsStorage) Get(ident string) (*model.AuthorityHint, error) {
	return s.findByIdent(ident)
}

func (s *AuthorityHintsStorage) Update(ident string, update model.AddAuthorityHint) (*model.AuthorityHint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	// Full-replacement PUT: entity_id is a required identity field (the
	// handler enforces presence), so it is always written; changing it to a
	// value already used by another row surfaces as a unique-constraint
	// conflict below.
	item.EntityID = update.EntityID
	item.Description = update.Description
	item.JWKSSync = update.JWKSSync
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("authority hint already exists")
		}
		return nil, errors.Wrap(err, "authority_hints: update failed")
	}
	return item, nil
}

// Patch partially updates an authority hint: only fields present in the patch
// map are changed; absent fields are left untouched. "description" null/string
// sets or clears the description; "jwks_sync" null clears syncing and an
// object replaces the sync configuration (mode is validated).
func (s *AuthorityHintsStorage) Patch(ident string, patch map[string]any) (*model.AuthorityHint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	if v, ok := patch["description"]; ok {
		if v == nil {
			item.Description = ""
		} else {
			desc, ok := v.(string)
			if !ok {
				return nil, model.ValidationError("description must be a string")
			}
			item.Description = desc
		}
	}
	if v, ok := patch["jwks_sync"]; ok {
		if v == nil {
			item.JWKSSync = nil
		} else {
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, model.ValidationError("jwks_sync must be an object")
			}
			var cfg model.JWKSSyncConf
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, model.ValidationError("jwks_sync must be an object")
			}
			if err := cfg.Validate(); err != nil {
				return nil, model.ValidationError(err.Error())
			}
			item.JWKSSync = &cfg
		}
	}
	if err := s.db.Save(item).Error; err != nil {
		return nil, errors.Wrap(err, "authority_hints: patch failed")
	}
	return item, nil
}

func (s *AuthorityHintsStorage) Delete(ident string) error {
	item, err := s.findByIdent(ident)
	if err != nil {
		return err
	}
	if err = s.db.Delete(item).Error; err != nil {
		return errors.Wrap(err, "authority_hints: delete failed")
	}
	return nil
}

// isUniqueConstraintError performs a cheap check across supported drivers.
func isUniqueConstraintError(err error) bool {
	msg := err.Error()
	// sqlite | mysql | postgres common markers
	if
	// SQLite
	(containsAny(msg, "UNIQUE constraint failed", "constraint failed")) ||
		// MySQL
		(containsAny(msg, "Duplicate entry", "Error 1062")) ||
		// Postgres
		(containsAny(msg, "duplicate key value", "violates unique constraint")) {
		return true
	}
	return false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
