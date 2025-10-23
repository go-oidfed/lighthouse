package adminapi

import "github.com/gofiber/fiber/v2"

func registerKeys(r fiber.Router) {
	g := r.Group("/entity-configuration/keys")
	g.Get("/", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"jwks": []fiber.Map{}}) })
	g.Post("/", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Post(
		"/:kid",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{"kid": c.Params("kid")}) },
	)
	g.Delete("/:kid", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}
