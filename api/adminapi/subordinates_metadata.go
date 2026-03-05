package adminapi

import (
	"encoding/json"
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinateMetadata registers metadata endpoints for subordinates.
func registerSubordinateMetadata(
	r fiber.Router,
	storages model.Backends,
) {
	g := r.Group("/subordinates/:subordinateID/metadata")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get full subordinate-specific metadata
	g.Get("/", handleGetSubordinateMetadata(storages.Subordinates))

	// PUT / - Replace full subordinate-specific metadata (transactional)
	withCacheWipe.Put("/", handlePutSubordinateMetadata(storages))

	// Entity type endpoints
	g.Get("/:entityType", handleGetSubordinateMetadataEntityType(storages.Subordinates))
	withCacheWipe.Put("/:entityType", handlePutSubordinateMetadataEntityType(storages))
	withCacheWipe.Post("/:entityType", handlePostSubordinateMetadataEntityType(storages))
	withCacheWipe.Delete("/:entityType", handleDeleteSubordinateMetadataEntityType(storages))

	// Claim endpoints
	g.Get("/:entityType/:claim", handleGetSubordinateMetadataClaim(storages.Subordinates))
	withCacheWipe.Put("/:entityType/:claim", handlePutSubordinateMetadataClaim(storages))
	withCacheWipe.Delete("/:entityType/:claim", handleDeleteSubordinateMetadataClaim(storages))
}

func handleGetSubordinateMetadata(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.Metadata == nil {
			return writeNotFound(c, "metadata not found")
		}
		return c.JSON(info.Metadata)
	}
}

func handlePutSubordinateMetadata(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var body oidfed.Metadata
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		var result *oidfed.Metadata
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			info.Metadata = &body
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			// Record metadata update event within transaction
			if err := RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeMetadataUpdated); err != nil {
				return err
			}
			result = &body
			return nil
		})

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.JSON(result)
	}
}

func handleGetSubordinateMetadataEntityType(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		m := getEntityMetadata(info.Metadata, et)
		if m == nil {
			return writeNotFound(c, "metadata not found")
		}
		return c.JSON(m)
	}
}

func handlePutSubordinateMetadataEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		var body map[string]any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}

		var result map[string]any
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			if info.Metadata == nil {
				info.Metadata = &oidfed.Metadata{}
			}
			setEntityMetadata(info.Metadata, et, body)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			// Record metadata update event within transaction
			if err := RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeMetadataUpdated, WithMessage("entity type: "+et)); err != nil {
				return err
			}
			result = body
			return nil
		})

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.JSON(result)
	}
}

func handlePostSubordinateMetadataEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		var body map[string]any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}

		var result map[string]any
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			if info.Metadata == nil {
				info.Metadata = &oidfed.Metadata{}
			}
			existing := getEntityMetadata(info.Metadata, et)
			if existing == nil {
				existing = map[string]any{}
			}
			for k, v := range body {
				existing[k] = v
			}
			setEntityMetadata(info.Metadata, et, existing)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			// Record metadata update event within transaction
			if err := RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeMetadataUpdated, WithMessage("entity type: "+et)); err != nil {
				return err
			}
			result = existing
			return nil
		})

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.JSON(result)
	}
}

func handleDeleteSubordinateMetadataEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			if getEntityMetadata(info.Metadata, et) == nil {
				return model.NotFoundError("metadata not found")
			}
			if info.Metadata == nil {
				info.Metadata = &oidfed.Metadata{}
			}
			deleteEntityMetadata(info.Metadata, et)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			// Record metadata deleted event within transaction
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeMetadataDeleted, WithMessage("entity type: "+et))
		})

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateMetadataClaim(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		et := c.Params("entityType")
		claim := c.Params("claim")
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		m := getEntityMetadata(info.Metadata, et)
		if m == nil {
			return writeNotFound(c, "metadata not found")
		}
		v, ok := m[claim]
		if !ok {
			return writeNotFound(c, "metadata not found")
		}
		return c.JSON(v)
	}
}

func handlePutSubordinateMetadataClaim(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")
		var body any
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return writeBadBody(c)
		}

		var result any
		var created bool
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			created = false
			if info.Metadata == nil {
				info.Metadata = &oidfed.Metadata{}
			}
			m := getEntityMetadata(info.Metadata, et)
			if m == nil {
				m = map[string]any{}
				created = true
			}
			if _, ok := m[claim]; !ok {
				created = true
			}
			m[claim] = body
			setEntityMetadata(info.Metadata, et, m)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			// Record metadata update event within transaction
			if err := RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeMetadataUpdated, WithMessage(et+"."+claim)); err != nil {
				return err
			}
			result = body
			return nil
		})

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		if created {
			return c.Status(fiber.StatusCreated).JSON(result)
		}
		return c.JSON(result)
	}
}

func handleDeleteSubordinateMetadataClaim(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		et := c.Params("entityType")
		claim := c.Params("claim")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}

			m := getEntityMetadata(info.Metadata, et)
			if m == nil {
				return model.NotFoundError("metadata not found")
			}
			if _, ok := m[claim]; !ok {
				return model.NotFoundError("metadata not found")
			}
			delete(m, claim)
			if info.Metadata == nil {
				info.Metadata = &oidfed.Metadata{}
			}
			setEntityMetadata(info.Metadata, et, m)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			// Record metadata deleted event within transaction
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeMetadataDeleted, WithMessage(et+"."+claim))
		})

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
