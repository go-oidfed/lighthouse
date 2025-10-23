package storage

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AuthorityHintsStorage provides CRUD access to AuthorityHint records.
type AuthorityHintsStorage struct {
	db *gorm.DB
}

// AuthorityHintsStore is the abstraction used by handlers.
type AuthorityHintsStore interface {
	List() ([]model.AuthorityHint, error)
	Create(hint model.AddAuthorityHint) (*model.AuthorityHint, error)
	Get(ident string) (*model.AuthorityHint, error)
	Update(ident string, update model.AddAuthorityHint) (*model.AuthorityHint, error)
	Delete(ident string) error
}

func (s *AuthorityHintsStorage) List() ([]model.AuthorityHint, error) {
	var items []model.AuthorityHint
	if err := s.db.Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "authority_hints: list failed")
	}
	return items, nil
}

func (s *AuthorityHintsStorage) Create(hint model.AddAuthorityHint) (*model.AuthorityHint, error) {
	item := &model.AuthorityHint{
		EntityID:    hint.EntityID,
		Description: hint.Description,
	}
	if err := s.db.Create(item).Error; err != nil {
		// Check unique/constraint violations
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
	item.EntityID = update.EntityID
	item.Description = update.Description
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("authority hint already exists")
		}
		return nil, errors.Wrap(err, "authority_hints: update failed")
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
