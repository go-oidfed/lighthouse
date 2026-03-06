package storage

import (
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// IssuedTrustMarkInstanceStorage provides GORM-based storage for issued trust mark instances.
type IssuedTrustMarkInstanceStorage struct {
	db *gorm.DB
}

// NewIssuedTrustMarkInstanceStorage creates a new IssuedTrustMarkInstanceStorage.
func NewIssuedTrustMarkInstanceStorage(db *gorm.DB) *IssuedTrustMarkInstanceStorage {
	return &IssuedTrustMarkInstanceStorage{db: db}
}

// Create records a new issued trust mark instance.
func (s *IssuedTrustMarkInstanceStorage) Create(instance *model.IssuedTrustMarkInstance) error {
	if instance.JTI == "" {
		return errors.New("JTI is required")
	}
	instance.CreatedAt = int(time.Now().Unix())
	instance.UpdatedAt = instance.CreatedAt
	if err := s.db.Create(instance).Error; err != nil {
		return errors.Wrap(err, "issued_trust_mark_instances: create failed")
	}
	return nil
}

// GetByJTI retrieves an instance by its JTI (JWT ID).
func (s *IssuedTrustMarkInstanceStorage) GetByJTI(jti string) (*model.IssuedTrustMarkInstance, error) {
	var instance model.IssuedTrustMarkInstance
	if err := s.db.Where("jti = ?", jti).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundErrorFmt("trust mark instance not found: %s", jti)
		}
		return nil, errors.Wrap(err, "issued_trust_mark_instances: get by JTI failed")
	}
	return &instance, nil
}

// Revoke marks a trust mark instance as revoked.
func (s *IssuedTrustMarkInstanceStorage) Revoke(jti string) error {
	result := s.db.Model(&model.IssuedTrustMarkInstance{}).
		Where("jti = ?", jti).
		Updates(map[string]any{
			"revoked":    true,
			"updated_at": int(time.Now().Unix()),
		})
	if result.Error != nil {
		return errors.Wrap(result.Error, "issued_trust_mark_instances: revoke failed")
	}
	if result.RowsAffected == 0 {
		return model.NotFoundErrorFmt("trust mark instance not found: %s", jti)
	}
	return nil
}

// RevokeBySubjectID revokes all instances for a given TrustMarkSubjectID.
// Returns the number of revoked instances.
func (s *IssuedTrustMarkInstanceStorage) RevokeBySubjectID(subjectID uint) (int64, error) {
	result := s.db.Model(&model.IssuedTrustMarkInstance{}).
		Where("trust_mark_subject_id = ? AND revoked = ?", subjectID, false).
		Updates(map[string]any{
			"revoked":    true,
			"updated_at": int(time.Now().Unix()),
		})
	if result.Error != nil {
		return 0, errors.Wrap(result.Error, "issued_trust_mark_instances: revoke by subject ID failed")
	}
	return result.RowsAffected, nil
}

// GetStatus returns the status of a trust mark instance.
// Status is determined by: revoked flag, expiration time, and existence.
func (s *IssuedTrustMarkInstanceStorage) GetStatus(jti string) (model.TrustMarkInstanceStatus, error) {
	instance, err := s.GetByJTI(jti)
	if err != nil {
		var notFound model.NotFoundError
		if errors.As(err, &notFound) {
			// Not found means we don't know about this trust mark
			return "", err
		}
		return "", err
	}

	// Check revocation first
	if instance.Revoked {
		return model.TrustMarkStatusRevoked, nil
	}

	// Check expiration
	if instance.ExpiresAt > 0 && int(time.Now().Unix()) > instance.ExpiresAt {
		return model.TrustMarkStatusExpired, nil
	}

	return model.TrustMarkStatusActive, nil
}

// ListBySubject returns all instances for a given trust mark type and subject.
func (s *IssuedTrustMarkInstanceStorage) ListBySubject(trustMarkType, entityID string) ([]model.IssuedTrustMarkInstance, error) {
	var instances []model.IssuedTrustMarkInstance
	if err := s.db.Where("trust_mark_type = ? AND subject = ?", trustMarkType, entityID).
		Order("created_at DESC").
		Find(&instances).Error; err != nil {
		return nil, errors.Wrap(err, "issued_trust_mark_instances: list by subject failed")
	}
	return instances, nil
}

// DeleteExpired removes expired instances older than the given retention period.
// Returns the number of deleted records.
func (s *IssuedTrustMarkInstanceStorage) DeleteExpired(retentionDays int) (int64, error) {
	cutoff := int(time.Now().AddDate(0, 0, -retentionDays).Unix())
	result := s.db.Where("expires_at > 0 AND expires_at < ?", cutoff).
		Delete(&model.IssuedTrustMarkInstance{})
	if result.Error != nil {
		return 0, errors.Wrap(result.Error, "issued_trust_mark_instances: delete expired failed")
	}
	return result.RowsAffected, nil
}

// FindSubjectID looks up the TrustMarkSubjectID for a given trust mark type and entity.
// This is used to link issued instances to their subject records.
func (s *IssuedTrustMarkInstanceStorage) FindSubjectID(trustMarkType, entityID string) (uint, error) {
	var subject model.TrustMarkSubject
	err := s.db.
		Joins("JOIN trust_mark_specs ON trust_mark_specs.id = trust_mark_subjects.trust_mark_spec_id").
		Where("trust_mark_specs.trust_mark_type = ? AND trust_mark_subjects.entity_id = ?", trustMarkType, entityID).
		First(&subject).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Subject not found - this is OK, the trust mark may be issued without a subject record
			return 0, nil
		}
		return 0, errors.Wrap(err, "issued_trust_mark_instances: find subject ID failed")
	}
	return subject.ID, nil
}
