package adminapi

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinateConstraints registers subordinate-specific constraint endpoints.
// All write operations are wrapped in transactions for atomicity.
func registerSubordinateConstraints(
	r fiber.Router,
	storages model.Backends,
) {
	g := r.Group("/subordinates/:subordinateID/constraints")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get subordinate constraints
	g.Get("/", handleGetSubordinateConstraints(storages.Subordinates))

	// PUT / - Replace subordinate constraints (transactional)
	withCacheWipe.Put("/", handlePutSubordinateConstraints(storages))

	// POST / - Copy general constraints to subordinate (transactional)
	withCacheWipe.Post("/", handlePostSubordinateConstraints(storages))

	// DELETE / - Delete subordinate constraints (transactional)
	withCacheWipe.Delete("/", handleDeleteSubordinateConstraints(storages))

	// Max path length endpoints
	g.Get("/max-path-length", handleGetSubordinateMaxPathLength(storages.Subordinates))
	withCacheWipe.Put("/max-path-length", handlePutSubordinateMaxPathLength(storages))
	withCacheWipe.Delete("/max-path-length", handleDeleteSubordinateMaxPathLength(storages))

	// Naming constraints endpoints
	g.Get("/naming-constraints", handleGetSubordinateNamingConstraints(storages.Subordinates))
	withCacheWipe.Put("/naming-constraints", handlePutSubordinateNamingConstraints(storages))
	withCacheWipe.Delete("/naming-constraints", handleDeleteSubordinateNamingConstraints(storages))

	// Allowed entity types endpoints
	g.Get("/allowed-entity-types", handleGetSubordinateAllowedEntityTypes(storages.Subordinates))
	withCacheWipe.Put("/allowed-entity-types", handlePutSubordinateAllowedEntityTypes(storages))
	withCacheWipe.Post("/allowed-entity-types", handlePostSubordinateAllowedEntityTypes(storages))
	withCacheWipe.Delete("/allowed-entity-types/:entityType", handleDeleteSubordinateAllowedEntityType(storages))
}

// Helper to get general constraints from KV store
func loadGeneralConstraints(kv model.KeyValueStore) (*oidfed.ConstraintSpecification, bool, error) {
	var cs oidfed.ConstraintSpecification
	found, err := kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &cs)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	return &cs, true, nil
}

func handleGetSubordinateConstraints(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.Constraints == nil {
			return c.JSON(oidfed.ConstraintSpecification{})
		}
		return c.JSON(info.Constraints)
	}
}

func handlePutSubordinateConstraints(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var body oidfed.ConstraintSpecification
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		if body.MaxPathLength != nil && *body.MaxPathLength < 0 {
			return writeBadRequest(c, "max_path_length must be >= 0")
		}

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			info.Constraints = &body
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsUpdated)
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(body)
	}
}

func handlePostSubordinateConstraints(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var result *oidfed.ConstraintSpecification

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			general, found, err := loadGeneralConstraints(tx.KV)
			if err != nil {
				return err
			}
			if !found || general == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			} else {
				copied := *general
				info.Constraints = &copied
			}
			if err = tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = info.Constraints
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsUpdated, WithMessage("copied from general"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	}
}

func handleDeleteSubordinateConstraints(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			info.Constraints = nil
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsDeleted)
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateMaxPathLength(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.Constraints == nil || info.Constraints.MaxPathLength == nil {
			return writeNotFound(c, "max_path_length not set")
		}
		return c.JSON(*info.Constraints.MaxPathLength)
	}
}

func handlePutSubordinateMaxPathLength(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		if len(c.Body()) == 0 {
			return writeBadRequest(c, "empty body")
		}
		var mpl int
		if err := json.Unmarshal(c.Body(), &mpl); err != nil {
			return writeBadBody(c)
		}
		if mpl < 0 {
			return writeBadRequest(c, "max_path_length must be >= 0")
		}

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			info.Constraints.MaxPathLength = &mpl
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsUpdated, WithMessage("max_path_length"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(mpl)
	}
}

func handleDeleteSubordinateMaxPathLength(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints != nil {
				info.Constraints.MaxPathLength = nil
				if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
					return err
				}
				return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsDeleted, WithMessage("max_path_length"))
			}
			return nil
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateNamingConstraints(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.Constraints == nil || info.Constraints.NamingConstraints == nil {
			return writeNotFound(c, "naming_constraints not set")
		}
		return c.JSON(info.Constraints.NamingConstraints)
	}
}

func handlePutSubordinateNamingConstraints(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var body oidfed.NamingConstraints
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			info.Constraints.NamingConstraints = &body
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsUpdated, WithMessage("naming_constraints"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(body)
	}
}

func handleDeleteSubordinateNamingConstraints(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints != nil {
				info.Constraints.NamingConstraints = nil
				if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
					return err
				}
				return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsDeleted, WithMessage("naming_constraints"))
			}
			return nil
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetSubordinateAllowedEntityTypes(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		info, err := handleSubordinateLookup(c, subordinates)
		if err != nil {
			return err
		}
		if info.Constraints == nil || info.Constraints.AllowedEntityTypes == nil {
			return writeNotFound(c, "allowed_entity_types not set")
		}
		return c.JSON(info.Constraints.AllowedEntityTypes)
	}
}

func handlePutSubordinateAllowedEntityTypes(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var body []string
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		var result []string
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			info.Constraints.AllowedEntityTypes = body
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = info.Constraints.AllowedEntityTypes
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsUpdated, WithMessage("allowed_entity_types"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.JSON(result)
	}
}

func handlePostSubordinateAllowedEntityTypes(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		if len(c.Body()) == 0 {
			return writeBadRequest(c, "empty body")
		}
		entityType := string(c.Body())

		var result []string
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints == nil {
				info.Constraints = &oidfed.ConstraintSpecification{}
			}
			// Check if already exists
			for _, t := range info.Constraints.AllowedEntityTypes {
				if t == entityType {
					result = info.Constraints.AllowedEntityTypes
					return nil // Already exists, no update needed
				}
			}
			info.Constraints.AllowedEntityTypes = append(info.Constraints.AllowedEntityTypes, entityType)
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = info.Constraints.AllowedEntityTypes
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsUpdated, WithMessage("allowed_entity_types"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	}
}

func handleDeleteSubordinateAllowedEntityType(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		entityType := c.Params("entityType")

		var result []string
		err := storages.InTransaction(func(tx *model.Backends) error {
			info, err := tx.Subordinates.GetByDBID(id)
			if err != nil {
				return err
			}
			if info == nil {
				return model.NotFoundError("subordinate not found")
			}
			if info.Constraints == nil {
				result = nil
				return nil
			}
			updated := make([]string, 0, len(info.Constraints.AllowedEntityTypes))
			removed := false
			for _, t := range info.Constraints.AllowedEntityTypes {
				if t == entityType {
					removed = true
					continue
				}
				updated = append(updated, t)
			}
			if !removed {
				result = info.Constraints.AllowedEntityTypes
				return nil
			}
			info.Constraints.AllowedEntityTypes = updated
			if err := tx.Subordinates.Update(info.EntityID, *info); err != nil {
				return err
			}
			result = info.Constraints.AllowedEntityTypes
			return RecordEvent(tx.SubordinateEvents, info.ID, model.EventTypeConstraintsDeleted, WithMessage("allowed_entity_types"))
		})
		if err != nil {
			return handleTxError(c, err)
		}
		if result == nil {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.JSON(result)
	}
}

// registerGeneralConstraints registers general constraint endpoints.
func registerGeneralConstraints(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/constraints")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - Get general constraints
	g.Get("/", handleGetGeneralConstraints(kv))

	// PUT / - Replace general constraints
	withCacheWipe.Put("/", handlePutGeneralConstraints(kv))

	// Allowed entity types endpoints
	g.Get("/allowed-entity-types", handleGetGeneralAllowedEntityTypes(kv))
	withCacheWipe.Put("/allowed-entity-types", handlePutGeneralAllowedEntityTypes(kv))
	withCacheWipe.Post("/allowed-entity-types", handlePostGeneralAllowedEntityTypes(kv))
	withCacheWipe.Delete("/allowed-entity-types/:entityType", handleDeleteGeneralAllowedEntityType(kv))

	// Max path length endpoints
	g.Get("/max-path-length", handleGetGeneralMaxPathLength(kv))
	withCacheWipe.Put("/max-path-length", handlePutGeneralMaxPathLength(kv))
	withCacheWipe.Delete("/max-path-length", handleDeleteGeneralMaxPathLength(kv))

	// Naming constraints endpoints
	g.Get("/naming-constraints", handleGetGeneralNamingConstraints(kv))
	withCacheWipe.Put("/naming-constraints", handlePutGeneralNamingConstraints(kv))
	withCacheWipe.Delete("/naming-constraints", handleDeleteGeneralNamingConstraints(kv))
}

func saveGeneralConstraints(kv model.KeyValueStore, cs *oidfed.ConstraintSpecification) error {
	if cs == nil {
		return kv.Delete(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints)
	}
	return kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, *cs)
}

func handleGetGeneralConstraints(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return writeNotFound(c, "constraints not set")
		}
		return c.JSON(cs)
	}
}

func handlePutGeneralConstraints(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body oidfed.ConstraintSpecification
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		if body.MaxPathLength != nil && *body.MaxPathLength < 0 {
			return writeBadRequest(c, "max_path_length must be >= 0")
		}
		if err := saveGeneralConstraints(kv, &body); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(body)
	}
}

func handleGetGeneralAllowedEntityTypes(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return writeNotFound(c, "allowed_entity_types not set")
		}
		if cs.AllowedEntityTypes == nil {
			return writeNotFound(c, "allowed_entity_types not set")
		}
		return c.JSON(cs.AllowedEntityTypes)
	}
}

func handlePutGeneralAllowedEntityTypes(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body []string
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		cs, _, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if cs == nil {
			cs = &oidfed.ConstraintSpecification{}
		}
		cs.AllowedEntityTypes = body
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(cs.AllowedEntityTypes)
	}
}

func handlePostGeneralAllowedEntityTypes(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if len(c.Body()) == 0 {
			return writeBadRequest(c, "empty body")
		}
		entityType := string(c.Body())
		cs, _, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if cs == nil {
			cs = &oidfed.ConstraintSpecification{}
		}
		for _, t := range cs.AllowedEntityTypes {
			if t == entityType {
				return c.Status(fiber.StatusCreated).JSON(cs.AllowedEntityTypes)
			}
		}
		cs.AllowedEntityTypes = append(cs.AllowedEntityTypes, entityType)
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(cs.AllowedEntityTypes)
	}
}

func handleDeleteGeneralAllowedEntityType(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		entityType := c.Params("entityType")
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return c.SendStatus(fiber.StatusNoContent)
		}
		updated := make([]string, 0, len(cs.AllowedEntityTypes))
		removed := false
		for _, t := range cs.AllowedEntityTypes {
			if t == entityType {
				removed = true
				continue
			}
			updated = append(updated, t)
		}
		if !removed {
			return c.SendStatus(fiber.StatusNoContent)
		}
		cs.AllowedEntityTypes = updated
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(cs.AllowedEntityTypes)
	}
}

func handleGetGeneralMaxPathLength(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found || cs.MaxPathLength == nil {
			return writeNotFound(c, "max_path_length not set")
		}
		return c.JSON(*cs.MaxPathLength)
	}
}

func handlePutGeneralMaxPathLength(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if len(c.Body()) == 0 {
			return writeBadRequest(c, "empty body")
		}
		var mpl int
		if err := json.Unmarshal(c.Body(), &mpl); err != nil {
			return writeBadBody(c)
		}
		if mpl < 0 {
			return writeBadRequest(c, "max_path_length must be >= 0")
		}
		cs, _, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if cs == nil {
			cs = &oidfed.ConstraintSpecification{}
		}
		cs.MaxPathLength = &mpl
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(mpl)
	}
}

func handleDeleteGeneralMaxPathLength(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return c.SendStatus(fiber.StatusNoContent)
		}
		cs.MaxPathLength = nil
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleGetGeneralNamingConstraints(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found || cs.NamingConstraints == nil {
			return writeNotFound(c, "naming_constraints not set")
		}
		return c.JSON(cs.NamingConstraints)
	}
}

func handlePutGeneralNamingConstraints(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body oidfed.NamingConstraints
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		cs, _, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if cs == nil {
			cs = &oidfed.ConstraintSpecification{}
		}
		cs.NamingConstraints = &body
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(body)
	}
}

func handleDeleteGeneralNamingConstraints(kv model.KeyValueStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cs, found, err := loadGeneralConstraints(kv)
		if err != nil {
			return writeServerError(c, err)
		}
		if !found {
			return c.SendStatus(fiber.StatusNoContent)
		}
		cs.NamingConstraints = nil
		if err := saveGeneralConstraints(kv, cs); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
