package adminapi

import (
	"github.com/gofiber/fiber/v2"
)

func registerEntityTrustMarks(r fiber.Router) {
	g := r.Group("/entity-configuration/trust-marks")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Get("/:trustMarkID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/:trustMarkID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/:trustMarkID", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}
