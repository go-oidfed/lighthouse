package adminapi

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

const (
	defaultEventsLimit          = 50
	maxEventsLimit              = 10000
	defaultRemovedEntitiesLimit = 50
	maxRemovedEntitiesLimit     = 500
)

// eventsHandlers serves the events query across all entities.
type eventsHandlers struct {
	events model.SubordinateEventStore
	// history supplies the query parsing shared with the per-subordinate route.
	history historyHandlers
}

// registerEvents registers the events query routes.
func registerEvents(r fiber.Router, events model.SubordinateEventStore) {
	h := &eventsHandlers{
		events:  events,
		history: historyHandlers{events: events},
	}
	g := r.Group("/events")
	g.Get("/", h.list)
	g.Get("/removed-entities", h.removedEntities)
}

// list returns events of all entities, newest first, filtered by entity_id,
// actor, from/to (unix seconds) and any of the repeated type parameters.
func (h *eventsHandlers) list(c fiber.Ctx) error {
	opts, ok := h.history.parseQueryOpts(c)
	if !ok {
		return nil
	}
	if opts.Limit < 0 {
		return writeBadRequest(c, "invalid limit parameter")
	}
	if opts.Offset < 0 {
		return writeBadRequest(c, "invalid offset parameter")
	}
	// Repeated type parameters replace the single type of the shared parsing.
	opts.EventType = nil
	for _, t := range c.RequestCtx().QueryArgs().PeekMulti("type") {
		if len(t) > 0 {
			opts.EventTypes = append(opts.EventTypes, string(t))
		}
	}
	if entityID := c.Query("entity_id"); entityID != "" {
		opts.EntityID = &entityID
	}
	if actor := c.Query("actor"); actor != "" {
		opts.Actor = &actor
	}
	if opts.Limit == 0 {
		opts.Limit = defaultEventsLimit
	}
	opts.Limit = min(opts.Limit, maxEventsLimit)

	list, total, err := h.events.Query(opts)
	if err != nil {
		return writeServerError(c, err)
	}
	resp := make([]eventResponse, len(list))
	for i, e := range list {
		resp[i] = newEventResponse(e)
	}
	return c.JSON(
		fiber.Map{
			"events": resp,
			"pagination": fiber.Map{
				"total":  total,
				"limit":  opts.Limit,
				"offset": opts.Offset,
			},
		},
	)
}

// removedEntities lists, sorted, the entity IDs that have events but are not
// a current subordinate and contain q.
func (h *eventsHandlers) removedEntities(c fiber.Ctx) error {
	limit := defaultRemovedEntitiesLimit
	if s := c.Query("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return writeBadRequest(c, "invalid limit parameter")
		}
		if n > 0 {
			limit = min(n, maxRemovedEntitiesLimit)
		}
	}
	ids, err := h.events.RemovedEntityIDs(c.Query("q"), limit)
	if err != nil {
		return writeServerError(c, err)
	}
	if ids == nil {
		ids = []string{}
	}
	return c.JSON(fiber.Map{"entity_ids": ids})
}
