package adminapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinatesBase registers basic CRUD endpoints for subordinates.
func registerSubordinatesBase(
	r fiber.Router,
	storages model.Backends,
) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List subordinates, optional filters via query: entity_type, status
	g.Get("/", handleListSubordinates(storages.Subordinates))

	// POST / - Create subordinate (transactional)
	g.Post("/", handleCreateSubordinate(storages))

	// GET /:subordinateID - Get subordinate details
	g.Get("/:subordinateID", handleGetSubordinate(storages.Subordinates))

	// PUT /:subordinateID - Update subordinate details (transactional)
	withCacheWipe.Put("/:subordinateID", handleUpdateSubordinate(storages))

	// DELETE /:subordinateID - Delete subordinate (transactional)
	withCacheWipe.Delete("/:subordinateID", handleDeleteSubordinate(storages))

	// PUT /:subordinateID/status - Update subordinate status (transactional)
	withCacheWipe.Put("/:subordinateID/status", handleUpdateSubordinateStatus(storages))

	// GET /:subordinateID/history - Subordinate history
	g.Get("/:subordinateID/history", handleSubordinateHistory(storages.Subordinates, storages.SubordinateEvents))
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

func handleCreateSubordinate(storages model.Backends) fiber.Handler {
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
		// Active status requires at least one key
		if req.Status == model.StatusActive && !jwksHasKeys(&req.JWKS) {
			return writeBadRequest(c, "status cannot be active without keys")
		}

		var stored *model.ExtendedSubordinateInfo
		err := storages.InTransaction(
			func(tx *model.Backends) error {
				if err := tx.Subordinates.Add(req); err != nil {
					return err
				}
				var err error
				stored, err = tx.Subordinates.Get(req.EntityID)
				if err != nil {
					return err
				}
				// Record creation event within transaction
				return RecordEvent(
					tx.SubordinateEvents,
					stored.ID,
					model.EventTypeCreated,
					WithStatus(stored.Status),
					WithMessage(fmt.Sprintf("subordinate created: %s", stored.EntityID)),
				)
			},
		)
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

func handleUpdateSubordinate(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		var body struct {
			Description           *string  `json:"description"`
			RegisteredEntityTypes []string `json:"registered_entity_types"`
		}
		if err := c.BodyParser(&body); err != nil {
			return writeBadBody(c)
		}

		var result *model.ExtendedSubordinateInfo
		err := storages.InTransaction(
			func(tx *model.Backends) error {
				existing, err := tx.Subordinates.GetByDBID(id)
				if err != nil {
					return err
				}
				if existing == nil {
					return model.NotFoundError("subordinate not found")
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
				if err = tx.Subordinates.Update(existing.EntityID, *existing); err != nil {
					return err
				}
				// Record update event within transaction
				if err = RecordEvent(
					tx.SubordinateEvents, existing.ID, model.EventTypeUpdated, WithStatus(existing.Status),
				); err != nil {
					return err
				}
				result = existing
				return nil
			},
		)

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

func handleDeleteSubordinate(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		err := storages.InTransaction(
			func(tx *model.Backends) error {
				// Get subordinate info before deletion
				existing, err := tx.Subordinates.GetByDBID(id)
				if err != nil {
					return err
				}
				if existing == nil {
					return model.NotFoundError("subordinate not found")
				}

				// Delete events first (within transaction, will rollback if subordinate delete fails)
				if err := tx.SubordinateEvents.DeleteBySubordinateID(existing.ID); err != nil {
					return err
				}

				return tx.Subordinates.DeleteByDBID(id)
			},
		)

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

func handleUpdateSubordinateStatus(storages model.Backends) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		// Parse status from plain text body
		statusStr := strings.TrimSpace(string(c.Body()))
		if statusStr == "" {
			return writeBadRequest(c, "status is required")
		}
		status, err := model.ParseStatus(statusStr)
		if err != nil {
			return writeBadRequest(c, err.Error())
		}

		var result *model.ExtendedSubordinateInfo
		err = storages.InTransaction(
			func(tx *model.Backends) error {
				// Get existing info to record old status
				existing, err := tx.Subordinates.GetByDBID(id)
				if err != nil {
					return err
				}
				if existing == nil {
					return model.NotFoundError("subordinate not found")
				}
				oldStatus := existing.Status

				// Check if setting to active - need to verify subordinate has keys
				if status == model.StatusActive {
					if !subordinateHasKeys(existing) {
						return fmt.Errorf("status cannot be active without keys")
					}
				}
				if err := tx.Subordinates.UpdateStatusByDBID(id, status); err != nil {
					return err
				}

				info, err := tx.Subordinates.GetByDBID(id)
				if err != nil {
					return err
				}
				if info == nil {
					return model.NotFoundError("subordinate not found")
				}

				// Record status update event within transaction
				if err = RecordEvent(
					tx.SubordinateEvents,
					info.ID,
					model.EventTypeStatusUpdated,
					WithStatus(status),
					WithMessage(fmt.Sprintf("status changed from %s to %s", oldStatus, status)),
				); err != nil {
					return err
				}
				result = info
				return nil
			},
		)

		if err != nil {
			var nf model.NotFoundError
			if errors.As(err, &nf) {
				return writeNotFound(c, err.Error())
			}
			// Check for the "status cannot be active without keys" error
			if err.Error() == "status cannot be active without keys" {
				return writeBadRequest(c, err.Error())
			}
			return writeServerError(c, err)
		}
		return c.JSON(result)
	}
}

func handleSubordinateHistory(
	subordinates model.SubordinateStorageBackend,
	events model.SubordinateEventStore,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		// Verify subordinate exists
		info, err := subordinates.GetByDBID(id)
		if err != nil {
			return writeServerError(c, err)
		}
		if info == nil {
			return writeNotFound(c, "subordinate not found")
		}

		// Parse query parameters
		var opts model.EventQueryOpts

		// Limit (default: 50, max: 100)
		if limitStr := c.Query("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil {
				return writeBadRequest(c, "invalid limit parameter")
			}
			opts.Limit = limit
		}

		// Offset (default: 0)
		if offsetStr := c.Query("offset"); offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err != nil {
				return writeBadRequest(c, "invalid offset parameter")
			}
			opts.Offset = offset
		}

		// Event type filter
		if eventType := c.Query("type"); eventType != "" {
			opts.EventType = &eventType
		}

		// From timestamp filter
		if fromStr := c.Query("from"); fromStr != "" {
			from, err := strconv.ParseInt(fromStr, 10, 64)
			if err != nil {
				return writeBadRequest(c, "invalid from parameter")
			}
			opts.FromTime = &from
		}

		// To timestamp filter
		if toStr := c.Query("to"); toStr != "" {
			to, err := strconv.ParseInt(toStr, 10, 64)
			if err != nil {
				return writeBadRequest(c, "invalid to parameter")
			}
			opts.ToTime = &to
		}

		// Query events
		eventsList, total, err := events.GetBySubordinateID(info.ID, opts)
		if err != nil {
			return writeServerError(c, err)
		}

		// Build response matching OpenAPI schema
		type eventResponse struct {
			Timestamp int64   `json:"timestamp"`
			Type      string  `json:"type"`
			Status    *string `json:"status,omitempty"`
			Message   *string `json:"message,omitempty"`
			Actor     *string `json:"actor,omitempty"`
		}

		eventsResp := make([]eventResponse, len(eventsList))
		for i, e := range eventsList {
			eventsResp[i] = eventResponse{
				Timestamp: e.Timestamp,
				Type:      e.Type,
				Status:    e.Status,
				Message:   e.Message,
				Actor:     e.Actor,
			}
		}

		// Default limit for pagination display
		limit := opts.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 100 {
			limit = 100
		}

		return c.JSON(
			fiber.Map{
				"events": eventsResp,
				"pagination": fiber.Map{
					"total":  total,
					"limit":  limit,
					"offset": opts.Offset,
				},
			},
		)
	}
}
