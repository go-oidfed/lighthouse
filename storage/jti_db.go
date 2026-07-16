package storage

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// JTIDBStorage implements model.JTIStorageBackend using the database
type JTIDBStorage struct {
	db *gorm.DB
}

// NewJTIDBStorage creates a new database-based JTI storage
func NewJTIDBStorage(db *gorm.DB) *JTIDBStorage {
	return &JTIDBStorage{db: db}
}

// Exists checks if a JTI has been used
func (s *JTIDBStorage) Exists(jti string) (bool, error) {
	var count int64
	if err := s.db.Model(&model.JTIUsed{}).Where("jti = ?", jti).Count(&count).Error; err != nil {
		return false, errors.Wrap(err, "failed to check JTI in database")
	}
	return count > 0, nil
}

// Store marks a JTI as used with expiration
func (s *JTIDBStorage) Store(jti string, expiresAt time.Time) error {
	jtiUsed := model.JTIUsed{
		JTI:       jti,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	// Use Create to insert the JTI
	// Ignore duplicate key errors (replay attempts will be caught by Exists check before Store)
	err := s.db.Create(&jtiUsed).Error
	if err != nil {
		// Check if it's a duplicate key error (already exists)
		// Handle different database error formats
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate key") ||
			strings.Contains(errStr, "UNIQUE constraint failed") ||
			strings.Contains(errStr, "Duplicate entry") {
			// This shouldn't happen if Exists is called before Store, but handle gracefully
			return nil
		}
		return errors.Wrap(err, "failed to store JTI in database")
	}
	return nil
}

// Cleanup removes expired JTIs from the database
func (s *JTIDBStorage) Cleanup() error {
	now := time.Now()
	result := s.db.Where("expires_at < ?", now).Delete(&model.JTIUsed{})
	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to cleanup expired JTIs")
	}
	if result.RowsAffected > 0 {
		log.Debug().Int64("cleaned", result.RowsAffected).Msg("Cleaned up expired JTIs")
	}
	return nil
}

// Ensure JTIDBStorage implements model.JTIStorageBackend
var _ model.JTIStorageBackend = (*JTIDBStorage)(nil)
