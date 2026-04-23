package storage

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/internal/stats"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// Storage is a GORM-based storage implementation
type Storage struct {
	db         *gorm.DB
	userParams Argon2idParams
}

var models = []any{
	&model.ExtendedSubordinateInfo{},
	&model.SubordinateEntityType{},
	&model.SubordinateEvent{},
	&model.JWKS{},
	&model.KeyValue{},
	&model.PolicyOperator{},
	&model.IssuedTrustMarkInstance{},
	&model.TrustMarkType{},
	&model.TrustMarkOwner{},
	&model.TrustMarkIssuer{},
	&model.TrustMarkSpec{},
	&model.TrustMarkSubject{},
	&model.PublishedTrustMark{},
	&model.HistoricalKey{},
	&model.AuthorityHint{},
	&model.SubordinateAdditionalClaim{},
	&model.EntityConfigurationAdditionalClaim{},
	&model.User{},
}

// statsModels contains models for the stats feature.
// These are migrated separately when stats is enabled.
var statsModels = []any{
	&stats.RequestLog{},
	&stats.DailyStats{},
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

	// Fill user hash params with defaults if zero values
	params := config.UsersHash
	if params.Time == 0 {
		params = defaultArgon2idParams()
	}

	return &Storage{
		db:         db,
		userParams: params,
	}, nil
}

// MigrateStats migrates the stats-related tables.
// This is called separately when stats collection is enabled.
func MigrateStats(db *gorm.DB) error {
	return db.AutoMigrate(statsModels...)
}

// MigrateStatsFromBackends migrates stats tables using a StatsStorage backend.
// This is a convenience function for when you only have access to Backends.
func MigrateStatsFromBackends(backends model.Backends) error {
	if backends.Stats == nil {
		return nil
	}
	// Try to get the underlying db from the stats storage
	if ss, ok := backends.Stats.(*StatsStorage); ok {
		return MigrateStats(ss.db)
	}
	return nil
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

// TrustMarkTypesStorage returns a TrustMarkTypesStorage
func (s *Storage) TrustMarkTypesStorage() *TrustMarkTypesStorage {
	return &TrustMarkTypesStorage{db: s.db}
}

// TrustMarkOwnersStorage returns a TrustMarkOwnersStorage
func (s *Storage) TrustMarkOwnersStorage() *TrustMarkOwnersStorage {
	return &TrustMarkOwnersStorage{db: s.db}
}

// TrustMarkIssuersStorage returns a TrustMarkIssuersStorage
func (s *Storage) TrustMarkIssuersStorage() *TrustMarkIssuersStorage {
	return &TrustMarkIssuersStorage{db: s.db}
}

// SubordinateEventsStorage returns a SubordinateEventsStorage
func (s *Storage) SubordinateEventsStorage() *SubordinateEventsStorage {
	return NewSubordinateEventsStorage(s.db)
}

// DBPublicKeyStorage returns a DBPublicKeyStorage
func (s *Storage) DBPublicKeyStorage(typeID string) *DBPublicKeyStorage {
	return NewDBPublicKeyStorage(s.db, typeID)
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
	err := s.db.
		Joins("JOIN trust_mark_specs ON trust_mark_specs.id = trust_mark_subjects.trust_mark_spec_id").
		Where("trust_mark_specs.trust_mark_type = ? AND trust_mark_subjects.entity_id = ?", trustMarkType, entityID).
		First(&entity).Error
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
	// First find the TrustMarkSpec by type
	var spec model.TrustMarkSpec
	if err := s.db.Where("trust_mark_type = ?", trustMarkType).First(&spec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // Nothing to delete if spec doesn't exist
		}
		return errors.Wrap(err, "failed to find trust mark spec")
	}
	// Delete the subject
	err := s.db.Where(
		"trust_mark_spec_id = ? AND entity_id = ?", spec.ID, entityID,
	).Delete(&model.TrustMarkSubject{}).Error
	if err != nil {
		return errors.Wrap(err, "failed to delete trust marked entity")
	}
	return nil
}

// Load is a no-op for GORM storage
func (*TrustMarkedEntitiesStorage) Load() error {
	// Nothing to do for GORM as it's already connected to the database
	return nil
}

// HasTrustMark checks if an entity has an active trust mark
func (s *TrustMarkedEntitiesStorage) HasTrustMark(trustMarkType, entityID string) (bool, error) {
	var count int64
	if err := s.db.Model(&model.TrustMarkSubject{}).
		Joins("JOIN trust_mark_specs ON trust_mark_specs.id = trust_mark_subjects.trust_mark_spec_id").
		Where("trust_mark_specs.trust_mark_type = ? AND trust_mark_subjects.entity_id = ? AND trust_mark_subjects.status = ?", trustMarkType, entityID, model.StatusActive).
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
			return tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "trust_mark_spec_id"}, {Name: "entity_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"status"}),
			}).Create(&entity).Error
		},
	)
}

// trustMarkedEntities returns entities with a specific trust mark and status
func (s *TrustMarkedEntitiesStorage) trustMarkedEntities(trustMarkType string, status model.Status) (
	[]string, error,
) {
	var entityIDs []string
	query := s.db.Model(&model.TrustMarkSubject{}).Where("trust_mark_subjects.status = ?", status)

	if trustMarkType != "" {
		query = query.
			Joins("JOIN trust_mark_specs ON trust_mark_specs.id = trust_mark_subjects.trust_mark_spec_id").
			Where("trust_mark_specs.trust_mark_type = ?", trustMarkType)
	}

	if err := query.Pluck("trust_mark_subjects.entity_id", &entityIDs).Error; err != nil {
		return nil, errors.Wrap(err, "failed to get trust marked entities")
	}

	return entityIDs, nil
}

// TrustMarkTypesStorage provides CRUD and relations for TrustMarkType, owner and issuers.
type TrustMarkTypesStorage struct {
	db *gorm.DB
}

// findTypeByIdent tries numeric ID first, then trust_mark_type string.
func (s *TrustMarkTypesStorage) findTypeByIdent(ident string) (*model.TrustMarkType, error) {
	var item model.TrustMarkType
	// Try numeric ID
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}
	// Fallback to trust_mark_type match
	if err := s.db.Where("trust_mark_type = ?", ident).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark type not found")
		}
		return nil, errors.Wrap(err, "trust_mark_types: get failed")
	}
	return &item, nil
}

func (s *TrustMarkTypesStorage) List() ([]model.TrustMarkType, error) {
	var items []model.TrustMarkType
	if err := s.db.Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: list failed")
	}
	return items, nil
}

func (s *TrustMarkTypesStorage) Create(req model.AddTrustMarkType) (*model.TrustMarkType, error) {
	item := &model.TrustMarkType{
		TrustMarkType: req.TrustMarkType,
		Description:   req.Description,
	}
	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark type already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_types: create failed")
	}
	// Optional owner
	if req.TrustMarkOwner != nil {
		if _, err := s.CreateOwner(strconv.FormatUint(uint64(item.ID), 10), *req.TrustMarkOwner); err != nil {
			return nil, err
		}
	}
	// Optional issuers
	if len(req.TrustMarkIssuers) > 0 {
		if _, err := s.SetIssuers(strconv.FormatUint(uint64(item.ID), 10), req.TrustMarkIssuers); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s *TrustMarkTypesStorage) Get(ident string) (*model.TrustMarkType, error) {
	return s.findTypeByIdent(ident)
}

func (s *TrustMarkTypesStorage) Update(ident string, req model.AddTrustMarkType) (*model.TrustMarkType, error) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	item.TrustMarkType = req.TrustMarkType
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark type already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_types: update failed")
	}
	return item, nil
}

func (s *TrustMarkTypesStorage) Delete(ident string) error {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return err
	}
	// Null owner
	item.OwnerID = nil
	item.Owner = nil
	if err = s.db.Save(item).Error; err != nil {
		return errors.Wrap(err, "trust_mark_types: clear owner failed")
	}
	if err = s.db.Delete(item).Error; err != nil {
		return errors.Wrap(err, "trust_mark_types: delete failed")
	}
	return nil
}

// OwnersByType returns a map of trust_mark_type -> TrustMarkOwner for all types that have an owner.
func (s *TrustMarkTypesStorage) OwnersByType() (oidfed.TrustMarkOwners, error) {
	var types []model.TrustMarkType
	if err := s.db.Where("owner_id IS NOT NULL").Find(&types).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: list owners by type failed")
	}
	out := make(oidfed.TrustMarkOwners, len(types))
	for _, t := range types {
		var owner model.TrustMarkOwner
		if err := s.db.Preload("JWKS").First(&owner, *t.OwnerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Skip missing owner rows gracefully
				continue
			}
			return nil, errors.Wrap(err, "trust_mark_types: get owner failed")
		}
		out[t.TrustMarkType] = oidfed.TrustMarkOwnerSpec{
			ID:   owner.EntityID,
			JWKS: owner.JWKS.Keys,
		}
	}
	return out, nil
}

// IssuersByType returns a map of trust_mark_type -> []issuer (entity IDs) for all types.
func (s *TrustMarkTypesStorage) IssuersByType() (oidfed.AllowedTrustMarkIssuers, error) {
	var types []model.TrustMarkType
	if err := s.db.Preload("Issuers").Find(&types).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: list issuers by type failed")
	}
	out := make(oidfed.AllowedTrustMarkIssuers)
	for _, t := range types {
		for _, iss := range t.Issuers {
			out[t.TrustMarkType] = append(out[t.TrustMarkType], iss.Issuer)
		}
	}
	return out, nil
}

// Issuers management
func (s *TrustMarkTypesStorage) ListIssuers(ident string) ([]model.TrustMarkIssuer, error) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	var typ model.TrustMarkType
	if err = s.db.Preload("Issuers").First(&typ, item.ID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: list issuers failed")
	}
	return typ.Issuers, nil
}

func (s *TrustMarkTypesStorage) SetIssuers(ident string, in []model.AddTrustMarkIssuer) (
	[]model.TrustMarkIssuer, error,
) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	// Resolve all issuers
	issuers := make([]model.TrustMarkIssuer, 0, len(in))
	for _, iss := range in {
		issuerID, err := s.resolveIssuerID(iss)
		if err != nil {
			return nil, err
		}
		var issuer model.TrustMarkIssuer
		if err = s.db.First(&issuer, issuerID).Error; err != nil {
			return nil, errors.Wrap(err, "trust_mark_types: resolve issuer row failed")
		}
		issuers = append(issuers, issuer)
	}
	// Replace association
	if err = s.db.Model(&model.TrustMarkType{ID: item.ID}).Association("Issuers").Replace(issuers); err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: set issuers failed")
	}
	return s.ListIssuers(ident)
}

func (s *TrustMarkTypesStorage) AddIssuer(ident string, issuer model.AddTrustMarkIssuer) (
	[]model.TrustMarkIssuer, error,
) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	issuerID, err := s.resolveIssuerID(issuer)
	if err != nil {
		return nil, err
	}
	var issuerRow model.TrustMarkIssuer
	if err = s.db.First(&issuerRow, issuerID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: resolve issuer row failed")
	}
	if err = s.db.Model(&model.TrustMarkType{ID: item.ID}).Association("Issuers").Append(&issuerRow); err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: add issuer failed")
	}
	return s.ListIssuers(ident)
}

func (s *TrustMarkTypesStorage) DeleteIssuerByID(ident string, issuerID uint) ([]model.TrustMarkIssuer, error) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	var issuerRow model.TrustMarkIssuer
	if err = s.db.First(&issuerRow, issuerID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: resolve issuer row failed")
	}
	if err = s.db.Model(&model.TrustMarkType{ID: item.ID}).Association("Issuers").Delete(&issuerRow); err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: delete issuer failed")
	}
	return s.ListIssuers(ident)
}

// resolveIssuerID finds or creates a global issuer based on the request
func (s *TrustMarkTypesStorage) resolveIssuerID(req model.AddTrustMarkIssuer) (uint, error) {
	if req.IssuerID != nil {
		var issuer model.TrustMarkIssuer
		if err := s.db.First(&issuer, *req.IssuerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, model.NotFoundError("issuer not found")
			}
			return 0, errors.Wrap(err, "trust_mark_types: resolve issuer id failed")
		}
		return issuer.ID, nil
	}
	if req.Issuer == "" {
		return 0, model.NotFoundError("issuer not specified")
	}
	var existing model.TrustMarkIssuer
	if err := s.db.Where("issuer = ?", req.Issuer).First(&existing).Error; err == nil {
		return existing.ID, nil
	}
	// Create new
	newIss := &model.TrustMarkIssuer{
		Issuer:      req.Issuer,
		Description: req.Description,
	}
	if err := s.db.Create(newIss).Error; err != nil {
		if isUniqueConstraintError(err) {
			if er2 := s.db.Where("issuer = ?", req.Issuer).First(&existing).Error; er2 == nil {
				return existing.ID, nil
			}
		}
		return 0, errors.Wrap(err, "trust_mark_types: create global issuer failed")
	}
	return newIss.ID, nil
}

// Owner management
func (s *TrustMarkTypesStorage) GetOwner(ident string) (*model.TrustMarkOwner, error) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	if item.OwnerID == nil {
		return nil, model.NotFoundError("trust mark owner not set")
	}
	var owner model.TrustMarkOwner
	if err = s.db.First(&owner, *item.OwnerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark owner not found")
		}
		return nil, errors.Wrap(err, "trust_mark_types: get owner failed")
	}
	return &owner, nil
}

func (s *TrustMarkTypesStorage) CreateOwner(ident string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	var owner model.TrustMarkOwner
	if req.OwnerID != nil {
		// Link existing owner
		if err = s.db.First(&owner, *req.OwnerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, model.NotFoundError("trust mark owner not found")
			}
			return nil, errors.Wrap(err, "trust_mark_types: get owner failed")
		}
	} else {
		// Create new owner row
		newOwner := &model.TrustMarkOwner{
			EntityID: req.EntityID,
			JWKS:     req.JWKS,
		}
		if err = s.db.Create(newOwner).Error; err != nil {
			if isUniqueConstraintError(err) {
				return nil, model.AlreadyExistsError("trust mark owner already exists")
			}
			return nil, errors.Wrap(err, "trust_mark_types: create owner failed")
		}
		owner = *newOwner
	}
	// Attach to type
	item.OwnerID = &owner.ID
	item.Owner = &owner
	if err = s.db.Save(item).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_types: attach owner failed")
	}
	return &owner, nil
}

func (s *TrustMarkTypesStorage) UpdateOwner(ident string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return nil, err
	}
	if req.OwnerID != nil {
		// Relink to another existing owner
		var newOwner model.TrustMarkOwner
		if err = s.db.First(&newOwner, *req.OwnerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, model.NotFoundError("trust mark owner not found")
			}
			return nil, errors.Wrap(err, "trust_mark_types: get owner failed")
		}
		item.OwnerID = &newOwner.ID
		item.Owner = &newOwner
		if err = s.db.Save(item).Error; err != nil {
			return nil, errors.Wrap(err, "trust_mark_types: relink owner failed")
		}
		return &newOwner, nil
	}
	// Update the currently linked owner
	if item.OwnerID == nil {
		return nil, model.NotFoundError("trust mark owner not set")
	}
	var owner model.TrustMarkOwner
	if err = s.db.First(&owner, *item.OwnerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark owner not found")
		}
		return nil, errors.Wrap(err, "trust_mark_types: get owner failed")
	}
	owner.EntityID = req.EntityID
	owner.JWKS = req.JWKS
	if err = s.db.Save(&owner).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark owner already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_types: update owner failed")
	}
	return &owner, nil
}

func (s *TrustMarkTypesStorage) DeleteOwner(ident string) error {
	item, err := s.findTypeByIdent(ident)
	if err != nil {
		return err
	}
	if item.OwnerID == nil {
		return nil
	}
	// Delete owner row and detach
	if err = s.db.Delete(&model.TrustMarkOwner{}, *item.OwnerID).Error; err != nil {
		return errors.Wrap(err, "trust_mark_types: delete owner failed")
	}
	item.OwnerID = nil
	item.Owner = nil
	if err = s.db.Save(item).Error; err != nil {
		return errors.Wrap(err, "trust_mark_types: clear owner failed")
	}
	return nil
}

// TrustMarkOwnersStorage provides CRUD and relation management for global owners
type TrustMarkOwnersStorage struct {
	db *gorm.DB
}

func (s *TrustMarkOwnersStorage) List() ([]model.TrustMarkOwner, error) {
	var items []model.TrustMarkOwner
	if err := s.db.Preload("JWKS").Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_owners: list failed")
	}
	return items, nil
}

func (s *TrustMarkOwnersStorage) Create(req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	item := &model.TrustMarkOwner{
		EntityID: req.EntityID,
		JWKS:     req.JWKS,
	}
	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark owner already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_owners: create failed")
	}
	return item, nil
}

func (s *TrustMarkOwnersStorage) findByIdent(ident string) (*model.TrustMarkOwner, error) {
	var item model.TrustMarkOwner
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.Preload("JWKS").First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}
	if err := s.db.Preload("JWKS").Where("entity_id = ?", ident).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark owner not found")
		}
		return nil, errors.Wrap(err, "trust_mark_owners: get failed")
	}
	return &item, nil
}

func (s *TrustMarkOwnersStorage) Get(ident string) (*model.TrustMarkOwner, error) {
	return s.findByIdent(ident)
}

func (s *TrustMarkOwnersStorage) Update(ident string, req model.AddTrustMarkOwner) (*model.TrustMarkOwner, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	item.EntityID = req.EntityID
	item.JWKS = req.JWKS
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark owner already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_owners: update failed")
	}
	return item, nil
}

func (s *TrustMarkOwnersStorage) Delete(ident string) error {
	item, err := s.findByIdent(ident)
	if err != nil {
		return err
	}
	if err = s.db.Delete(item).Error; err != nil {
		return errors.Wrap(err, "trust_mark_owners: delete failed")
	}
	return nil
}

func (s *TrustMarkOwnersStorage) Types(ident string) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err = s.db.Model(&model.TrustMarkType{}).
		Where("owner_id = ?", item.ID).
		Pluck("id", &ids).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_owners: list types failed")
	}
	return ids, nil
}

func (s *TrustMarkOwnersStorage) SetTypes(ident string, typeIdents []string) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	if err = s.db.Model(&model.TrustMarkType{}).
		Where("owner_id = ?", item.ID).
		Update("owner_id", nil).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_owners: clear types failed")
	}
	for _, ident := range typeIdents {
		var t model.TrustMarkType
		// Resolve by numeric ID or trust_mark_type string
		if id, er := strconv.ParseUint(ident, 10, 64); er == nil {
			if err = s.db.First(&t, uint(id)).Error; err != nil {
				return nil, errors.Wrap(err, "trust_mark_owners: resolve type id failed")
			}
		} else {
			if err = s.db.Where("trust_mark_type = ?", ident).First(&t).Error; err != nil {
				return nil, errors.Wrap(err, "trust_mark_owners: resolve type ident failed")
			}
		}
		if err = s.db.Model(&model.TrustMarkType{}).
			Where("id = ?", t.ID).
			Update("owner_id", item.ID).Error; err != nil {
			return nil, errors.Wrap(err, "trust_mark_owners: set type failed")
		}
	}
	return s.Types(ident)
}

func (s *TrustMarkOwnersStorage) AddType(ident string, typeID uint) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	if err = s.db.Model(&model.TrustMarkType{}).
		Where("id = ?", typeID).
		Update("owner_id", item.ID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_owners: add type failed")
	}
	return s.Types(ident)
}

func (s *TrustMarkOwnersStorage) DeleteType(ident string, typeID uint) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	if err = s.db.Model(&model.TrustMarkType{}).
		Where("id = ? AND owner_id = ?", typeID, item.ID).
		Update("owner_id", nil).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_owners: delete type failed")
	}
	return s.Types(ident)
}

// TrustMarkIssuersStorage provides CRUD and relation management for global issuers
type TrustMarkIssuersStorage struct {
	db *gorm.DB
}

func (s *TrustMarkIssuersStorage) List() ([]model.TrustMarkIssuer, error) {
	var items []model.TrustMarkIssuer
	if err := s.db.Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: list failed")
	}
	return items, nil
}

func (s *TrustMarkIssuersStorage) Create(req model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
	if req.Issuer == "" {
		return nil, model.AlreadyExistsError("issuer is required")
	}
	item := &model.TrustMarkIssuer{
		Issuer:      req.Issuer,
		Description: req.Description,
	}
	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark issuer already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_issuers: create failed")
	}
	return item, nil
}

func (s *TrustMarkIssuersStorage) findByIdent(ident string) (*model.TrustMarkIssuer, error) {
	var item model.TrustMarkIssuer
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}
	if err := s.db.Where("issuer = ?", ident).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark issuer not found")
		}
		return nil, errors.Wrap(err, "trust_mark_issuers: get failed")
	}
	return &item, nil
}

func (s *TrustMarkIssuersStorage) Get(ident string) (*model.TrustMarkIssuer, error) {
	return s.findByIdent(ident)
}

func (s *TrustMarkIssuersStorage) Update(ident string, req model.AddTrustMarkIssuer) (*model.TrustMarkIssuer, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	if req.Issuer != "" {
		item.Issuer = req.Issuer
	}
	item.Description = req.Description
	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark issuer already exists")
		}
		return nil, errors.Wrap(err, "trust_mark_issuers: update failed")
	}
	return item, nil
}

func (s *TrustMarkIssuersStorage) Delete(ident string) error {
	item, err := s.findByIdent(ident)
	if err != nil {
		return err
	}
	if err = s.db.Delete(item).Error; err != nil {
		return errors.Wrap(err, "trust_mark_issuers: delete failed")
	}
	return nil
}

func (s *TrustMarkIssuersStorage) Types(ident string) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	var issuer model.TrustMarkIssuer
	if err = s.db.Preload("Types").First(&issuer, item.ID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: list types failed")
	}
	ids := make([]uint, len(issuer.Types))
	for i, t := range issuer.Types {
		ids[i] = t.ID
	}
	return ids, nil
}

func (s *TrustMarkIssuersStorage) SetTypes(ident string, typeIdents []string) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	types := make([]model.TrustMarkType, 0, len(typeIdents))
	for _, ident := range typeIdents {
		var t model.TrustMarkType
		if id, er := strconv.ParseUint(ident, 10, 64); er == nil {
			if err = s.db.First(&t, uint(id)).Error; err != nil {
				return nil, errors.Wrap(err, "trust_mark_issuers: resolve type id failed")
			}
		} else {
			if err = s.db.Where("trust_mark_type = ?", ident).First(&t).Error; err != nil {
				return nil, errors.Wrap(err, "trust_mark_issuers: resolve type ident failed")
			}
		}
		types = append(types, t)
	}
	if err = s.db.Model(&model.TrustMarkIssuer{ID: item.ID}).Association("Types").Replace(types); err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: set type failed")
	}
	return s.Types(ident)
}

func (s *TrustMarkIssuersStorage) AddType(ident string, typeID uint) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	var t model.TrustMarkType
	if err = s.db.First(&t, typeID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: resolve type id failed")
	}
	if err = s.db.Model(&model.TrustMarkIssuer{ID: item.ID}).Association("Types").Append(&t); err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: add type failed")
	}
	return s.Types(ident)
}

func (s *TrustMarkIssuersStorage) DeleteType(ident string, typeID uint) ([]uint, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	var t model.TrustMarkType
	if err = s.db.First(&t, typeID).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: resolve type id failed")
	}
	if err = s.db.Model(&model.TrustMarkIssuer{ID: item.ID}).Association("Types").Delete(&t); err != nil {
		return nil, errors.Wrap(err, "trust_mark_issuers: delete type failed")
	}
	return s.Types(ident)
}

// TrustMarkSpecStorage returns a TrustMarkSpecStorage
func (s *Storage) TrustMarkSpecStorage() *TrustMarkSpecStorage {
	return &TrustMarkSpecStorage{db: s.db}
}

// TrustMarkSpecStorage provides CRUD for TrustMarkSpec entities
type TrustMarkSpecStorage struct {
	db *gorm.DB
}

// List returns all TrustMarkSpecs
func (s *TrustMarkSpecStorage) List() ([]model.TrustMarkSpec, error) {
	var items []model.TrustMarkSpec
	if err := s.db.Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_specs: list failed")
	}
	return items, nil
}

// Create creates a new TrustMarkSpec
func (s *TrustMarkSpecStorage) Create(spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
	record := &model.TrustMarkSpec{
		TrustMarkType:     spec.TrustMarkType,
		Lifetime:          spec.Lifetime,
		Ref:               spec.Ref,
		LogoURI:           spec.LogoURI,
		DelegationJWT:     spec.DelegationJWT,
		AdditionalClaims:  spec.AdditionalClaims,
		Description:       spec.Description,
		EligibilityConfig: spec.EligibilityConfig,
		CacheTTL:          spec.CacheTTL,
	}
	if err := s.db.Create(record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark spec already exists for this type")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: create failed")
	}
	return record, nil
}

// findByIdent finds a TrustMarkSpec by ID or trust_mark_type
func (s *TrustMarkSpecStorage) findByIdent(ident string) (*model.TrustMarkSpec, error) {
	var item model.TrustMarkSpec
	// Try numeric ID first
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}
	// Fallback to trust_mark_type
	if err := s.db.Where("trust_mark_type = ?", ident).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark spec not found")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: get failed")
	}
	return &item, nil
}

// Get returns a TrustMarkSpec by ID or trust_mark_type
func (s *TrustMarkSpecStorage) Get(ident string) (*model.TrustMarkSpec, error) {
	return s.findByIdent(ident)
}

// GetByType returns a TrustMarkSpec by trust_mark_type
func (s *TrustMarkSpecStorage) GetByType(trustMarkType string) (*model.TrustMarkSpec, error) {
	var item model.TrustMarkSpec
	if err := s.db.Where("trust_mark_type = ?", trustMarkType).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark spec not found")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: get by type failed")
	}
	return &item, nil
}

// Update updates an existing TrustMarkSpec (full replacement)
func (s *TrustMarkSpecStorage) Update(ident string, spec *model.AddTrustMarkSpec) (*model.TrustMarkSpec, error) {
	existing, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	existing.TrustMarkType = spec.TrustMarkType
	existing.Lifetime = spec.Lifetime
	existing.Ref = spec.Ref
	existing.LogoURI = spec.LogoURI
	existing.DelegationJWT = spec.DelegationJWT
	existing.AdditionalClaims = spec.AdditionalClaims
	existing.Description = spec.Description
	existing.EligibilityConfig = spec.EligibilityConfig
	existing.CacheTTL = spec.CacheTTL

	if err = s.db.Save(existing).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark spec already exists for this type")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: update failed")
	}
	return existing, nil
}

// Patch partially updates a TrustMarkSpec
func (s *TrustMarkSpecStorage) Patch(ident string, updates map[string]any) (*model.TrustMarkSpec, error) {
	existing, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	if err = s.db.Model(existing).Updates(updates).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark spec already exists for this type")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: patch failed")
	}
	return existing, nil
}

// Delete deletes a TrustMarkSpec
func (s *TrustMarkSpecStorage) Delete(ident string) error {
	existing, err := s.findByIdent(ident)
	if err != nil {
		return err
	}
	if err = s.db.Delete(existing).Error; err != nil {
		return errors.Wrap(err, "trust_mark_specs: delete failed")
	}
	return nil
}

// ListSubjects returns all TrustMarkSubjects for a TrustMarkSpec
func (s *TrustMarkSpecStorage) ListSubjects(specIdent string, status *model.Status) ([]model.TrustMarkSubject, error) {
	spec, err := s.findByIdent(specIdent)
	if err != nil {
		return nil, err
	}
	query := s.db.Where("trust_mark_spec_id = ?", spec.ID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	var subjects []model.TrustMarkSubject
	if err = query.Find(&subjects).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_specs: list subjects failed")
	}
	return subjects, nil
}

// CreateSubject creates a new TrustMarkSubject for a TrustMarkSpec.
// If a soft-deleted subject with the same entity_id exists, it will be restored.
func (s *TrustMarkSpecStorage) CreateSubject(specIdent string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
	spec, err := s.findByIdent(specIdent)
	if err != nil {
		return nil, err
	}

	record := &model.TrustMarkSubject{
		TrustMarkSpecID:  spec.ID,
		EntityID:         subject.EntityID,
		Status:           subject.Status,
		Description:      subject.Description,
		AdditionalClaims: subject.AdditionalClaims,
	}

	// Check for soft-deleted record with same entity_id (unscoped to include deleted)
	var existing model.TrustMarkSubject
	if err := s.db.Unscoped().Where(
		"trust_mark_spec_id = ? AND entity_id = ?", spec.ID, subject.EntityID,
	).First(&existing).Error; err == nil {
		// Record exists (possibly soft-deleted)
		if existing.DeletedAt.Valid {
			// Restore soft-deleted record by updating it
			existing.DeletedAt = gorm.DeletedAt{} // Clear soft-delete
			existing.Status = subject.Status
			existing.AdditionalClaims = subject.AdditionalClaims
			existing.Description = subject.Description
			if err := s.db.Unscoped().Save(&existing).Error; err != nil {
				return nil, errors.Wrap(err, "trust_mark_specs: restore subject failed")
			}
			return &existing, nil
		}
		// Record exists and is not deleted - conflict
		return nil, model.AlreadyExistsError("subject already exists for this trust mark spec")
	}

	// No existing record, create new
	if err = s.db.Omit("TrustMarkSpec").Create(record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("subject already exists for this trust mark spec")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: create subject failed")
	}
	return record, nil
}

// findSubjectByIdent finds a TrustMarkSubject by ID or entity_id within a spec
func (s *TrustMarkSpecStorage) findSubjectByIdent(specIdent, subjectIdent string) (*model.TrustMarkSubject, error) {
	spec, err := s.findByIdent(specIdent)
	if err != nil {
		return nil, err
	}
	var subject model.TrustMarkSubject
	// Try numeric ID first
	if id, err := strconv.ParseUint(subjectIdent, 10, 64); err == nil {
		if tx := s.db.Where("trust_mark_spec_id = ?", spec.ID).First(&subject, uint(id)); tx.Error == nil {
			return &subject, nil
		}
	}
	// Fallback to entity_id
	if err := s.db.Where("trust_mark_spec_id = ? AND entity_id = ?", spec.ID, subjectIdent).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark subject not found")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: get subject failed")
	}
	return &subject, nil
}

// GetSubject returns a TrustMarkSubject by ID or entity_id
func (s *TrustMarkSpecStorage) GetSubject(specIdent, subjectIdent string) (*model.TrustMarkSubject, error) {
	return s.findSubjectByIdent(specIdent, subjectIdent)
}

// UpdateSubject updates an existing TrustMarkSubject
func (s *TrustMarkSpecStorage) UpdateSubject(specIdent, subjectIdent string, subject *model.AddTrustMarkSubject) (*model.TrustMarkSubject, error) {
	existing, err := s.findSubjectByIdent(specIdent, subjectIdent)
	if err != nil {
		return nil, err
	}
	existing.EntityID = subject.EntityID
	existing.Status = subject.Status
	existing.Description = subject.Description
	existing.AdditionalClaims = subject.AdditionalClaims

	if err = s.db.Save(existing).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("subject already exists for this trust mark spec")
		}
		return nil, errors.Wrap(err, "trust_mark_specs: update subject failed")
	}
	return existing, nil
}

// DeleteSubject deletes a TrustMarkSubject and revokes all associated trust mark instances.
func (s *TrustMarkSpecStorage) DeleteSubject(specIdent, subjectIdent string) error {
	existing, err := s.findSubjectByIdent(specIdent, subjectIdent)
	if err != nil {
		return err
	}

	// Revoke all issued trust mark instances for this subject before deletion
	s.revokeInstancesForSubject(existing.ID, existing.EntityID, specIdent)

	if err = s.db.Delete(existing).Error; err != nil {
		return errors.Wrap(err, "trust_mark_specs: delete subject failed")
	}
	return nil
}

// ChangeSubjectStatus changes the status of a TrustMarkSubject.
// If the new status is blocked or inactive, all associated trust mark instances are revoked.
func (s *TrustMarkSpecStorage) ChangeSubjectStatus(specIdent, subjectIdent string, status model.Status) (*model.TrustMarkSubject, error) {
	existing, err := s.findSubjectByIdent(specIdent, subjectIdent)
	if err != nil {
		return nil, err
	}
	existing.Status = status
	if err = s.db.Save(existing).Error; err != nil {
		return nil, errors.Wrap(err, "trust_mark_specs: change subject status failed")
	}

	// Revoke all issued trust mark instances if status is blocked or inactive
	if status == model.StatusBlocked || status == model.StatusInactive {
		s.revokeInstancesForSubject(existing.ID, existing.EntityID, specIdent)
	}

	return existing, nil
}

// revokeInstancesForSubject revokes all issued trust mark instances for a subject.
// This is called when a subject's status changes to blocked/inactive or when deleted.
func (s *TrustMarkSpecStorage) revokeInstancesForSubject(subjectID uint, entityID, specIdent string) {
	now := int(time.Now().Unix())
	result := s.db.Model(&model.IssuedTrustMarkInstance{}).
		Where("trust_mark_subject_id = ? AND revoked = ?", subjectID, false).
		Updates(map[string]any{
			"revoked":    true,
			"updated_at": now,
		})

	if result.Error != nil {
		log.WithError(result.Error).WithFields(log.Fields{
			"subject_id": subjectID,
			"entity_id":  entityID,
			"spec_ident": specIdent,
		}).Error("failed to revoke trust mark instances for subject")
		return
	}

	if result.RowsAffected > 0 {
		log.WithFields(log.Fields{
			"subject_id":    subjectID,
			"entity_id":     entityID,
			"spec_ident":    specIdent,
			"revoked_count": result.RowsAffected,
		}).Info("automatically revoked trust mark instances due to subject status change")
	}
}
