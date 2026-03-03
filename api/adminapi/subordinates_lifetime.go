package adminapi

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerGeneralSubordinateLifetime adds handlers for the general subordinate lifetime endpoints.
func registerGeneralSubordinateLifetime(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET /subordinates/lifetime - Get general subordinate lifetime in seconds
	g.Get("/lifetime", handleGetSubordinateLifetime(kv))

	// PUT /subordinates/lifetime - Update general subordinate lifetime in seconds
	withCacheWipe.Put("/lifetime", handlePutSubordinateLifetime(kv))
}

func handleGetSubordinateLifetime(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var seconds int
		found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyLifetime, &seconds)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found || seconds <= 0 {
			seconds = int(storage.DefaultSubordinateStatementLifetime.Seconds())
		}
		return c.JSON(seconds)
	}
}

func handlePutSubordinateLifetime(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if len(c.Body()) == 0 {
			return writeBadRequest(c, "empty body")
		}
		var seconds int
		if err := json.Unmarshal(c.Body(), &seconds); err != nil {
			return writeBadBody(c)
		}
		if seconds < 0 {
			return writeBadRequest(c, "lifetime must be non-negative")
		}
		if err := kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyLifetime, seconds); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(seconds)
	}
}
