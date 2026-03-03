package adminapi

import (
	"fmt"
	"strconv"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerSubordinatesBase registers basic CRUD endpoints for subordinates.
func registerSubordinatesBase(
	r fiber.Router,
	subordinates model.SubordinateStorageBackend,
	events model.SubordinateEventStore,
	recorder *eventRecorder,
) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	// GET / - List subordinates, optional filters via query: entity_type, status
	g.Get("/", handleListSubordinates(subordinates))

	// POST / - Create subordinate
	g.Post("/", handleCreateSubordinate(subordinates, recorder))

	// GET /:subordinateID - Get subordinate details
	g.Get("/:subordinateID", handleGetSubordinate(subordinates))

	// PUT /:subordinateID - Update subordinate details (replace basic fields)
	withCacheWipe.Put("/:subordinateID", handleUpdateSubordinate(subordinates, recorder))

	// DELETE /:subordinateID - Delete subordinate
	withCacheWipe.Delete("/:subordinateID", handleDeleteSubordinate(subordinates, recorder))

	// PUT /:subordinateID/status - Update subordinate status
	withCacheWipe.Put("/:subordinateID/status", handleUpdateSubordinateStatus(subordinates, recorder))

	// GET /:subordinateID/history - Subordinate history
	g.Get("/:subordinateID/history", handleSubordinateHistory(subordinates, events))
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

func handleCreateSubordinate(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
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
		if err := subordinates.Add(req); err != nil {
			return writeServerError(c, err)
		}
		stored, err := subordinates.Get(req.EntityID)
		if err != nil {
			return writeServerError(c, err)
		}
		// Record creation event
		recorder.Record(
			stored.ID,
			model.EventTypeCreated,
			WithStatus(stored.Status),
			WithMessage(fmt.Sprintf("subordinate created: %s", stored.EntityID)),
		)
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

func handleUpdateSubordinate(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
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
		// Record update event
		recorder.Record(existing.ID, model.EventTypeUpdated, WithStatus(existing.Status))
		return c.JSON(existing)
	}
}

func handleDeleteSubordinate(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")
		// Get subordinate info before deletion to record event
		existing, err := subordinates.GetByDBID(id)
		if err != nil {
			return writeServerError(c, err)
		}
		if existing == nil {
			return writeNotFound(c, "subordinate not found")
		}
		subordinateID := existing.ID

		// Delete events first (since they reference the subordinate)
		recorder.DeleteForSubordinate(subordinateID)

		if err := subordinates.DeleteByDBID(id); err != nil {
			return writeServerError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleUpdateSubordinateStatus(
	subordinates model.SubordinateStorageBackend,
	recorder *eventRecorder,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("subordinateID")

		// Get existing info to record old status
		existing, err := subordinates.GetByDBID(id)
		if err != nil {
			return writeServerError(c, err)
		}
		if existing == nil {
			return writeNotFound(c, "subordinate not found")
		}
		oldStatus := existing.Status

		var status model.Status
		if err := c.BodyParser(&status); err != nil {
			return writeBadBody(c)
		}
		if !status.Valid() {
			return writeBadRequest(c, "invalid status")
		}
		// Check if setting to active - need to verify subordinate has keys
		if status == model.StatusActive {
			if !subordinateHasKeys(existing) {
				return writeBadRequest(c, "status cannot be active without keys")
			}
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

		// Record status update event
		recorder.Record(
			info.ID,
			model.EventTypeStatusUpdated,
			WithStatus(status),
			WithMessage(fmt.Sprintf("status changed from %s to %s", oldStatus, status)),
		)
		return c.JSON(info)
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

		return c.JSON(fiber.Map{
			"events": eventsResp,
			"pagination": fiber.Map{
				"total":  total,
				"limit":  limit,
				"offset": opts.Offset,
			},
		})
	}
}
