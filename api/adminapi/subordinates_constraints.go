package adminapi

import (
	"encoding/json"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// generalConstraintsStore wraps KV operations for general constraints.
type generalConstraintsStore struct {
	kv model.KeyValueStore
}

func (s *generalConstraintsStore) load() (*oidfed.ConstraintSpecification, bool, error) {
	var cs oidfed.ConstraintSpecification
	found, err := s.kv.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &cs)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	return &cs, true, nil
}

func (s *generalConstraintsStore) save(cs *oidfed.ConstraintSpecification) error {
	if cs == nil {
		return s.kv.Delete(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints)
	}
	return s.kv.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, *cs)
}

// subordinateConstraintsHandlers groups handlers for subordinate-specific constraint endpoints.
type subordinateConstraintsHandlers struct {
	storages model.Backends
}

func (h *subordinateConstraintsHandlers) getAll(c *fiber.Ctx) error {
	info, err := handleSubordinateLookup(c, h.storages.Subordinates)
	if err != nil {
		return err
	}
	if info.Constraints == nil {
		return c.JSON(oidfed.ConstraintSpecification{})
	}
	return c.JSON(info.Constraints)
}

func (h *subordinateConstraintsHandlers) putAll(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	var body oidfed.ConstraintSpecification
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	if body.MaxPathLength != nil && *body.MaxPathLength < 0 {
		return writeBadRequest(c, "max_path_length must be >= 0")
	}

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) postAll(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	var result *oidfed.ConstraintSpecification
	store := &generalConstraintsStore{kv: h.storages.KV}

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
		}
		general, found, err := store.load()
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

func (h *subordinateConstraintsHandlers) deleteAll(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) getMaxPathLength(c *fiber.Ctx) error {
	info, err := handleSubordinateLookup(c, h.storages.Subordinates)
	if err != nil {
		return err
	}
	if info.Constraints == nil || info.Constraints.MaxPathLength == nil {
		return writeNotFound(c, "max_path_length not set")
	}
	return c.JSON(*info.Constraints.MaxPathLength)
}

func (h *subordinateConstraintsHandlers) putMaxPathLength(c *fiber.Ctx) error {
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

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) deleteMaxPathLength(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) getNamingConstraints(c *fiber.Ctx) error {
	info, err := handleSubordinateLookup(c, h.storages.Subordinates)
	if err != nil {
		return err
	}
	if info.Constraints == nil || info.Constraints.NamingConstraints == nil {
		return writeNotFound(c, "naming_constraints not set")
	}
	return c.JSON(info.Constraints.NamingConstraints)
}

func (h *subordinateConstraintsHandlers) putNamingConstraints(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	var body oidfed.NamingConstraints
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) deleteNamingConstraints(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) getAllowedEntityTypes(c *fiber.Ctx) error {
	info, err := handleSubordinateLookup(c, h.storages.Subordinates)
	if err != nil {
		return err
	}
	if info.Constraints == nil || info.Constraints.AllowedEntityTypes == nil {
		return writeNotFound(c, "allowed_entity_types not set")
	}
	return c.JSON(info.Constraints.AllowedEntityTypes)
}

func (h *subordinateConstraintsHandlers) putAllowedEntityTypes(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	var body []string
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	var result []string
	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

func (h *subordinateConstraintsHandlers) postAllowedEntityTypes(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	if len(c.Body()) == 0 {
		return writeBadRequest(c, "empty body")
	}
	entityType := string(c.Body())

	var result []string
	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
		}
		if info.Constraints == nil {
			info.Constraints = &oidfed.ConstraintSpecification{}
		}
		for _, t := range info.Constraints.AllowedEntityTypes {
			if t == entityType {
				result = info.Constraints.AllowedEntityTypes
				return nil
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

func (h *subordinateConstraintsHandlers) deleteAllowedEntityType(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	entityType := c.Params("entityType")

	var result []string
	err := h.storages.InTransaction(func(tx *model.Backends) error {
		info, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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

// generalConstraintsHandlers groups handlers for general constraint endpoints.
type generalConstraintsHandlers struct {
	store *generalConstraintsStore
}

func (h *generalConstraintsHandlers) getAll(c *fiber.Ctx) error {
	cs, found, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if !found {
		return writeNotFound(c, "constraints not set")
	}
	return c.JSON(cs)
}

func (h *generalConstraintsHandlers) putAll(c *fiber.Ctx) error {
	var body oidfed.ConstraintSpecification
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	if body.MaxPathLength != nil && *body.MaxPathLength < 0 {
		return writeBadRequest(c, "max_path_length must be >= 0")
	}
	if err := h.store.save(&body); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(body)
}

func (h *generalConstraintsHandlers) getAllowedEntityTypes(c *fiber.Ctx) error {
	cs, found, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if !found || cs.AllowedEntityTypes == nil {
		return writeNotFound(c, "allowed_entity_types not set")
	}
	return c.JSON(cs.AllowedEntityTypes)
}

func (h *generalConstraintsHandlers) putAllowedEntityTypes(c *fiber.Ctx) error {
	var body []string
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	cs, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if cs == nil {
		cs = &oidfed.ConstraintSpecification{}
	}
	cs.AllowedEntityTypes = body
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(cs.AllowedEntityTypes)
}

func (h *generalConstraintsHandlers) postAllowedEntityTypes(c *fiber.Ctx) error {
	if len(c.Body()) == 0 {
		return writeBadRequest(c, "empty body")
	}
	entityType := string(c.Body())
	cs, _, err := h.store.load()
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
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(cs.AllowedEntityTypes)
}

func (h *generalConstraintsHandlers) deleteAllowedEntityType(c *fiber.Ctx) error {
	entityType := c.Params("entityType")
	cs, found, err := h.store.load()
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
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(cs.AllowedEntityTypes)
}

func (h *generalConstraintsHandlers) getMaxPathLength(c *fiber.Ctx) error {
	cs, found, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if !found || cs.MaxPathLength == nil {
		return writeNotFound(c, "max_path_length not set")
	}
	return c.JSON(*cs.MaxPathLength)
}

func (h *generalConstraintsHandlers) putMaxPathLength(c *fiber.Ctx) error {
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
	cs, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if cs == nil {
		cs = &oidfed.ConstraintSpecification{}
	}
	cs.MaxPathLength = &mpl
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(mpl)
}

func (h *generalConstraintsHandlers) deleteMaxPathLength(c *fiber.Ctx) error {
	cs, found, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if !found {
		return c.SendStatus(fiber.StatusNoContent)
	}
	cs.MaxPathLength = nil
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *generalConstraintsHandlers) getNamingConstraints(c *fiber.Ctx) error {
	cs, found, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if !found || cs.NamingConstraints == nil {
		return writeNotFound(c, "naming_constraints not set")
	}
	return c.JSON(cs.NamingConstraints)
}

func (h *generalConstraintsHandlers) putNamingConstraints(c *fiber.Ctx) error {
	var body oidfed.NamingConstraints
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}
	cs, _, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if cs == nil {
		cs = &oidfed.ConstraintSpecification{}
	}
	cs.NamingConstraints = &body
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(body)
}

func (h *generalConstraintsHandlers) deleteNamingConstraints(c *fiber.Ctx) error {
	cs, found, err := h.store.load()
	if err != nil {
		return writeServerError(c, err)
	}
	if !found {
		return c.SendStatus(fiber.StatusNoContent)
	}
	cs.NamingConstraints = nil
	if err := h.store.save(cs); err != nil {
		return writeServerError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// registerSubordinateConstraints registers subordinate-specific constraint endpoints.
func registerSubordinateConstraints(r fiber.Router, storages model.Backends) {
	g := r.Group("/subordinates/:subordinateID/constraints")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	h := &subordinateConstraintsHandlers{storages: storages}

	g.Get("/", h.getAll)
	withCacheWipe.Put("/", h.putAll)
	withCacheWipe.Post("/", h.postAll)
	withCacheWipe.Delete("/", h.deleteAll)

	g.Get("/max-path-length", h.getMaxPathLength)
	withCacheWipe.Put("/max-path-length", h.putMaxPathLength)
	withCacheWipe.Delete("/max-path-length", h.deleteMaxPathLength)

	g.Get("/naming-constraints", h.getNamingConstraints)
	withCacheWipe.Put("/naming-constraints", h.putNamingConstraints)
	withCacheWipe.Delete("/naming-constraints", h.deleteNamingConstraints)

	g.Get("/allowed-entity-types", h.getAllowedEntityTypes)
	withCacheWipe.Put("/allowed-entity-types", h.putAllowedEntityTypes)
	withCacheWipe.Post("/allowed-entity-types", h.postAllowedEntityTypes)
	withCacheWipe.Delete("/allowed-entity-types/:entityType", h.deleteAllowedEntityType)
}

// registerGeneralConstraints registers general constraint endpoints.
func registerGeneralConstraints(r fiber.Router, kv model.KeyValueStore) {
	g := r.Group("/subordinates/constraints")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	store := &generalConstraintsStore{kv: kv}
	h := &generalConstraintsHandlers{store: store}

	g.Get("/", h.getAll)
	withCacheWipe.Put("/", h.putAll)

	g.Get("/allowed-entity-types", h.getAllowedEntityTypes)
	withCacheWipe.Put("/allowed-entity-types", h.putAllowedEntityTypes)
	withCacheWipe.Post("/allowed-entity-types", h.postAllowedEntityTypes)
	withCacheWipe.Delete("/allowed-entity-types/:entityType", h.deleteAllowedEntityType)

	g.Get("/max-path-length", h.getMaxPathLength)
	withCacheWipe.Put("/max-path-length", h.putMaxPathLength)
	withCacheWipe.Delete("/max-path-length", h.deleteMaxPathLength)

	g.Get("/naming-constraints", h.getNamingConstraints)
	withCacheWipe.Put("/naming-constraints", h.putNamingConstraints)
	withCacheWipe.Delete("/naming-constraints", h.deleteNamingConstraints)
}
