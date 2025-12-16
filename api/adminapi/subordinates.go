package adminapi

import "github.com/gofiber/fiber/v2"

func registerSubordinates(r fiber.Router) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Get("/:subordinateID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:subordinateID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:subordinateID", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:subordinateID/statement", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"statement": fiber.Map{}}) })
	g.Get("/:subordinateID/history", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"events": []fiber.Map{}}) })

	// Subordinate additional claims
	g.Get("/:subordinateID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:subordinateID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get(
		"/:subordinateID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	withCacheWipe.Put(
		"/:subordinateID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	withCacheWipe.Delete(
		"/:subordinateID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)
}

// General metadata policies (no subordinateID)
func registerGeneralMetadataPolicies(r fiber.Router) {
	g := r.Group("/subordinates/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim/:operator", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType/:claim/:operator", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType/:claim/:operator", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete(
		"/:entityType/:claim/:operator", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)
}

func registerSubordinateMetadata(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/metadata")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateMetadataPolicies(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/metadata-policies")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateConstraints(r fiber.Router) {
	withCacheWipe := r.Use(subordinateStatementsCacheInvalidationMiddleware)
	// General constraints
	r.Get("/subordinates/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put("/subordinates/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })

	r.Get("/subordinates/:subordinateID/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	withCacheWipe.Put(
		"/subordinates/:subordinateID/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	withCacheWipe.Post(
		"/subordinates/:subordinateID/constraints",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) },
	)
	withCacheWipe.Delete(
		"/subordinates/:subordinateID/constraints",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)

	// Allowed entity types
	r.Get("/subordinates/constraints/allowed-entity-types", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	withCacheWipe.Put(
		"/subordinates/constraints/allowed-entity-types", func(c *fiber.Ctx) error { return c.JSON([]string{}) },
	)
	withCacheWipe.Post(
		"/subordinates/constraints/allowed-entity-types",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON([]string{}) },
	)
	withCacheWipe.Delete(
		"/subordinates/constraints/allowed-entity-types/:entityType",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)

	// Max path length
	r.Get("/subordinates/constraints/max-path-length", func(c *fiber.Ctx) error { return c.JSON(nil) })
	withCacheWipe.Put("/subordinates/constraints/max-path-length", func(c *fiber.Ctx) error { return c.JSON(0) })
}

func registerSubordinateKeys(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/jwks")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{"jwks": []fiber.Map{}}) },
	)
	withCacheWipe.Delete("/:kid", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

// Subordinate crit is managed via the additional-claims endpoints; no separate crit endpoints

func registerSubordinateMetadataPolicyCrit(r fiber.Router) {
	g := r.Group("/subordinates/metadata-policy-crit")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	withCacheWipe.Put("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	withCacheWipe.Post("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	withCacheWipe.Delete("/:operator", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}
