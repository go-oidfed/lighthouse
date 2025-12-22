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
	).Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").First(&dbInfo)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find entity: %w", result.Error)
	}
	if dbInfo.MetadataPolicy == nil {
		kvStorage := KeyValueStorage{db: s.db}
		if _, err := kvStorage.GetAs(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyMetadataPolicy, &dbInfo.MetadataPolicy,
		); err != nil {
			return nil, errors.Wrap(
				err, "failed to get general metadata policy",
			)
		}
	}

	return &dbInfo, nil
}

// GetByDBID retrieves a subordinate by DB primary key
func (s *SubordinateStorage) GetByDBID(id string) (*model.ExtendedSubordinateInfo, error) {
	var dbInfo model.ExtendedSubordinateInfo
	result := s.db.Preload("SubordinateEntityTypes").Preload("SubordinateAdditionalClaims").First(&dbInfo, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find entity: %w", result.Error)
	}
	if dbInfo.MetadataPolicy == nil {
		kvStorage := KeyValueStorage{db: s.db}
		if _, err := kvStorage.GetAs(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyMetadataPolicy, &dbInfo.MetadataPolicy,
		); err != nil {
			return nil, errors.Wrap(
				err, "failed to get general metadata policy",
			)
		}
	}

	return &dbInfo, nil
}

// Update updates the subordinate info by entityID
func (s *SubordinateStorage) Update(entityID string, info model.ExtendedSubordinateInfo) error {
	return s.db.Transaction(
		func(tx *gorm.DB) error {
			var dbInfo model.ExtendedSubordinateInfo
			result := tx.Where("entity_id = ?", entityID).First(&dbInfo)
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
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
	db := s.db.Table(subTable + " as s").Joins("JOIN " + joinTable + " as set ON set.subordinate_id = s.id")
	if status != nil {
		db = db.Where("s.status = ?", *status)
	}
	db = db.Where("set.entity_type IN ?", entityTypes)
	if requireAll {
		db = db.Select("s.id").Group("s.id").Having("COUNT(DISTINCT set.entity_type) = ?", len(entityTypes))
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
func (s *SubordinateStorage) Load() error { return nil }
