package storage

import (
	"fmt"

	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// DBPublicKeyStorage implements public.PublicKeyStorage backed by the database.
type DBPublicKeyStorage struct {
	db *gorm.DB
}

// NewDBPublicKeyStorage creates a DB-backed PublicKeyStorage.
func NewDBPublicKeyStorage(db *gorm.DB, typeID string) *DBPublicKeyStorage {
	tableName := "public_keys"
	if typeID != "" {
		tableName = fmt.Sprintf("%s_%s", tableName, typeID)
	}
	table := db.Table(tableName)
	return &DBPublicKeyStorage{db: table}
}

// Load is a no-op for DB storage.
func (D *DBPublicKeyStorage) Load() error { return D.db.AutoMigrate(&public.PublicKeyEntry{}) }

// GetAll returns all keys, including revoked and expired ones.
func (D *DBPublicKeyStorage) GetAll() (out public.PublicKeyEntryList, err error) {
	err = errors.WithStack(D.db.Find(&out).Error)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// GetRevoked returns all revoked keys.
func (D *DBPublicKeyStorage) GetRevoked() (out public.PublicKeyEntryList, err error) {
	err = errors.WithStack(
		D.db.Where(
			"revoked_at IS NOT NULL AND revoked_at <= CURRENT_TIMESTAMP",
		).Find(&out).Error,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// GetExpired returns keys whose exp is in the past.
func (D *DBPublicKeyStorage) GetExpired() (out public.PublicKeyEntryList, err error) {
	err = errors.WithStack(
		D.db.Where(
			"expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP",
		).Find(&out).Error,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// GetHistorical returns revoked and expired keys.
func (D *DBPublicKeyStorage) GetHistorical() (out public.PublicKeyEntryList, err error) {
	err = errors.WithStack(
		D.db.Where(
			"(expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP) OR (revoked_at IS NOT NULL AND revoked_at <= CURRENT_TIMESTAMP)",
		).Find(&out).Error,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// GetActive returns keys that are currently usable.
func (D *DBPublicKeyStorage) GetActive() (out public.PublicKeyEntryList, err error) {
	err = errors.WithStack(
		D.db.Where(
			"(revoked_at IS NULL OR revoked_at > CURRENT_TIMESTAMP) AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP) AND (not_before IS NULL OR not_before <= CURRENT_TIMESTAMP)",
		).Find(&out).Error,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// GetValid returns keys that are valid now or in the future.
func (D *DBPublicKeyStorage) GetValid() (out public.PublicKeyEntryList, err error) {
	err = errors.WithStack(
		D.db.Where(
			"(revoked_at IS NULL OR revoked_at > CURRENT_TIMESTAMP) AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)",
		).Find(&out).Error,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return
}

// Add inserts a new key if the KID is unused.
func (D *DBPublicKeyStorage) Add(entry public.PublicKeyEntry) error {
	// Ensure KID is set
	if entry.KID == "" && entry.Key.Key != nil {
		var kid string
		_ = entry.Key.Get("kid", &kid)
		entry.KID = kid
	}
	if entry.KID == "" {
		return errors.New("missing kid for public key entry")
	}
	// Check existence
	var existing public.PublicKeyEntry
	res := D.db.Where("kid = ?", entry.KID).First(&existing)
	if res.Error == nil {
		// Already exists, do nothing
		return nil
	}
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		res.Error = nil
		return errors.Wrap(D.db.Create(&entry).Error, "DBPublicKeyStorage: add failed")
	}
	return errors.Wrap(res.Error, "DBPublicKeyStorage: Add: error during existence check")

}

// AddAll adds multiple keys.
func (D *DBPublicKeyStorage) AddAll(list []public.PublicKeyEntry) error {
	for _, e := range list {
		if err := D.Add(e); err != nil {
			return err
		}
	}
	return nil
}

// Update updates editable metadata for a key.
func (D *DBPublicKeyStorage) Update(kid string, data public.UpdateablePublicKeyMetadata) error {
	var row public.PublicKeyEntry
	if err := D.db.Where("kid = ?", kid).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return public.NotFoundError{KID: kid}
		}
		return errors.WithStack(err)
	}
	row.UpdateablePublicKeyMetadata = data
	return errors.WithStack(D.db.Save(&row).Error)
}

// Delete removes a key by kid (from both current and historical stores).
func (D *DBPublicKeyStorage) Delete(kid string) error {
	return errors.WithStack(D.db.Where("kid = ?", kid).Delete(&public.PublicKeyEntry{}).Error)
}

// Revoke marks a key as revoked and moves it to historical storage.
func (D *DBPublicKeyStorage) Revoke(kid, reason string) error {
	var row public.PublicKeyEntry
	if res := D.db.Where("kid = ?", kid).First(&row); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			res.Error = nil
			return public.NotFoundError{KID: kid}
		}
		return errors.WithStack(res.Error)
	}
	now := unixtime.Now()
	row.RevokedAt = &now
	row.Reason = reason
	return errors.WithStack(D.db.Save(&row).Error)
}

// Get returns a single entry by kid from current or historical store.
func (D *DBPublicKeyStorage) Get(kid string) (*public.PublicKeyEntry, error) {
	var row public.PublicKeyEntry
	if res := D.db.Where("kid = ?", kid).First(&row); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			res.Error = nil
			return nil, nil
		}
		return nil, errors.WithStack(res.Error)
	}
	return &row, nil
}

// NewDBPublicKeyStorageFromStorage creates a new DBPublicKeyStorage
// and populates it from the passed PublicKeyStorage implementation.
func NewDBPublicKeyStorageFromStorage(
	db *gorm.DB, typeID string,
	src public.PublicKeyStorage,
) (
	*DBPublicKeyStorage, error,
) {
	storage := NewDBPublicKeyStorage(db, typeID)
	if err := storage.Load(); err != nil {
		return nil, err
	}

	// Load source if necessary
	if err := src.Load(); err != nil {
		return nil, err
	}
	list, err := src.GetAll()
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		if e.KID == "" && e.Key.Key != nil {
			var kid string
			_ = e.Key.Get("kid", &kid)
			e.KID = kid
		}
		if e.KID == "" || e.Key.Key == nil {
			continue
		}
		if k, cerr := e.Key.Clone(); cerr == nil {
			e.Key.Key = k
		} else {
			continue
		}
		if err = storage.Add(e); err != nil {
			return nil, err
		}
	}
	return storage, nil
}
