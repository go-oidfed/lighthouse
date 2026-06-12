package storage

import (
	"time"

	"github.com/go-oidfed/lib/cache"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// JTICacheStorage implements model.JTIStorageBackend using the cache package
type JTICacheStorage struct{}

// NewJTICacheStorage creates a new cache-based JTI storage
func NewJTICacheStorage() *JTICacheStorage {
	return &JTICacheStorage{}
}

// jtiCacheKey generates a cache key for a JTI
func jtiCacheKey(jti string) string {
	return cache.Key("lh", "jti", jti)
}

// Exists checks if a JTI has been used
func (s *JTICacheStorage) Exists(jti string) (bool, error) {
	key := jtiCacheKey(jti)
	var dummy bool
	found, err := cache.Get(key, &dummy)
	if err != nil {
		return false, errors.Wrap(err, "failed to check JTI in cache")
	}
	return found, nil
}

// Store marks a JTI as used with expiration
func (s *JTICacheStorage) Store(jti string, expiresAt time.Time) error {
	key := jtiCacheKey(jti)
	ttl := time.Until(expiresAt)

	// If TTL is already expired or very short, don't store
	if ttl <= 0 {
		return nil
	}

	// Add a small buffer to ensure the JTI is stored slightly longer than the assertion validity
	// This prevents race conditions where the assertion expires but JTI is cleaned up first
	ttl = ttl + 1*time.Minute

	if err := cache.Set(key, true, ttl); err != nil {
		return errors.Wrap(err, "failed to store JTI in cache")
	}
	return nil
}

// Cleanup is a no-op for cache backend since expiration is automatic
func (s *JTICacheStorage) Cleanup() error {
	return nil
}

// Ensure JTICacheStorage implements model.JTIStorageBackend
var _ model.JTIStorageBackend = (*JTICacheStorage)(nil)
