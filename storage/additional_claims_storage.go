package storage

import (
	"strconv"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AdditionalClaimsStorage is the GORM implementation for model.AdditionalClaimsStore.
type AdditionalClaimsStorage struct {
	db *gorm.DB
}

func (s *Storage) AdditionalClaimsStorage() *AdditionalClaimsStorage {
	return &AdditionalClaimsStorage{db: s.db}
}

func (s *AdditionalClaimsStorage) List() ([]model.EntityConfigurationAdditionalClaim, error) {
	var rows []model.EntityConfigurationAdditionalClaim
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, errors.Wrap(err, "additional_claims: list failed")
	}
	return rows, nil
}

func (s *AdditionalClaimsStorage) Set(items []model.AddAdditionalClaim) (
	[]model.EntityConfigurationAdditionalClaim, error,
) {
	// Map request items to DB rows
	rows := make([]model.EntityConfigurationAdditionalClaim, 0, len(items))
	for _, it := range items {
		rows = append(
			rows, model.EntityConfigurationAdditionalClaim{
				Claim: it.Claim,
				Value: it.Value,
				Crit:  it.Crit,
			},
		)
	}
	return rows, s.db.Transaction(
		func(tx *gorm.DB) error {
			// delete all
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.EntityConfigurationAdditionalClaim{}).Error; err != nil {
				return errors.Wrap(err, "additional_claims: purge failed")
			}
			// insert new
			if len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					return errors.Wrap(err, "additional_claims: create batch failed")
				}
			}
			return nil
		},
	)
}

func (s *AdditionalClaimsStorage) Create(item model.AddAdditionalClaim) (
	*model.EntityConfigurationAdditionalClaim, error,
) {
	if item.Claim == "" {
		return nil, errors.New("additional_claims: claim is required")
	}
	row := &model.EntityConfigurationAdditionalClaim{
		Claim: item.Claim,
		Value: item.Value,
		Crit:  item.Crit,
	}
	if err := s.db.Create(row).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("additional claim already exists")
		}
		return nil, errors.Wrap(err, "additional_claims: create failed")
	}
	return row, nil
}

func (s *AdditionalClaimsStorage) findByIdent(ident string) (*model.EntityConfigurationAdditionalClaim, error) {
	var row model.EntityConfigurationAdditionalClaim
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.First(&row, uint(id)); tx.Error == nil {
			return &row, nil
		}
	}
	if err := s.db.Where("claim = ?", ident).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("additional claim not found")
		}
		return nil, errors.Wrap(err, "additional_claims: get failed")
	}
	return &row, nil
}

func (s *AdditionalClaimsStorage) Get(ident string) (*model.EntityConfigurationAdditionalClaim, error) {
	return s.findByIdent(ident)
}

func (s *AdditionalClaimsStorage) Update(
	ident string, item model.AddAdditionalClaim,
) (*model.EntityConfigurationAdditionalClaim, error) {
	row, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}
	row.Value = item.Value
	row.Crit = item.Crit
	if err := s.db.Save(row).Error; err != nil {
		return nil, errors.Wrap(err, "additional_claims: update failed")
	}
	return row, nil
}

func (s *AdditionalClaimsStorage) Delete(ident string) error {
	row, err := s.findByIdent(ident)
	if err != nil {
		return err
	}
	if err := s.db.Delete(row).Error; err != nil {
		return errors.Wrap(err, "additional_claims: delete failed")
	}
	return nil
}

// Removed crit-specific methods; crit is part of each claim row
