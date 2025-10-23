package adminapi

import "github.com/gofiber/fiber/v2"

func registerEntityMetadata(r fiber.Router) {
	g := r.Group("/entity-configuration/metadata")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
}

func registerEntityMetadataPolicies(r fiber.Router) {
	g := r.Group("/entity-configuration/metadata-policies")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
}
