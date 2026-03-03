package adminapi

import (
	"fmt"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinatesBase registers basic CRUD endpoints for subordinates.
func registerSubordinatesBase(r fiber.Router, subordinates model.SubordinateStorageBackend) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List subordinates, optional filters via query: entity_type, status
	g.Get("/", handleListSubordinates(subordinates))

	// POST / - Create subordinate
	g.Post("/", handleCreateSubordinate(subordinates))

	// GET /:subordinateID - Get subordinate details
	g.Get("/:subordinateID", handleGetSubordinate(subordinates))

	// PUT /:subordinateID - Update subordinate details (replace basic fields)
	withCacheWipe.Put("/:subordinateID", handleUpdateSubordinate(subordinates))

	// DELETE /:subordinateID - Delete subordinate
	withCacheWipe.Delete("/:subordinateID", handleDeleteSubordinate(subordinates))

	// PUT /:subordinateID/status - Update subordinate status
	withCacheWipe.Put("/:subordinateID/status", handleUpdateSubordinateStatus(subordinates))

	// GET /:subordinateID/history - Subordinate history (placeholder)
	g.Get("/:subordinateID/history", handleSubordinateHistory())
}

func handleListSubordinates(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type request struct {
			Status     *model.Status `query:"-"`
			EntityType []string      `query:"entity_type"`
		}
		var req request
		var infos []model.BasicSubordinateInfo
		var err error

		if err = c.QueryParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}

		s := c.Query("status")
		if s != "" {
			st, err := model.ParseStatus(s)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(
					oidfed.ErrorInvalidRequest(
						fmt.Sprintf("invalid status: %s", err.Error()),
					),
				)
			}
			req.Status = &st
		}

		if req.EntityType != nil {
			if req.Status != nil {
				infos, err = subordinates.GetByStatusAndAnyEntityType(*req.Status, req.EntityType)
			} else {
				infos, err = subordinates.GetByAnyEntityType(req.EntityType)
			}
		} else if req.Status != nil {
			infos, err = subordinates.GetByStatus(*req.Status)
		} else {
			infos, err = subordinates.GetAll()
		}
		if err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(infos)
	}
}

func handleCreateSubordinate(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req model.ExtendedSubordinateInfo
		req.Status = DefaultSubordinateStatus
		if err := c.BodyParser(&req); err != nil {
			return writeBadBody(c)
		}
		if req.EntityID == "" {
			return writeBadRequest(c, "missing entity_id")
		}
		if !req.Status.Valid() {
			return writeBadRequest(c, "invalid status")
		}
		if err := subordinates.Add(req); err != nil {
			return writeServerError(c, err)
		}
		stored, err := subordinates.Get(req.EntityID)
		if err != nil {
			return writeServerError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(stored)
	}
}

func handleGetSubordinate(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		info, err := subordinates.GetByDBID(id)
		if err != nil {
			return writeServerError(c, err)
		}
		if info == nil {
			return writeNotFound(c, "subordinate not found")
		}
		return c.JSON(*info)
	}
}

func handleUpdateSubordinate(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		existing, err := subordinates.GetByDBID(id)
		if err != nil {
			return writeServerError(c, err)
		}
		if existing == nil {
			return writeNotFound(c, "subordinate not found")
		}

		var body struct {
			Description           *string  `json:"description"`
			RegisteredEntityTypes []string `json:"registered_entity_types"`
		}
		if err = c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}
		if body.Description != nil {
			existing.Description = *body.Description
		}
		if body.RegisteredEntityTypes != nil {
			subordinateEntityTypes := make([]model.SubordinateEntityType, len(body.RegisteredEntityTypes))
			for i, et := range body.RegisteredEntityTypes {
				subordinateEntityTypes[i] = model.SubordinateEntityType{EntityType: et}
			}
			existing.SubordinateEntityTypes = subordinateEntityTypes
		}
		if err = subordinates.Update(existing.EntityID, *existing); err != nil {
			return writeServerError(c, err)
		}
		return c.JSON(existing)
	}
}

func handleDeleteSubordinate(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		if err := subordinates.DeleteByDBID(id); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleUpdateSubordinateStatus(subordinates model.SubordinateStorageBackend) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		var status model.Status
		if err := c.BodyParser(&status); err != nil {
			return writeBadBody(c)
		}
		if err := subordinates.UpdateStatusByDBID(id, status); err != nil {
			return writeServerError(c, err)
		}

		info, err := subordinates.GetByDBID(id)
		if err != nil {
			return writeServerError(c, err)
		}
		if info == nil {
			return writeNotFound(c, "subordinate not found")
		}
		return c.JSON(info)
	}
}

func handleSubordinateHistory() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Placeholder: hook into events model.SubordinateEvent when available
		return c.JSON(fiber.Map{"events": []fiber.Map{}})
	}
}
