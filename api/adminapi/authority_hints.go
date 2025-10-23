package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerAuthorityHints wires handlers using an AuthorityHintsStore abstraction.
func registerAuthorityHints(r fiber.Router, store storage.AuthorityHintsStore) {
	g := r.Group("/entity-configuration/authority-hints")

	g.Get(
		"/", func(c *fiber.Ctx) error {
			items, err := store.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(items)
		},
	)

	g.Post(
		"/", func(c *fiber.Ctx) error {
			var req model.AddAuthorityHint
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.EntityID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
			}
			item, err := store.Create(req)
			if err != nil {
				var alreadyExistsError model.AlreadyExistsError
				if errors.As(err, &alreadyExistsError) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("authority hint already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(item)
		},
	)

	g.Get(
		"/:authorityHintID", func(c *fiber.Ctx) error {
			item, err := store.Get(c.Params("authorityHintID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("authority hint not found"))
			}
			return c.JSON(item)
		},
	)

	g.Put(
		"/:authorityHintID", func(c *fiber.Ctx) error {
			var req model.AddAuthorityHint
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.EntityID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
			}
			item, err := store.Update(c.Params("authorityHintID"), req)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("authority hint not found"))
				}
				var alreadyExistsError model.AlreadyExistsError
				if errors.As(err, &alreadyExistsError) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("authority hint already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(item)
		},
	)

	g.Delete(
		"/:authorityHintID", func(c *fiber.Ctx) error {
			if err := store.Delete(c.Params("authorityHintID")); err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("authority hint not found"))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}
