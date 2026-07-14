package storage

import (
	"strconv"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// FederationEndpointStorage implements model.FederationEndpointStore using GORM.
type FederationEndpointStorage struct {
	db *gorm.DB
}

// NewFederationEndpointStorage creates a new FederationEndpointStorage.
func NewFederationEndpointStorage(db *gorm.DB) *FederationEndpointStorage {
	return &FederationEndpointStorage{db: db}
}

// List returns all federation endpoints with auth trust anchors preloaded.
func (s *FederationEndpointStorage) List() ([]model.FederationEndpoint, error) {
	var items []model.FederationEndpoint
	if err := s.db.Preload("AuthTrustAnchors").Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "federation_endpoints: list failed")
	}
	return items, nil
}

// findByType finds a federation endpoint by type with preloaded auth trust anchors.
func (s *FederationEndpointStorage) findByType(t model.FederationEndpointType) (*model.FederationEndpoint, error) {
	var item model.FederationEndpoint
	if err := s.db.Preload("AuthTrustAnchors").Where("type = ?", t).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("federation endpoint not found")
		}
		return nil, errors.Wrap(err, "federation_endpoints: get by type failed")
	}
	return &item, nil
}

// GetByType returns a federation endpoint by type.
func (s *FederationEndpointStorage) GetByType(t model.FederationEndpointType) (*model.FederationEndpoint, error) {
	return s.findByType(t)
}

// findByPath finds a federation endpoint whose path matches. Returns a
// NotFoundError if no endpoint has that path (or if the matching endpoint has
// a nil path, which cannot happen since the column stores the value).
func (s *FederationEndpointStorage) findByPath(path string) (*model.FederationEndpoint, error) {
	var item model.FederationEndpoint
	if err := s.db.Preload("AuthTrustAnchors").Where("path = ?", path).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("federation endpoint not found")
		}
		return nil, errors.Wrap(err, "federation_endpoints: get by path failed")
	}
	return &item, nil
}

// GetByPath returns a federation endpoint by path.
func (s *FederationEndpointStorage) GetByPath(path string) (*model.FederationEndpoint, error) {
	return s.findByPath(path)
}

// findByID finds a federation endpoint by primary key with preloaded auth trust anchors.
func (s *FederationEndpointStorage) findByID(id uint) (*model.FederationEndpoint, error) {
	var item model.FederationEndpoint
	if err := s.db.Preload("AuthTrustAnchors").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("federation endpoint not found")
		}
		return nil, errors.Wrap(err, "federation_endpoints: get by id failed")
	}
	return &item, nil
}

// GetByID returns a federation endpoint by numeric ID.
func (s *FederationEndpointStorage) GetByID(id uint) (*model.FederationEndpoint, error) {
	return s.findByID(id)
}

// Create creates a new federation endpoint.
func (s *FederationEndpointStorage) Create(req model.AddFederationEndpoint) (*model.FederationEndpoint, error) {
	if !model.IsValidFederationEndpointType(req.Type) {
		return nil, model.ValidationErrorFmt("invalid endpoint type: %s", req.Type)
	}
	// Resolve auth trust anchor rows.
	authTAs, err := s.resolveAuthTrustAnchors(req.AuthTrustAnchors)
	if err != nil {
		return nil, err
	}

	// Reactivate soft-deleted row with the same type if present.
	var existing model.FederationEndpoint
	result := s.db.Unscoped().Where("type = ?", req.Type).First(&existing)
	if result.Error == nil {
		if existing.DeletedAt.Valid {
			existing.DeletedAt = gorm.DeletedAt{}
			existing.Path = req.Path
			existing.URL = req.URL
			existing.AuthEnabled = req.AuthEnabled
			existing.Config = req.Config
			if err := s.db.Save(&existing).Error; err != nil {
				return nil, errors.Wrap(err, "federation_endpoints: reactivation failed")
			}
			if err := s.db.Model(&existing).Association("AuthTrustAnchors").Replace(authTAs); err != nil {
				return nil, errors.Wrap(err, "federation_endpoints: reactivation set auth trust anchors failed")
			}
			return s.findByType(req.Type)
		}
		return nil, model.AlreadyExistsError("federation endpoint already exists")
	}

	item := &model.FederationEndpoint{
		Type:        req.Type,
		Path:        req.Path,
		URL:         req.URL,
		AuthEnabled: req.AuthEnabled,
		Config:      req.Config,
	}
	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("federation endpoint already exists")
		}
		return nil, errors.Wrap(err, "federation_endpoints: create failed")
	}
	if len(authTAs) > 0 {
		if err := s.db.Model(item).Association("AuthTrustAnchors").Replace(authTAs); err != nil {
			return nil, errors.Wrap(err, "federation_endpoints: set auth trust anchors failed")
		}
	}
	return s.findByType(req.Type)
}

// Update updates an existing federation endpoint.
func (s *FederationEndpointStorage) Update(t model.FederationEndpointType, req model.AddFederationEndpoint) (*model.FederationEndpoint, error) {
	item, err := s.findByType(t)
	if err != nil {
		return nil, err
	}
	item.Path = req.Path
	item.URL = req.URL
	item.AuthEnabled = req.AuthEnabled
	item.Config = req.Config
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("federation endpoint already exists")
		}
		return nil, errors.Wrap(err, "federation_endpoints: update failed")
	}
	authTAs, err := s.resolveAuthTrustAnchors(req.AuthTrustAnchors)
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(item).Association("AuthTrustAnchors").Replace(authTAs); err != nil {
		return nil, errors.Wrap(err, "federation_endpoints: update auth trust anchors failed")
	}
	return s.findByType(t)
}

// Delete deletes a federation endpoint by type.
// Join table rows are preserved on soft delete; they are cleaned up
// automatically by FK ON DELETE CASCADE on hard delete.
func (s *FederationEndpointStorage) Delete(t model.FederationEndpointType) error {
	item, err := s.findByType(t)
	if err != nil {
		return err
	}
	if err := s.db.Delete(item).Error; err != nil {
		return errors.Wrap(err, "federation_endpoints: delete failed")
	}
	return nil
}

// SetAuthTrustAnchors replaces the auth trust anchors for an endpoint.
func (s *FederationEndpointStorage) SetAuthTrustAnchors(t model.FederationEndpointType, trustAnchorIDs []uint) ([]model.TrustAnchor, error) {
	item, err := s.findByType(t)
	if err != nil {
		return nil, err
	}
	authTAs, err := s.resolveAuthTrustAnchors(trustAnchorIDs)
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(item).Association("AuthTrustAnchors").Replace(authTAs); err != nil {
		return nil, errors.Wrap(err, "federation_endpoints: set auth trust anchors failed")
	}
	return authTAs, nil
}

// resolveAuthTrustAnchors loads trust anchor rows by ID, skipping unknowns.
func (s *FederationEndpointStorage) resolveAuthTrustAnchors(ids []uint) ([]model.TrustAnchor, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var tas []model.TrustAnchor
	if err := s.db.Where("id IN ?", ids).Find(&tas).Error; err != nil {
		return nil, errors.Wrap(err, "federation_endpoints: resolve auth trust anchors failed")
	}
	if len(tas) != len(ids) {
		// Build a set of found IDs to report the missing ones.
		found := make(map[uint]bool, len(tas))
		for _, ta := range tas {
			found[ta.ID] = true
		}
		var missing []string
		for _, id := range ids {
			if !found[id] {
				missing = append(missing, strconv.FormatUint(uint64(id), 10))
			}
		}
		return nil, model.NotFoundErrorFmt("trust anchor(s) not found: %s", missing)
	}
	return tas, nil
}
