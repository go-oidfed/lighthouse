package lighthouse

import (
	"sync"
	"time"
)

// IssuedTrustMarkCache caches issued trust mark JWTs to avoid repeated signing
// and database writes for the same (trust_mark_type, subject) combination.
type IssuedTrustMarkCache struct {
	mu      sync.RWMutex
	entries map[string]*issuedTrustMarkCacheEntry
}

type issuedTrustMarkCacheEntry struct {
	trustMarkJWT string
	expiresAt    time.Time
}

// NewIssuedTrustMarkCache creates a new issued trust mark cache
func NewIssuedTrustMarkCache() *IssuedTrustMarkCache {
	return &IssuedTrustMarkCache{
		entries: make(map[string]*issuedTrustMarkCacheEntry),
	}
}

// cacheKey generates a unique key for a trust mark type and subject combination
func (*IssuedTrustMarkCache) cacheKey(trustMarkType, subject string) string {
	return trustMarkType + "|" + subject
}

// Get retrieves a cached trust mark JWT.
// Returns the JWT and true if found and not expired, empty string and false otherwise.
func (c *IssuedTrustMarkCache) Get(trustMarkType, subject string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[c.cacheKey(trustMarkType, subject)]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.trustMarkJWT, true
}

// Set stores a trust mark JWT in the cache with the given TTL.
// If ttl <= 0, the entry is not cached.
func (c *IssuedTrustMarkCache) Set(trustMarkType, subject, trustMarkJWT string, ttl time.Duration) {
	if ttl <= 0 {
		return // Don't cache if TTL is 0 or negative
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[c.cacheKey(trustMarkType, subject)] = &issuedTrustMarkCacheEntry{
		trustMarkJWT: trustMarkJWT,
		expiresAt:    time.Now().Add(ttl),
	}
}

// Invalidate removes a specific entry from the cache.
// This should be called when a subject's status changes (e.g., blocked/revoked).
func (c *IssuedTrustMarkCache) Invalidate(trustMarkType, subject string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, c.cacheKey(trustMarkType, subject))
}

// InvalidateAll removes all entries for a specific trust mark type.
func (c *IssuedTrustMarkCache) InvalidateAll(trustMarkType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := trustMarkType + "|"
	for key := range c.entries {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(c.entries, key)
		}
	}
}

// CleanExpired removes all expired entries from the cache.
// This can be called periodically to prevent memory growth.
func (c *IssuedTrustMarkCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
			removed++
		}
	}
	return removed
}

// Size returns the current number of entries in the cache.
func (c *IssuedTrustMarkCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear removes all entries from the cache.
func (c *IssuedTrustMarkCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*issuedTrustMarkCacheEntry)
}

// StartCleanupRoutine starts a background goroutine that periodically cleans
// expired entries from the cache. Returns a stop function that should be called
// to stop the cleanup routine.
func (c *IssuedTrustMarkCache) StartCleanupRoutine(interval time.Duration) (stop func()) {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				c.CleanExpired()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}
