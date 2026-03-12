package adminapi

import (
	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/internal"
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

// subordinateStatementsCacheInvalidationMiddleware clears the cached
// subordinate statement JWT
// It checks if the request path contains a subordinate id,
// if so only that statement is cleared,
// otherwise all subordinate statements are cleared
func subordinateStatementsCacheInvalidationMiddleware(c *fiber.Ctx) error {
	if err := c.Next(); err != nil {
		return err
	}
	status := c.Response().StatusCode()
	if status >= 200 && status < 400 {
		if subordinateID := c.Params("subordinateID"); subordinateID != "" {
			_ = cache.Delete(cache.Key(internal.CacheKeySubordinateStatement, subordinateID))
		} else {
			_ = cache.Clear(internal.CacheKeySubordinateStatement)
		}
	}
	return nil
}
