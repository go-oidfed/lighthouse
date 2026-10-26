package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerAuthorityHints wires handlers using an AuthorityHintsStore abstraction.
// applyHook, when non-nil, is invoked after any successful mutation (create,
// update, delete) so that outbound JWKS-sync hooks can be re-attached at
// runtime; a non-nil error is logged but does not fail the already-persisted
// request.
func registerAuthorityHints(r fiber.Router, store model.AuthorityHintsStore, applyHook func() error) {
	g := r.Group("/entity-configuration/authority-hints")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	// validate checks an AddAuthorityHint body. entity_id is a required
	// identity field for both create (POST) and full-replacement update (PUT);
	// it is never derived from the path or DB id and may be changed (PUT only,
	// colliding values 409). Partial updates use PATCH, which does not accept
	// entity_id.
	validate := func(req *model.AddAuthorityHint) error {
		if req.EntityID == "" {
			return errors.New("entity_id is required")
		}
		if req.JWKSSync != nil {
			if err := req.JWKSSync.Validate(); err != nil {
				return err
			}
		}
		return nil
	}

	g.Get(
		"/", func(c *fiber.Ctx) error {
			items, err := store.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(items)
		},
	)

	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			var req model.AddAuthorityHint
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if err := validate(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
			}
			item, err := store.Create(req)
			if err != nil {
				if _, ok := errors.AsType[model.AlreadyExistsError](err); ok {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("authority hint already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			reapplyAuthorizationHintsHook(applyHook, item.EntityID)
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

	withCacheWipe.Put(
		"/:authorityHintID", func(c *fiber.Ctx) error {
			var req model.AddAuthorityHint
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if err := validate(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
			}
			item, err := store.Update(c.Params("authorityHintID"), req)
			if err != nil {
				if _, ok := errors.AsType[model.NotFoundError](err); ok {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("authority hint not found"))
				}
				if _, ok := errors.AsType[model.AlreadyExistsError](err); ok {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("authority hint already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			reapplyAuthorizationHintsHook(applyHook, item.EntityID)
			return c.JSON(item)
		},
	)

	withCacheWipe.Patch(
		"/:authorityHintID", func(c *fiber.Ctx) error {
			var patch map[string]any
			if err := c.BodyParser(&patch); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			item, err := store.Patch(c.Params("authorityHintID"), patch)
			if err != nil {
				if _, ok := errors.AsType[model.NotFoundError](err); ok {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("authority hint not found"))
				}
				if _, ok := errors.AsType[model.ValidationError](err); ok {
					return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			reapplyAuthorizationHintsHook(applyHook, item.EntityID)
			return c.JSON(item)
		},
	)

	withCacheWipe.Delete(
		"/:authorityHintID", func(c *fiber.Ctx) error {
			if err := store.Delete(c.Params("authorityHintID")); err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("authority hint not found"))
			}
			reapplyAuthorizationHintsHook(applyHook, c.Params("authorityHintID"))
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}

// reapplyAuthorizationHintsHook invokes the runtime hook applier (if set) and
// logs any failure. The DB mutation has already been committed; a failed
// re-application means the running process keeps its previous hook set until
// the next mutation or restart.
func reapplyAuthorizationHintsHook(applyHook func() error, entityID string) {
	if applyHook == nil {
		return
	}
	if err := applyHook(); err != nil {
		log.Error().Err(err).
			Str("entity_id", entityID).
			Msg("authority hints: failed to re-apply JWKS sync hooks; changes will take effect after the next mutation or restart")
	}
}
