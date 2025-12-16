package adminapi

import "github.com/gofiber/fiber/v2"

// registerGeneralSubordinateLifetime adds stub handlers for the general subordinate lifetime endpoints.
func registerGeneralSubordinateLifetime(r fiber.Router) {
	g := r.Group("/subordinates")
	g.Get("/lifetime", func(c *fiber.Ctx) error { return c.JSON(0) })
	g.Put(
		"/lifetime", subordinateStatementsCacheInvalidationMiddleware,
		func(c *fiber.Ctx) error { return c.JSON(0) },
	)
}
