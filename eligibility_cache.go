package lighthouse

import (
	"sync"
	"time"
)

// EligibilityCache caches eligibility check results for trust mark issuance.
// This reduces the load on external checkers (http_list, http_list_jwt) and
// database queries for repeated requests.
type EligibilityCache struct {
	mu      sync.RWMutex
	entries map[string]*eligibilityCacheEntry
}

type eligibilityCacheEntry struct {
	eligible  bool
	httpCode  int
	reason    string
	expiresAt time.Time
}

// NewEligibilityCache creates a new eligibility cache
func NewEligibilityCache() *EligibilityCache {
	return &EligibilityCache{
		entries: make(map[string]*eligibilityCacheEntry),
	}
}

// cacheKey generates a unique key for a trust mark type and subject combination
func (*EligibilityCache) cacheKey(trustMarkType, subject string) string {
	return trustMarkType + "|" + subject
}

// Get retrieves a cached eligibility result
// Returns eligible status, HTTP code, reason, and whether a valid entry was found
func (c *EligibilityCache) Get(trustMarkType, subject string) (eligible bool, httpCode int, reason string, found bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[c.cacheKey(trustMarkType, subject)]
	if !ok || time.Now().After(entry.expiresAt) {
		return false, 0, "", false
	}
	return entry.eligible, entry.httpCode, entry.reason, true
}

// Set stores an eligibility result in the cache
func (c *EligibilityCache) Set(trustMarkType, subject string, eligible bool, httpCode int, reason string, ttl time.Duration) {
	if ttl <= 0 {
		return // Don't cache if TTL is 0 or negative
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[c.cacheKey(trustMarkType, subject)] = &eligibilityCacheEntry{
		eligible:  eligible,
		httpCode:  httpCode,
		reason:    reason,
		expiresAt: time.Now().Add(ttl),
	}
}

// Invalidate removes a specific entry from the cache
// This should be called when a subject's status changes (e.g., via admin API)
func (c *EligibilityCache) Invalidate(trustMarkType, subject string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, c.cacheKey(trustMarkType, subject))
}

// InvalidateAll removes all entries for a specific trust mark type
// This should be called when the eligibility config for a trust mark type changes
func (c *EligibilityCache) InvalidateAll(trustMarkType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := trustMarkType + "|"
	for key := range c.entries {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(c.entries, key)
		}
	}
}

// InvalidateType is an alias for InvalidateAll for clearer semantics
func (c *EligibilityCache) InvalidateType(trustMarkType string) {
	c.InvalidateAll(trustMarkType)
}

// CleanExpired removes all expired entries from the cache
// This can be called periodically to prevent memory growth
func (c *EligibilityCache) CleanExpired() int {
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

// Size returns the current number of entries in the cache
func (c *EligibilityCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear removes all entries from the cache
func (c *EligibilityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*eligibilityCacheEntry)
}

// StartCleanupRoutine starts a background goroutine that periodically cleans
// expired entries from the cache. Returns a stop function that should be called
// to stop the cleanup routine.
func (c *EligibilityCache) StartCleanupRoutine(interval time.Duration) (stop func()) {
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
