package adminapi

import "github.com/gofiber/fiber/v2"

func registerSubordinates(r fiber.Router) {
	g := r.Group("/subordinates")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Get("/:subordinateID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:subordinateID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/:subordinateID", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:subordinateID/statement", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"statement": fiber.Map{}}) })
	g.Get("/:subordinateID/history", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"events": []fiber.Map{}}) })

	// Subordinate additional claims
	g.Get("/:subordinateID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:subordinateID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get("/:subordinateID/additional-claims/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:subordinateID/additional-claims/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete(
		"/:subordinateID/additional-claims/:claim",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)
}

func registerSubordinateMetadata(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/metadata")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateMetadataPolicies(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/metadata-policies")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateConstraints(r fiber.Router) {
	// General constraints
	r.Get("/subordinates/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Put("/subordinates/constraints", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })

	g := r.Group("/subordinates/:subordinateID/constraints")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Delete("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	// Allowed entity types
	r.Get("/subordinates/constraints/allowed-entity-types", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	r.Put("/subordinates/constraints/allowed-entity-types", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	r.Post(
		"/subordinates/constraints/allowed-entity-types",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON([]string{}) },
	)
	r.Delete(
		"/subordinates/constraints/allowed-entity-types/:entityType",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)

	// Max path length
	r.Get("/subordinates/constraints/max-path-length", func(c *fiber.Ctx) error { return c.JSON(nil) })
	r.Put("/subordinates/constraints/max-path-length", func(c *fiber.Ctx) error { return c.JSON(0) })
}

func registerSubordinateKeys(r fiber.Router) {
	g := r.Group("/subordinates/:subordinateID/jwks")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	g.Post(
		"/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{"jwks": []fiber.Map{}}) },
	)
	g.Delete("/:kid", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateCrit(r fiber.Router) {
	g := r.Group("/subordinates/crit")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	g.Delete("/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}

func registerSubordinateMetadataPolicyCrit(r fiber.Router) {
	g := r.Group("/subordinates/metadata-policy-crit")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	g.Delete("/:operator", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}
