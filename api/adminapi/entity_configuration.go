package adminapi

import "github.com/gofiber/fiber/v2"

func registerEntityConfiguration(r fiber.Router) {
	g := r.Group("/entity-configuration")
	g.Get(
		"/", func(c *fiber.Ctx) error {
			return c.JSON(
				fiber.Map{
					"status": "ok",
					"note":   "placeholder entity configuration",
				},
			)
		},
	)

	// Additional Claims collection
	g.Get("/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/additional-claims", func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	g.Get(
		"/additional-claims/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"claim": c.Params("claim")}) },
	)
	g.Put(
		"/additional-claims/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"claim": c.Params("claim")}) },
	)
	g.Delete("/additional-claims/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	// Critical claims
	g.Get("/crit", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	g.Put("/crit", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	g.Post("/crit", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	g.Delete("/crit/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	// Metadata
	g.Get(
		"/metadata/:entityType/:claim", func(c *fiber.Ctx) error {
			return c.JSON(
				fiber.Map{
					"entityType": c.Params("entityType"),
					"claim":      c.Params("claim"),
				},
			)
		},
	)
	g.Put("/metadata/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/metadata/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	g.Get("/metadata/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/metadata/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/metadata/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/metadata/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	// Metadata policies
	g.Get("/metadata-policies/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/metadata-policies/:entityType/:claim", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete(
		"/metadata-policies/:entityType/:claim", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)
	g.Get("/metadata-policies/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Put("/metadata-policies/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Post("/metadata-policies/:entityType", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	g.Delete("/metadata-policies/:entityType", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	// Metadata-policy crit operators
	r.Get("/metadata-policy-crit", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	r.Put("/metadata-policy-crit", func(c *fiber.Ctx) error { return c.JSON([]string{}) })
	r.Post("/metadata-policy-crit", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	r.Delete("/metadata-policy-crit/:operator", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
}
