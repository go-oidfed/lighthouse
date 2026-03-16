package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustMarkConfigInvalidator is implemented by types that cache trust mark configurations
// and need to be invalidated when trust marks change.
type TrustMarkConfigInvalidator interface {
	Invalidate()
}

// handleStoreError handles common store errors and returns appropriate HTTP responses.
// Returns nil if err is nil, otherwise returns an error response.
func handleStoreError(c *fiber.Ctx, err error, notFoundMsg string) error {
	if err == nil {
		return nil
	}
	var notFoundError model.NotFoundError
	if errors.As(err, &notFoundError) {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(notFoundMsg))
	}
	var alreadyExistsError model.AlreadyExistsError
	if errors.As(err, &alreadyExistsError) {
		return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	var validationError model.ValidationError
	if errors.As(err, &validationError) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
}

// registerEntityTrustMarks wires handlers for managing trust marks in the entity configuration.
// The configInvalidator is called after successful mutations to invalidate cached configs.
func registerEntityTrustMarks(
	r fiber.Router,
	store model.PublishedTrustMarksStore,
	configInvalidator TrustMarkConfigInvalidator,
) {
	g := r.Group("/entity-configuration/trust-marks")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	// Helper to invalidate config cache after successful mutations
	invalidateConfigs := func() {
		if configInvalidator != nil {
			configInvalidator.Invalidate()
		}
	}

	// GET / - List all trust marks
	g.Get(
		"/", func(c *fiber.Ctx) error {
			items, err := store.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(items)
		},
	)

	// POST / - Create a trust mark
	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			var req model.AddTrustMark
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			item, err := store.Create(req)
			if err != nil {
				return handleStoreError(c, err, "trust mark not found")
			}
			invalidateConfigs()
			return c.Status(fiber.StatusCreated).JSON(item)
		},
	)

	// GET /:trustMarkID - Get a single trust mark
	g.Get(
		"/:trustMarkID", func(c *fiber.Ctx) error {
			item, err := store.Get(c.Params("trustMarkID"))
			if err != nil {
				return handleStoreError(c, err, "trust mark not found")
			}
			return c.JSON(item)
		},
	)

	// PUT /:trustMarkID - Replace a trust mark entirely
	withCacheWipe.Put(
		"/:trustMarkID", func(c *fiber.Ctx) error {
			var req model.AddTrustMark
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			item, err := store.Update(c.Params("trustMarkID"), req)
			if err != nil {
				return handleStoreError(c, err, "trust mark not found")
			}
			invalidateConfigs()
			return c.JSON(item)
		},
	)

	// PATCH /:trustMarkID - Partially update a trust mark
	withCacheWipe.Patch(
		"/:trustMarkID", func(c *fiber.Ctx) error {
			var req model.UpdateTrustMark
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			item, err := store.Patch(c.Params("trustMarkID"), req)
			if err != nil {
				return handleStoreError(c, err, "trust mark not found")
			}
			invalidateConfigs()
			return c.JSON(item)
		},
	)

	// DELETE /:trustMarkID - Delete a trust mark
	withCacheWipe.Delete(
		"/:trustMarkID", func(c *fiber.Ctx) error {
			if err := store.Delete(c.Params("trustMarkID")); err != nil {
				return handleStoreError(c, err, "trust mark not found")
			}
			invalidateConfigs()
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}
