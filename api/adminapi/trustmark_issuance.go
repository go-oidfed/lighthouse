package adminapi

import "github.com/gofiber/fiber/v2"

func registerTrustMarkIssuance(r fiber.Router) {
	base := "/trust-marks/issuance-spec/:trustMarkSpecID/subjects"
	r.Get("/trust-marks/issuance-spec", func(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) })
	r.Post(
		"/trust-marks/issuance-spec",
		func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) },
	)
	r.Get("/trust-marks/issuance-spec/:trustMarkSpecID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Put("/trust-marks/issuance-spec/:trustMarkSpecID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Patch("/trust-marks/issuance-spec/:trustMarkSpecID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Delete(
		"/trust-marks/issuance-spec/:trustMarkSpecID",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)

	r.Get(base, func(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) })
	r.Post(base, func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{}) })
	r.Get(base+"/:trustMarkSubjectID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Put(base+"/:trustMarkSubjectID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Patch(base+"/:trustMarkSubjectID", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Delete(base+"/:trustMarkSubjectID", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	r.Put(base+"/:trustMarkSubjectID/status", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Get(base+"/:trustMarkSubjectID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Put(base+"/:trustMarkSubjectID/additional-claims", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	r.Get(
		base+"/:trustMarkSubjectID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	r.Put(
		base+"/:trustMarkSubjectID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) },
	)
	r.Delete(
		base+"/:trustMarkSubjectID/additional-claims/:additionalClaimsID",
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
	)
}
