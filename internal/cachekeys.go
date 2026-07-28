package internal

import (
	"encoding/base64"

	"github.com/go-oidfed/lib/cache"
)

const (
	CacheKeyEntityConfiguration  = "lh:entity_configuration"
	CacheKeySubordinateStatement = "lh:subordinate_statement"
)

// SubordinateStatementCacheKey constructs the cache key for a signed
// subordinate statement JWT. The entity ID is base64 (URL-safe) encoded so the
// key never contains characters (like "/") that would break prefix-based
// cache clearing on backends using glob-style pattern matching.
func SubordinateStatementCacheKey(entityID string) string {
	return cache.Key(CacheKeySubordinateStatement, base64.URLEncoding.EncodeToString([]byte(entityID)))
}
