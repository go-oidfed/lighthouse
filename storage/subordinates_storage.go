package storage

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// SubordinateStorage implements the SubordinateStorageBackend interface
type SubordinateStorage struct {
	db *gorm.DB
}

// Add stores a model.ExtendedSubordinateInfo
func (s *SubordinateStorage) Add(info model.ExtendedSubordinateInfo) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			// Then save the subordinate info
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&info).Error; err != nil {
				return err
			}
			// Insert entity type rows from pre-populated join slice (UnmarshalJSON)
			if len(info.SubordinateEntityTypes) > 0 {
				for i := range info.SubordinateEntityTypes {
					info.SubordinateEntityTypes[i].SubordinateID = info.ID
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&info.SubordinateEntityTypes).Error; err != nil {
					return errors.Wrap(err, "failed to insert subordinate entity types")
				}
			}
			return nil
		},
	)
}

// Delete removes a subordinate
func (s *SubordinateStorage) Delete(entityID string) error {
	return s.db.Where("entity_id = ?", entityID).Delete(&model.ExtendedSubordinateInfo{}).Error
}

// DeleteByDBID removes a subordinate by primary key ID
func (s *SubordinateStorage) DeleteByDBID(id string) error {
	return s.db.Delete(&model.ExtendedSubordinateInfo{}, id).Error
}

// UpdateStatus updates the status of a subordinate by entityID
func (s *SubordinateStorage) UpdateStatus(entityID string, status model.Status) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			var dbInfo model.ExtendedSubordinateInfo
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

// Get retrieves a subordinate by entity ID
func (s *SubordinateStorage) Get(entityID string) (*model.ExtendedSubordinateInfo, error) {
	var dbInfo model.ExtendedSubordinateInfo
	result := s.db.Where(
		"entity_id = ?", entityID,
	).Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").Preload("JWKS").First(&dbInfo)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find entity: %w", result.Error)
	}
	if err := s.applyGeneralFallbacks(&dbInfo); err != nil {
		return nil, err
	}
	return &dbInfo, nil
}

// GetByDBID retrieves a subordinate by DB primary key
func (s *SubordinateStorage) GetByDBID(id string) (*model.ExtendedSubordinateInfo, error) {
	var dbInfo model.ExtendedSubordinateInfo
	result := s.db.Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").Preload("JWKS").First(&dbInfo, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find entity: %w", result.Error)
	}
	if err := s.applyGeneralFallbacks(&dbInfo); err != nil {
		return nil, err
	}
	return &dbInfo, nil
}

// applyGeneralFallbacks fills in subordinate fields with general defaults from KV store
// when the subordinate-specific values are not set.
func (s *SubordinateStorage) applyGeneralFallbacks(info *model.ExtendedSubordinateInfo) error {
	kvStorage := KeyValueStorage{db: s.db}

	// Fallback for MetadataPolicy
	if info.MetadataPolicy == nil {
		if _, err := kvStorage.GetAs(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyMetadataPolicy, &info.MetadataPolicy,
		); err != nil {
			return errors.Wrap(err, "failed to get general metadata policy")
		}
	}

	// Fallback for Metadata
	if info.Metadata == nil {
		if _, err := kvStorage.GetAs(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyMetadata, &info.Metadata,
		); err != nil {
			return errors.Wrap(err, "failed to get general metadata")
		}
	}

	// Fallback for Constraints
	if info.Constraints == nil {
		if _, err := kvStorage.GetAs(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyConstraints, &info.Constraints,
		); err != nil {
			return errors.Wrap(err, "failed to get general constraints")
		}
	}

	// Fallback for AdditionalClaims
	if len(info.SubordinateAdditionalClaims) == 0 {
		var generalClaims []model.SubordinateAdditionalClaim
		if _, err := kvStorage.GetAs(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyAdditionalClaims, &generalClaims,
		); err != nil {
			return errors.Wrap(err, "failed to get general additional claims")
		}
		info.SubordinateAdditionalClaims = generalClaims
	}

	return nil
}

// Update updates the subordinate info by entityID
func (s *SubordinateStorage) Update(entityID string, info model.ExtendedSubordinateInfo) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			var dbInfo model.ExtendedSubordinateInfo
			result := tx.Where("entity_id = ?", entityID).First(&dbInfo)
			if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return result.Error
			}
			info.ID = dbInfo.ID
			return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&info).Error
		},
	)
}

// UpdateStatusByDBID updates status by DB primary key
func (s *SubordinateStorage) UpdateStatusByDBID(id string, status model.Status) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			var info model.ExtendedSubordinateInfo
			if err := tx.First(&info, id).Error; err != nil {
				return errors.Wrap(err, "failed to find subordinate by id")
			}
			info.Status = status
			return tx.Save(&info).Error
		},
	)
}

// UpdateJWKSByDBID updates the JWKS for a subordinate by DB primary key.
// If the subordinate has no JWKS yet, one is created and linked.
// Returns the updated JWKS with correct ID.
func (s *SubordinateStorage) UpdateJWKSByDBID(id string, jwks model.JWKS) (*model.JWKS, error) {
	var resultJWKS *model.JWKS
	err := s.db.Transaction(
		func(tx *gorm.DB) error {
			var info model.ExtendedSubordinateInfo
			if err := tx.Preload("JWKS").First(&info, id).Error; err != nil {
				return errors.Wrap(err, "failed to find subordinate by id")
			}

			if info.JWKSID == 0 {
				// No JWKS exists yet, create a new one
				if err := tx.Create(&jwks).Error; err != nil {
					return errors.Wrap(err, "failed to create JWKS")
				}
				info.JWKSID = jwks.ID
				info.JWKS = jwks
				if err := tx.Save(&info).Error; err != nil {
					return errors.Wrap(err, "failed to link JWKS to subordinate")
				}
			} else {
				// JWKS exists, update its keys
				info.JWKS.Keys = jwks.Keys
				if err := tx.Save(&info.JWKS).Error; err != nil {
					return errors.Wrap(err, "failed to update JWKS")
				}
			}
			resultJWKS = &info.JWKS
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return resultJWKS, nil
}

// GetAll returns all subordinates
func (s *SubordinateStorage) GetAll() ([]model.BasicSubordinateInfo, error) {
	var infos []model.ExtendedSubordinateInfo
	if err := s.db.Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").Find(&infos).Error; err != nil {
		return nil, errors.Wrap(err, "failed to get all subordinates")
	}
	basics := make([]model.BasicSubordinateInfo, len(infos))
	for i := range infos {
		basics[i] = infos[i].BasicSubordinateInfo
		basics[i].SubordinateEntityTypes = infos[i].SubordinateEntityTypes
	}
	return basics, nil
}

// GetByStatus returns all subordinates with a specific status
func (s *SubordinateStorage) GetByStatus(status model.Status) ([]model.BasicSubordinateInfo, error) {
	var infos []model.ExtendedSubordinateInfo
	if err := s.db.Where(
		"status = ?", status,
	).Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").Find(&infos).Error; err != nil {
		return nil, errors.Wrap(err, "failed to get subordinates by status")
	}
	basics := make([]model.BasicSubordinateInfo, len(infos))
	for i := range infos {
		basics[i] = infos[i].BasicSubordinateInfo
		basics[i].SubordinateEntityTypes = infos[i].SubordinateEntityTypes
	}
	return basics, nil
}
func (s *SubordinateStorage) GetByEntityTypes(entityTypes []string) ([]model.BasicSubordinateInfo, error) {
	ids, err := s.buildEntityTypeJoin(nil, entityTypes, true)
	if err != nil {
		return nil, err
	}
	return s.fetchByIDsBasic(ids)
}

func (s *SubordinateStorage) GetByAnyEntityType(entityTypes []string) ([]model.BasicSubordinateInfo, error) {
	ids, err := s.buildEntityTypeJoin(nil, entityTypes, false)
	if err != nil {
		return nil, err
	}
	return s.fetchByIDsBasic(ids)
}

// GetByStatusAndEntityTypes returns subordinates matching both the specified status and all entity types
func (s *SubordinateStorage) GetByStatusAndEntityTypes(
	status model.Status, entityTypes []string,
) ([]model.BasicSubordinateInfo, error) {
	ids, err := s.buildEntityTypeJoin(&status, entityTypes, true)
	if err != nil {
		return nil, err
	}
	if ids == nil { // means only status filtering requested
		return s.GetByStatus(status)
	}
	return s.fetchByIDsBasic(ids)
}

// GetByStatusOrEntityTypes returns subordinates matching status and any of the entity types
func (s *SubordinateStorage) GetByStatusAndAnyEntityType(
	status model.Status, entityTypes []string,
) ([]model.BasicSubordinateInfo, error) {
	ids, err := s.buildEntityTypeJoin(&status, entityTypes, false)
	if err != nil {
		return nil, err
	}
	if ids == nil { // only status filtering requested
		return s.GetByStatus(status)
	}
	return s.fetchByIDsBasic(ids)
}

// buildEntityTypeJoin returns matching subordinate IDs for a given optional status and entity types filter.
// If status is provided and entityTypes is empty, returns nil IDs to signal status-only filtering.
func (s *SubordinateStorage) buildEntityTypeJoin(status *model.Status, entityTypes []string, requireAll bool) (
	[]uint, error,
) {
	if len(entityTypes) == 0 {
		if status == nil {
			return []uint{}, nil
		}
		return nil, nil // status-only; caller can route to GetByStatus
	}
	subTable := s.db.NamingStrategy.TableName("subordinates")
	joinTable := s.db.NamingStrategy.TableName("subordinate_entity_types")
	db := s.db.Table(subTable + " as s").Joins("JOIN " + joinTable + " as setypes ON setypes.subordinate_id = s.id")
	if status != nil {
		db = db.Where("s.status = ?", *status)
	}
	db = db.Where("setypes.entity_type IN ?", entityTypes)
	if requireAll {
		db = db.Select("s.id").Group("s.id").Having("COUNT(DISTINCT setypes.entity_type) = ?", len(entityTypes))
	} else {
		db = db.Select("DISTINCT s.id")
	}
	var ids []uint
	if err := db.Pluck("s.id", &ids).Error; err != nil {
		return nil, errors.Wrap(err, "failed to query subordinate ids by entity types")
	}
	return ids, nil
}

// fetchByIDs loads ExtendedSubordinateInfo rows by primary keys with entity types preloaded.
func (s *SubordinateStorage) fetchByIDs(ids []uint) ([]model.ExtendedSubordinateInfo, error) {
	if len(ids) == 0 {
		return []model.ExtendedSubordinateInfo{}, nil
	}
	var infos []model.ExtendedSubordinateInfo
	if err := s.db.Where(
		"id IN ?", ids,
	).Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").Find(&infos).Error; err != nil {
		return nil, errors.Wrap(err, "failed to load subordinates by ids")
	}
	return infos, nil
}

// fetchByIDsBasic loads rows and returns only BasicSubordinateInfo slices.
func (s *SubordinateStorage) fetchByIDsBasic(ids []uint) ([]model.BasicSubordinateInfo, error) {
	infos, err := s.fetchByIDs(ids)
	if err != nil {
		return nil, err
	}
	basics := make([]model.BasicSubordinateInfo, len(infos))
	for i := range infos {
		basics[i] = infos[i].BasicSubordinateInfo
		basics[i].SubordinateEntityTypes = infos[i].SubordinateEntityTypes
	}
	return basics, nil
}

// Load is a no-op for GORM storage
func (*SubordinateStorage) Load() error { return nil }

// ListAdditionalClaims returns all additional claims for a subordinate.
func (s *SubordinateStorage) ListAdditionalClaims(subordinateDBID string) ([]model.SubordinateAdditionalClaim, error) {
	subID, err := s.parseSubordinateID(subordinateDBID)
	if err != nil {
		return nil, err
	}
	var claims []model.SubordinateAdditionalClaim
	if err := s.db.Where("subordinate_id = ?", subID).Find(&claims).Error; err != nil {
		return nil, errors.Wrap(err, "failed to list additional claims")
	}
	return claims, nil
}

// SetAdditionalClaims replaces all additional claims for a subordinate.
func (s *SubordinateStorage) SetAdditionalClaims(
	subordinateDBID string, claims []model.AddAdditionalClaim,
) ([]model.SubordinateAdditionalClaim, error) {
	subID, err := s.parseSubordinateID(subordinateDBID)
	if err != nil {
		return nil, err
	}
	// Verify subordinate exists
	if _, err := s.verifySubordinateExists(subID); err != nil {
		return nil, err
	}

	var result []model.SubordinateAdditionalClaim
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing claims
		if err := tx.Where("subordinate_id = ?", subID).Delete(&model.SubordinateAdditionalClaim{}).Error; err != nil {
			return errors.Wrap(err, "failed to delete existing claims")
		}
		// Insert new claims
		if len(claims) == 0 {
			return nil
		}
		rows := make([]model.SubordinateAdditionalClaim, len(claims))
		for i, c := range claims {
			rows[i] = model.SubordinateAdditionalClaim{
				SubordinateID: subID,
				Claim:         c.Claim,
				Value:         c.Value,
				Crit:          c.Crit,
			}
		}
		if err := tx.Create(&rows).Error; err != nil {
			return errors.Wrap(err, "failed to insert claims")
		}
		result = rows
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CreateAdditionalClaim creates a single additional claim for a subordinate.
func (s *SubordinateStorage) CreateAdditionalClaim(
	subordinateDBID string, claim model.AddAdditionalClaim,
) (*model.SubordinateAdditionalClaim, error) {
	subID, err := s.parseSubordinateID(subordinateDBID)
	if err != nil {
		return nil, err
	}
	// Verify subordinate exists
	if _, err := s.verifySubordinateExists(subID); err != nil {
		return nil, err
	}

	row := model.SubordinateAdditionalClaim{
		SubordinateID: subID,
		Claim:         claim.Claim,
		Value:         claim.Value,
		Crit:          claim.Crit,
	}
	if err := s.db.Create(&row).Error; err != nil {
		if isUniqueConstraintErr(err) {
			return nil, model.AlreadyExistsErrorFmt("claim %q already exists for this subordinate", claim.Claim)
		}
		return nil, errors.Wrap(err, "failed to create additional claim")
	}
	return &row, nil
}

// GetAdditionalClaim retrieves a single additional claim by ID for a subordinate.
func (s *SubordinateStorage) GetAdditionalClaim(
	subordinateDBID string, claimID string,
) (*model.SubordinateAdditionalClaim, error) {
	subID, err := s.parseSubordinateID(subordinateDBID)
	if err != nil {
		return nil, err
	}
	var row model.SubordinateAdditionalClaim
	result := s.db.Where("subordinate_id = ? AND id = ?", subID, claimID).First(&row)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("additional claim not found")
		}
		return nil, errors.Wrap(result.Error, "failed to get additional claim")
	}
	return &row, nil
}

// UpdateAdditionalClaim updates an existing additional claim for a subordinate.
func (s *SubordinateStorage) UpdateAdditionalClaim(
	subordinateDBID string, claimID string, claim model.AddAdditionalClaim,
) (*model.SubordinateAdditionalClaim, error) {
	subID, err := s.parseSubordinateID(subordinateDBID)
	if err != nil {
		return nil, err
	}
	var row model.SubordinateAdditionalClaim
	err = s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("subordinate_id = ? AND id = ?", subID, claimID).First(&row)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return model.NotFoundError("additional claim not found")
			}
			return errors.Wrap(result.Error, "failed to find additional claim")
		}
		// Check if claim name is changing and would conflict
		if claim.Claim != "" && claim.Claim != row.Claim {
			var existing model.SubordinateAdditionalClaim
			if err := tx.Where("subordinate_id = ? AND claim = ? AND id != ?", subID, claim.Claim, row.ID).First(&existing).Error; err == nil {
				return model.AlreadyExistsErrorFmt("claim %q already exists for this subordinate", claim.Claim)
			}
			row.Claim = claim.Claim
		}
		row.Value = claim.Value
		row.Crit = claim.Crit
		if err := tx.Save(&row).Error; err != nil {
			return errors.Wrap(err, "failed to update additional claim")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// DeleteAdditionalClaim deletes an additional claim for a subordinate.
func (s *SubordinateStorage) DeleteAdditionalClaim(subordinateDBID string, claimID string) error {
	subID, err := s.parseSubordinateID(subordinateDBID)
	if err != nil {
		return err
	}
	result := s.db.Where("subordinate_id = ? AND id = ?", subID, claimID).Delete(&model.SubordinateAdditionalClaim{})
	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to delete additional claim")
	}
	if result.RowsAffected == 0 {
		return model.NotFoundError("additional claim not found")
	}
	return nil
}

// parseSubordinateID parses the subordinate DB ID string to uint.
func (*SubordinateStorage) parseSubordinateID(subordinateDBID string) (uint, error) {
	var subID uint
	if _, err := fmt.Sscanf(subordinateDBID, "%d", &subID); err != nil {
		return 0, model.NotFoundErrorFmt("invalid subordinate ID: %s", subordinateDBID)
	}
	return subID, nil
}

// verifySubordinateExists checks if a subordinate with the given ID exists.
func (s *SubordinateStorage) verifySubordinateExists(subID uint) (*model.ExtendedSubordinateInfo, error) {
	var info model.ExtendedSubordinateInfo
	if err := s.db.First(&info, subID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("subordinate not found")
		}
		return nil, errors.Wrap(err, "failed to verify subordinate exists")
	}
	return &info, nil
}

// isUniqueConstraintErr checks if an error is a unique constraint violation.
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common unique constraint error patterns across databases
	return contains(errStr, "UNIQUE constraint") || // SQLite
		contains(errStr, "duplicate key") || // PostgreSQL
		contains(errStr, "Duplicate entry") // MySQL
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
