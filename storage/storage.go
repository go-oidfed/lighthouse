package storage

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// Storage is a GORM-based storage implementation
type Storage struct {
	db         *gorm.DB
	userParams Argon2idParams
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

// DBPublicKeyStorage returns a DBPublicKeyStorage
func (s *Storage) DBPublicKeyStorage(typeID string) *DBPublicKeyStorage {
	return NewDBPublicKeyStorage(s.db, typeID)
}

// Users storage is implemented in users_storage.go

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
		if err := s.db.First(&owner, *t.OwnerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Skip missing owner rows gracefully
				continue
			}
			return nil, errors.Wrap(err, "trust_mark_types: get owner failed")
		}
		out[t.TrustMarkType] = oidfed.TrustMarkOwnerSpec{
			ID:   owner.EntityID,
			JWKS: owner.JWKS.JWKS(),
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
	if err := s.db.Find(&items).Error; err != nil {
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
		if tx := s.db.First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}
	if err := s.db.Where("entity_id = ?", ident).First(&item).Error; err != nil {
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
