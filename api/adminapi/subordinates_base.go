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

// subordinatesBaseHandlers groups handlers for basic subordinate CRUD operations.
type subordinatesBaseHandlers struct {
	storages model.Backends
}

type listSubordinatesRequest struct {
	Status     *model.Status `query:"-"`
	EntityType []string      `query:"entity_type"`
}

func (h *subordinatesBaseHandlers) list(c *fiber.Ctx) error {
	var req listSubordinatesRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}

	if s := c.Query("status"); s != "" {
		st, err := model.ParseStatus(s)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				oidfed.ErrorInvalidRequest(fmt.Sprintf("invalid status: %s", err.Error())),
			)
		}
		req.Status = &st
	}

	var infos []model.BasicSubordinateInfo
	var err error

	switch {
	case req.EntityType != nil && req.Status != nil:
		infos, err = h.storages.Subordinates.GetByStatusAndAnyEntityType(*req.Status, req.EntityType)
	case req.EntityType != nil:
		infos, err = h.storages.Subordinates.GetByAnyEntityType(req.EntityType)
	case req.Status != nil:
		infos, err = h.storages.Subordinates.GetByStatus(*req.Status)
	default:
		infos, err = h.storages.Subordinates.GetAll()
	}

	if err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(infos)
}

func (h *subordinatesBaseHandlers) create(c *fiber.Ctx) error {
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
	if req.Status == model.StatusActive && !jwksHasKeys(&req.JWKS) {
		return writeBadRequest(c, "status cannot be active without keys")
	}

	var stored *model.ExtendedSubordinateInfo
	err := h.storages.InTransaction(func(tx *model.Backends) error {
		if err := tx.Subordinates.Add(req); err != nil {
			return err
		}
		var err error
		stored, err = tx.Subordinates.Get(req.EntityID)
		if err != nil {
			return err
		}
		return RecordEvent(
			tx.SubordinateEvents,
			stored.ID,
			model.EventTypeCreated,
			WithStatus(stored.Status),
			WithMessage(fmt.Sprintf("subordinate created: %s", stored.EntityID)),
			WithActor(GetActor(c)),
		)
	})
	if err != nil {
		return writeServerError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(stored)
}

func (h *subordinatesBaseHandlers) get(c *fiber.Ctx) error {
	id := c.Params("subordinateID")
	info, err := h.storages.Subordinates.GetByDBID(id)
	if err != nil {
		return writeServerError(c, err)
	}
	if info == nil {
		return writeNotFound(c, "subordinate not found")
	}
	return c.JSON(*info)
}

type updateSubordinateRequest struct {
	Description           *string  `json:"description"`
	RegisteredEntityTypes []string `json:"registered_entity_types"`
}

func (h *subordinatesBaseHandlers) update(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	var body updateSubordinateRequest
	if err := c.BodyParser(&body); err != nil {
		return writeBadBody(c)
	}

	var result *model.ExtendedSubordinateInfo
	err := h.storages.InTransaction(func(tx *model.Backends) error {
		existing, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
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
		if err = RecordEvent(tx.SubordinateEvents, existing.ID, model.EventTypeUpdated, WithStatus(existing.Status), WithActor(GetActor(c))); err != nil {
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

func (h *subordinatesBaseHandlers) delete(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	err := h.storages.InTransaction(func(tx *model.Backends) error {
		existing, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
		}
		if err := tx.SubordinateEvents.DeleteBySubordinateID(existing.ID); err != nil {
			return err
		}
		return tx.Subordinates.DeleteByDBID(id)
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

func (h *subordinatesBaseHandlers) updateStatus(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	statusStr := strings.TrimSpace(string(c.Body()))
	if statusStr == "" {
		return writeBadRequest(c, "status is required")
	}
	status, err := model.ParseStatus(statusStr)
	if err != nil {
		return writeBadRequest(c, err.Error())
	}

	var result *model.ExtendedSubordinateInfo
	err = h.storages.InTransaction(func(tx *model.Backends) error {
		existing, err := getSubordinateByDBID(tx.Subordinates, id)
		if err != nil {
			return err
		}
		oldStatus := existing.Status

		if status == model.StatusActive && !subordinateHasKeys(existing) {
			return fmt.Errorf("status cannot be active without keys")
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

		if err = RecordEvent(
			tx.SubordinateEvents,
			info.ID,
			model.EventTypeStatusUpdated,
			WithStatus(status),
			WithMessage(fmt.Sprintf("status changed from %s to %s", oldStatus, status)),
			WithActor(GetActor(c)),
		); err != nil {
			return err
		}
		result = info
		return nil
	})

	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return writeNotFound(c, err.Error())
		}
		if err.Error() == "status cannot be active without keys" {
			return writeBadRequest(c, err.Error())
		}
		return writeServerError(c, err)
	}
	return c.JSON(result)
}

// historyHandlers groups handlers for subordinate history endpoints.
type historyHandlers struct {
	subordinates model.SubordinateStorageBackend
	events       model.SubordinateEventStore
}

type eventResponse struct {
	Timestamp int64   `json:"timestamp"`
	Type      string  `json:"type"`
	Status    *string `json:"status,omitempty"`
	Message   *string `json:"message,omitempty"`
	Actor     *string `json:"actor,omitempty"`
}

func (h *historyHandlers) getHistory(c *fiber.Ctx) error {
	id := c.Params("subordinateID")

	info, err := h.subordinates.GetByDBID(id)
	if err != nil {
		return writeServerError(c, err)
	}
	if info == nil {
		return writeNotFound(c, "subordinate not found")
	}

	opts, ok := h.parseQueryOpts(c)
	if !ok {
		return nil
	}

	eventsList, total, err := h.events.GetBySubordinateID(info.ID, opts)
	if err != nil {
		return writeServerError(c, err)
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

	limit := h.normalizeLimit(opts.Limit)

	return c.JSON(fiber.Map{
		"events": eventsResp,
		"pagination": fiber.Map{
			"total":  total,
			"limit":  limit,
			"offset": opts.Offset,
		},
	})
}

// parseQueryOpts parses query parameters for event history requests.
// Returns (opts, true) on success, or (zero, false) if an error response was written.
func (*historyHandlers) parseQueryOpts(c *fiber.Ctx) (model.EventQueryOpts, bool) {
	var opts model.EventQueryOpts

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			_ = writeBadRequest(c, "invalid limit parameter")
			return opts, false
		}
		opts.Limit = limit
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			_ = writeBadRequest(c, "invalid offset parameter")
			return opts, false
		}
		opts.Offset = offset
	}

	if eventType := c.Query("type"); eventType != "" {
		opts.EventType = &eventType
	}

	if fromStr := c.Query("from"); fromStr != "" {
		from, err := strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			_ = writeBadRequest(c, "invalid from parameter")
			return opts, false
		}
		opts.FromTime = &from
	}

	if toStr := c.Query("to"); toStr != "" {
		to, err := strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			_ = writeBadRequest(c, "invalid to parameter")
			return opts, false
		}
		opts.ToTime = &to
	}

	return opts, true
}

func (*historyHandlers) normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// registerSubordinatesBase registers basic CRUD endpoints for subordinates.
func registerSubordinatesBase(r fiber.Router, storages model.Backends) {
	g := r.Group("/subordinates")
	withCacheWipe := g.Use(subordinateStatementsCacheInvalidationMiddleware)

	baseH := &subordinatesBaseHandlers{storages: storages}
	historyH := &historyHandlers{
		subordinates: storages.Subordinates,
		events:       storages.SubordinateEvents,
	}

	g.Get("/", baseH.list)
	g.Post("/", baseH.create)
	g.Get("/:subordinateID", baseH.get)
	withCacheWipe.Put("/:subordinateID", baseH.update)
	withCacheWipe.Delete("/:subordinateID", baseH.delete)
	withCacheWipe.Put("/:subordinateID/status", baseH.updateStatus)
	g.Get("/:subordinateID/history", historyH.getHistory)
}
