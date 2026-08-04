package adminapi

import (
	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// entityConfigurationCacheInvalidationMiddleware clears the cached entity configuration JWT
// for requests that successfully modify entity configuration state.
// It should be attached only to non-GET routes.
func entityConfigurationCacheInvalidationMiddleware(c *fiber.Ctx) error {
	if err := c.Next(); err != nil {
		return err
	}
	status := c.Response().StatusCode()
	if status >= 200 && status < 400 {
		_ = cache.Delete(internal.CacheKeyEntityConfiguration)
	}
	return nil
}

// subordinateStatementsCacheInvalidationMiddleware returns a Fiber middleware
// that clears the cached subordinate statement JWT for requests that
// successfully modify subordinate state.
//
// The fetch endpoint caches statements keyed by entity ID, but admin routes
// identify subordinates by DB primary key (the :subordinateID route param).
// When a :subordinateID is present and a storage backend is provided, the
// middleware resolves the DB ID to the entity ID and deletes the single
// cache entry. If the lookup fails (e.g. the subordinate was just deleted) or
// no :subordinateID is present (collection-level routes), all subordinate
// statement cache entries are cleared.
func subordinateStatementsCacheInvalidationMiddleware(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		status := c.Response().StatusCode()
		if status >= 200 && status < 400 {
			if id := c.Params("subordinateID"); id != "" && subordinates != nil {
				info, err := subordinates.GetByDBID(id)
				if err != nil || info == nil {
					_ = cache.Clear(internal.CacheKeySubordinateStatement)
				} else {
					_ = cache.Delete(internal.SubordinateStatementCacheKey(info.EntityID))
				}
			} else {
				_ = cache.Clear(internal.CacheKeySubordinateStatement)
			}
		}
		return nil
	}
}
