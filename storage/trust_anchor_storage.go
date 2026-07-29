package storage

import (
	"strconv"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lib/jwx"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustAnchorStorage implements model.TrustAnchorStore using GORM.
type TrustAnchorStorage struct {
	db *gorm.DB
}

// NewTrustAnchorStorage creates a new TrustAnchorStorage.
func NewTrustAnchorStorage(db *gorm.DB) *TrustAnchorStorage {
	return &TrustAnchorStorage{db: db}
}

// List returns all trust anchors with their JWKS preloaded.
func (s *TrustAnchorStorage) List() ([]model.TrustAnchor, error) {
	var items []model.TrustAnchor
	if err := s.db.Preload("JWKS").Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "trust_anchors: list failed")
	}
	return items, nil
}

// findByEntityID finds a trust anchor by entity_id, preload JWKS.
func (s *TrustAnchorStorage) findByEntityID(entityID string) (*model.TrustAnchor, error) {
	var item model.TrustAnchor
	if err := s.db.Preload("JWKS").Where("entity_id = ?", entityID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust anchor not found")
		}
		return nil, errors.Wrap(err, "trust_anchors: get failed")
	}
	return &item, nil
}

// Get returns a trust anchor by entity_id.
func (s *TrustAnchorStorage) Get(entityID string) (*model.TrustAnchor, error) {
	return s.findByEntityID(entityID)
}

// findByID finds a trust anchor by primary key, preload JWKS.
func (s *TrustAnchorStorage) findByID(id uint) (*model.TrustAnchor, error) {
	var item model.TrustAnchor
	if err := s.db.Preload("JWKS").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust anchor not found")
		}
		return nil, errors.Wrap(err, "trust_anchors: get by id failed")
	}
	return &item, nil
}

// GetByID returns a trust anchor by numeric ID.
func (s *TrustAnchorStorage) GetByID(id uint) (*model.TrustAnchor, error) {
	return s.findByID(id)
}

// findByIdent resolves a trust anchor by numeric ID (as string) or entity_id.
func (s *TrustAnchorStorage) findByIdent(ident string) (*model.TrustAnchor, error) {
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		return s.findByID(uint(id))
	}
	return s.findByEntityID(ident)
}

// Create creates a new trust anchor.
func (s *TrustAnchorStorage) Create(req model.AddTrustAnchor) (*model.TrustAnchor, error) {
	if req.EntityID == "" {
		return nil, model.ValidationError("entity_id is required")
	}

	// Check for soft-deleted row with the same entity_id and reactivate it.
	var existing model.TrustAnchor
	result := s.db.Unscoped().Where("entity_id = ?", req.EntityID).First(&existing)
	if result.Error == nil {
		if existing.DeletedAt.Valid {
			existing.DeletedAt = gorm.DeletedAt{}
			existing.EnableJWKSUpdate = req.EnableJWKSUpdate
			existing.KeyPollInterval = req.KeyPollInterval
			if err := s.db.Save(&existing).Error; err != nil {
				return nil, errors.Wrap(err, "trust_anchors: reactivation failed")
			}
			// Replace JWKS if provided.
			if req.JWKS != nil {
				if err := s.replaceJWKS(&existing, req.JWKS); err != nil {
					return nil, err
				}
			}
			return s.findByEntityID(req.EntityID)
		}
		return nil, model.AlreadyExistsError("trust anchor already exists")
	}

	item := &model.TrustAnchor{
		EntityID:         req.EntityID,
		EnableJWKSUpdate: req.EnableJWKSUpdate,
		KeyPollInterval:  req.KeyPollInterval,
	}
	if req.JWKS != nil && (req.JWKS.Keys.Set != nil && req.JWKS.Keys.Len() > 0) {
		item.JWKS = *req.JWKS
	}
	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust anchor already exists")
		}
		return nil, errors.Wrap(err, "trust_anchors: create failed")
	}
	return s.findByEntityID(req.EntityID)
}

// Update updates an existing trust anchor.
func (s *TrustAnchorStorage) Update(entityID string, req model.AddTrustAnchor) (*model.TrustAnchor, error) {
	item, err := s.findByEntityID(entityID)
	if err != nil {
		return nil, err
	}
	item.EnableJWKSUpdate = req.EnableJWKSUpdate
	item.KeyPollInterval = req.KeyPollInterval
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust anchor already exists")
		}
		return nil, errors.Wrap(err, "trust_anchors: update failed")
	}
	if req.JWKS != nil {
		if err := s.replaceJWKS(item, req.JWKS); err != nil {
			return nil, err
		}
	}
	return s.findByEntityID(entityID)
}

// replaceJWKS deletes the old JWKS row (if any) and links a new one.
func (s *TrustAnchorStorage) replaceJWKS(ta *model.TrustAnchor, jwks *model.JWKS) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			// Delete old JWKS if present.
			if ta.JWKSID != nil {
				if err := tx.Unscoped().Delete(&model.JWKS{}, *ta.JWKSID).Error; err != nil {
					return errors.Wrap(err, "failed to delete old JWKS")
				}
				ta.JWKSID = nil
			}
			if jwks != nil && (jwks.Keys.Set != nil && jwks.Keys.Len() > 0) {
				if err := tx.Create(jwks).Error; err != nil {
					return errors.Wrap(err, "failed to create new JWKS")
				}
				ta.JWKSID = &jwks.ID
				if err := tx.Save(ta).Error; err != nil {
					return errors.Wrap(err, "failed to link JWKS to trust anchor")
				}
			}
			return nil
		},
	)
}

// Delete deletes a trust anchor by entity_id, including its JWKS row.
func (s *TrustAnchorStorage) Delete(entityID string) error {
	item, err := s.findByEntityID(entityID)
	if err != nil {
		return err
	}
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			// Permanently delete associated JWKS if present.
			if item.JWKSID != nil {
				if err := tx.Unscoped().Delete(&model.JWKS{}, *item.JWKSID).Error; err != nil {
					return errors.Wrap(err, "failed to permanently delete JWKS")
				}
			}
			if err := tx.Delete(item).Error; err != nil {
				return errors.Wrap(err, "trust_anchors: delete failed")
			}
			return nil
		},
	)
}

// GetJWKS returns the stored JWKS for an entity, or nil if none is stored.
func (s *TrustAnchorStorage) GetJWKS(entityID string) (*jwx.JWKS, error) {
	ta, err := s.findByEntityID(entityID)
	if err != nil {
		if _, ok := err.(model.NotFoundError); ok {
			return nil, nil
		}
		return nil, err
	}
	if ta.JWKSID == nil || ta.JWKS.Keys.Set == nil {
		return nil, nil
	}
	return new(ta.JWKS.Keys), nil
}

// UpdateJWKS stores/replaces the JWKS for an entity. If the entity has no JWKS
// row yet, one is created and linked.
func (s *TrustAnchorStorage) UpdateJWKS(entityID string, jwks jwx.JWKS) error {
	ta, err := s.findByEntityID(entityID)
	if err != nil {
		return err
	}
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			if ta.JWKSID != nil {
				// Update existing JWKS keys in place.
				ta.JWKS.Keys = jwks
				if err := tx.Save(&ta.JWKS).Error; err != nil {
					return errors.Wrap(err, "trust_anchors: failed to update JWKS")
				}
				return nil
			}
			// No JWKS yet: create and link.
			newJWKS := model.JWKS{Keys: jwks}
			if err := tx.Create(&newJWKS).Error; err != nil {
				return errors.Wrap(err, "trust_anchors: failed to create JWKS")
			}
			ta.JWKSID = &newJWKS.ID
			if err := tx.Save(ta).Error; err != nil {
				return errors.Wrap(err, "trust_anchors: failed to link JWKS")
			}
			return nil
		},
	)
}
