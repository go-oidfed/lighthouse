package storage

import (
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// Storage is a GORM-based storage implementation
type Storage struct {
	db *gorm.DB
}

var models = []any{
	&model.SubordinateInfo{},
	&model.SubordinateEvent{},
	&model.Key{},
	&model.JWKS{},
	&model.KeyValue{},
	&model.PolicyOperator{},
	&model.IssuedTrustMarkInstance{},
	&model.TrustMarkType{},
	&model.TrustMarkOwner{},
	&model.TrustMarkTypeIssuer{},
	&model.TrustMarkSpec{},
	&model.TrustMarkSubject{},
	&model.PublishedTrustMark{},
	&model.HistoricalKey{},
	&model.AuthorityHint{},
	&model.SubordinateAdditionalClaim{},
	&model.EntityConfigurationAdditionalClaim{},
}

// NewStorage creates a new GORM-based storage
func NewStorage(config Config) (*Storage, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate the schemas
	if err = db.AutoMigrate(models...); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Storage{db: db}, nil
}

// SubordinateStorage returns a SubordinateStorageBackend
func (s *Storage) SubordinateStorage() *SubordinateStorage {
	return &SubordinateStorage{db: s.db}
}

// TrustMarkedEntitiesStorage returns a TrustMarkedEntitiesStorage
func (s *Storage) TrustMarkedEntitiesStorage() *TrustMarkedEntitiesStorage {
	return &TrustMarkedEntitiesStorage{db: s.db}
}

// AuthorityHintsStorage returns a AuthorityHintsStorage
func (s *Storage) AuthorityHintsStorage() *AuthorityHintsStorage {
	return &AuthorityHintsStorage{db: s.db}
}

// SubordinateStorage implements the SubordinateStorageBackend interface
type SubordinateStorage struct {
	db *gorm.DB
}

// Write stores a model.SubordinateInfo
func (s *SubordinateStorage) Write(_ string, info model.SubordinateInfo) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			// First, check if entity types already exist and use those instead
			for i, entityType := range info.EntityTypes {
				var existingType model.EntityType
				if err := tx.Where("entity_type = ?", entityType.EntityType).First(&existingType).Error; err == nil {
					// If found, use the existing entity type
					info.EntityTypes[i] = existingType
				}
			}

			// Then save the subordinate info
			return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&info).Error
		},
	)
}

// Delete removes a subordinate
func (s *SubordinateStorage) Delete(entityID string) error {
	return s.db.Where("entity_id = ?", entityID).Delete(&model.SubordinateInfo{}).Error
}

// Block marks a subordinate as blocked
func (s *SubordinateStorage) Block(entityID string) error {
	return s.changeStatus(entityID, model.StatusBlocked)
}

// Approve marks a subordinate as active
func (s *SubordinateStorage) Approve(entityID string) error {
	return s.changeStatus(entityID, model.StatusActive)
}

// changeStatus changes the status of a subordinate
func (s *SubordinateStorage) changeStatus(entityID string, status model.Status) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			var dbInfo model.SubordinateInfo
			result := tx.Where("entity_id = ?", entityID).First(&dbInfo)
			if result.Error != nil {
				return model.NotFoundErrorFmt("failed to find entity: %s", result.Error)
			}

			// Update status
			dbInfo.Status = status
			return tx.Save(&dbInfo).Error
		},
	)
}

// Subordinate retrieves a subordinate by entity ID
func (s *SubordinateStorage) Subordinate(entityID string) (*model.SubordinateInfo, error) {
	var dbInfo model.SubordinateInfo
	result := s.db.Where("entity_id = ?", entityID).First(&dbInfo)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find entity: %w", result.Error)
	}

	return &dbInfo, nil
}

// Active returns a query for active subordinates
func (s *SubordinateStorage) Active() model.SubordinateStorageQuery {
	return &GormSubordinateStorageQuery{
		db:     s.db,
		status: model.StatusActive,
	}
}

// Blocked returns a query for blocked subordinates
func (s *SubordinateStorage) Blocked() model.SubordinateStorageQuery {
	return &GormSubordinateStorageQuery{
		db:     s.db,
		status: model.StatusBlocked,
	}
}

// Pending returns a query for pending subordinates
func (s *SubordinateStorage) Pending() model.SubordinateStorageQuery {
	return &GormSubordinateStorageQuery{
		db:     s.db,
		status: model.StatusPending,
	}
}

// Load is a no-op for GORM storage
func (s *SubordinateStorage) Load() error {
	// Nothing to do for GORM as it's already connected to the database
	return nil
}

// GormSubordinateStorageQuery implements a query for subordinates
type GormSubordinateStorageQuery struct {
	db      *gorm.DB
	status  model.Status
	filters []func(info model.SubordinateInfo) bool
}

func (q *GormSubordinateStorageQuery) applyFilter(infos []model.SubordinateInfo) []model.SubordinateInfo {
	var filtered []model.SubordinateInfo
	for _, s := range infos {
		stillOK := true
		for _, f := range q.filters {
			if !f(s) {
				stillOK = false
				break
			}
		}
		if stillOK {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// Subordinates returns all subordinates matching the query
func (q *GormSubordinateStorageQuery) Subordinates() (infos []model.SubordinateInfo, err error) {
	query := q.db.Where("status = ?", q.status)
	err = errors.Wrap(query.Find(&infos).Error, "failed to query subordinates")
	if err != nil {
		return
	}
	infos = q.applyFilter(infos)
	return
}

// EntityIDs returns all entity IDs matching the query
func (q *GormSubordinateStorageQuery) EntityIDs() (entityIDs []string, err error) {
	if len(q.filters) == 0 {
		query := q.db.Model(&model.SubordinateInfo{}).Where("status = ?", q.status)
		err = errors.Wrap(query.Pluck("entity_id", &entityIDs).Error, "failed to query entity IDs")
		return
	}
	infos, err := q.Subordinates()
	if err != nil {
		return nil, err
	}
	entityIDs = make([]string, len(infos))
	for i, info := range infos {
		entityIDs[i] = info.EntityID
	}
	return
}

// AddFilter adds a filter to the query
func (q *GormSubordinateStorageQuery) AddFilter(filter model.SubordinateStorageQueryFilter, value any) error {
	q.filters = append(
		q.filters, func(info model.SubordinateInfo) bool {
			return filter(info, value)
		},
	)
	return nil
}

// TrustMarkedEntitiesStorage implements the TrustMarkedEntitiesStorageBackend interface
type TrustMarkedEntitiesStorage struct {
	db *gorm.DB
}

// Block marks a trust mark as blocked for an entity
func (s *TrustMarkedEntitiesStorage) Block(trustMarkType, entityID string) error {
	return s.writeStatus(trustMarkType, entityID, model.StatusBlocked)
}

// Approve marks a trust mark as active for an entity
func (s *TrustMarkedEntitiesStorage) Approve(trustMarkType, entityID string) error {
	return s.writeStatus(trustMarkType, entityID, model.StatusActive)
}

// Request marks a trust mark as pending for an entity
func (s *TrustMarkedEntitiesStorage) Request(trustMarkType, entityID string) error {
	return s.writeStatus(trustMarkType, entityID, model.StatusPending)
}

// TrustMarkedStatus returns the status of a trust mark for an entity
func (s *TrustMarkedEntitiesStorage) TrustMarkedStatus(trustMarkType, entityID string) (model.Status, error) {
	var entity model.TrustMarkSubject
	err := s.db.Where("trust_mark_type = ? AND entity_id = ?", trustMarkType, entityID).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.StatusInactive, nil
		}
		return model.StatusInactive, errors.Wrap(err, "failed to get trust mark status")
	}

	return entity.Status, nil
}

// Active returns all active entities for a trust mark type
func (s *TrustMarkedEntitiesStorage) Active(trustMarkType string) ([]string, error) {
	return s.trustMarkedEntities(trustMarkType, model.StatusActive)
}

// Blocked returns all blocked entities for a trust mark type
func (s *TrustMarkedEntitiesStorage) Blocked(trustMarkType string) ([]string, error) {
	return s.trustMarkedEntities(trustMarkType, model.StatusBlocked)
}

// Pending returns all pending entities for a trust mark type
func (s *TrustMarkedEntitiesStorage) Pending(trustMarkType string) ([]string, error) {
	return s.trustMarkedEntities(trustMarkType, model.StatusPending)
}

// Delete removes a trust mark for an entity
func (s *TrustMarkedEntitiesStorage) Delete(trustMarkType, entityID string) error {
	err := s.db.Where(
		"trust_mark_type = ? AND entity_id = ?", trustMarkType, entityID,
	).Delete(&model.TrustMarkSubject{}).Error
	if err != nil {
		return errors.Wrap(err, "failed to delete trust marked entity")
	}
	return nil
}

// Load is a no-op for GORM storage
func (s *TrustMarkedEntitiesStorage) Load() error {
	// Nothing to do for GORM as it's already connected to the database
	return nil
}

// HasTrustMark checks if an entity has an active trust mark
func (s *TrustMarkedEntitiesStorage) HasTrustMark(trustMarkType, entityID string) (bool, error) {
	var count int64
	if err := s.db.Model(&model.TrustMarkSubject{}).
		Where("trust_mark_type = ? AND entity_id = ? AND status = ?", trustMarkType, entityID, model.StatusActive).
		Count(&count).Error; err != nil {
		return false, errors.Wrap(err, "failed to check if entity has trust mark")
	}

	return count > 0, nil
}

// writeStatus updates or creates a trust mark entity
func (s *TrustMarkedEntitiesStorage) writeStatus(trustMarkType, entityID string, status model.Status) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			var dbTrustMarkSpec model.TrustMarkSpec
			if err := tx.Where("trust_mark_type = ?", trustMarkType).First(&dbTrustMarkSpec).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return model.NotFoundErrorFmt("unknown trust mark type: %s", trustMarkType)
				}
				return err
			}

			entity := model.TrustMarkSubject{
				TrustMarkSpecID: dbTrustMarkSpec.ID,
				EntityID:        entityID,
				Status:          status,
			}
			return tx.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"status"})}).Create(&entity).Error
		},
	)
}

// trustMarkedEntities returns entities with a specific trust mark and status
func (s *TrustMarkedEntitiesStorage) trustMarkedEntities(trustMarkType string, status model.Status) (
	[]string, error,
) {
	var entityIDs []string
	query := s.db.Model(&model.TrustMarkSubject{}).Where("status = ?", status)

	if trustMarkType != "" {
		query = query.Where("trust_mark_type = ?", trustMarkType)
	}

	if err := query.Pluck("entity_id", &entityIDs).Error; err != nil {
		return nil, errors.Wrap(err, "failed to get trust marked entities")
	}

	return entityIDs, nil
}
