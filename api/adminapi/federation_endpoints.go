package adminapi

import (
	"encoding/json"
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// federationEndpointsHandlers groups handlers for federation endpoint management.
type federationEndpointsHandlers struct {
	store      model.FederationEndpointStore
	controller LighthouseController
}

func (h *federationEndpointsHandlers) list(c *fiber.Ctx) error {
	list, err := h.store.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(list)
}

func (h *federationEndpointsHandlers) getByType(c *fiber.Ctx) error {
	t := model.FederationEndpointType(c.Params("type"))
	if !model.IsValidFederationEndpointType(t) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid endpoint type"))
	}
	item, err := h.store.GetByType(t)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("federation endpoint not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(item)
}

func (h *federationEndpointsHandlers) create(c *fiber.Ctx) error {
	var req model.AddFederationEndpoint
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if !model.IsValidFederationEndpointType(req.Type) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid endpoint type"))
	}
	// Validate the config JSON blob if provided.
	if req.Config != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(req.Config), &raw); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				oidfed.ErrorInvalidRequest("config must be valid JSON: " + err.Error()),
			)
		}
	}
	item, err := h.store.Create(req)
	if err != nil {
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("federation endpoint already exists"))
		}
		var ve model.ValidationError
		if errors.As(err, &ve) {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.reloadEndpoints()
	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *federationEndpointsHandlers) update(c *fiber.Ctx) error {
	t := model.FederationEndpointType(c.Params("type"))
	if !model.IsValidFederationEndpointType(t) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid endpoint type"))
	}
	var req model.AddFederationEndpoint
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.Type == "" {
		req.Type = t
	}
	// Validate the config JSON blob if provided.
	if req.Config != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(req.Config), &raw); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				oidfed.ErrorInvalidRequest("config must be valid JSON: " + err.Error()),
			)
		}
	}
	item, err := h.store.Update(t, req)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("federation endpoint not found"))
		}
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("federation endpoint already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.reloadEndpoints()
	return c.JSON(item)
}

func (h *federationEndpointsHandlers) delete(c *fiber.Ctx) error {
	t := model.FederationEndpointType(c.Params("type"))
	if !model.IsValidFederationEndpointType(t) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid endpoint type"))
	}
	if err := h.store.Delete(t); err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("federation endpoint not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.reloadEndpoints()
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *federationEndpointsHandlers) setAuthTrustAnchors(c *fiber.Ctx) error {
	t := model.FederationEndpointType(c.Params("type"))
	if !model.IsValidFederationEndpointType(t) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid endpoint type"))
	}
	var ids []uint
	if err := c.BodyParser(&ids); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body: expected array of trust anchor IDs"))
	}
	tas, err := h.store.SetAuthTrustAnchors(t, ids)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.reloadEndpoints()
	return c.JSON(tas)
}

// reloadEndpoints reloads all federation endpoints from the DB into the
// in-memory registry and invalidates the entity configuration cache.
func (h *federationEndpointsHandlers) reloadEndpoints() {
	if h.controller == nil {
		return
	}
	if err := h.controller.ReloadEndpointsFromDB(); err != nil {
		log.Error().Err(err).Msg("failed to reload endpoints from DB after admin API mutation")
	}
}

// registerFederationEndpoints registers federation endpoint management endpoints.
func registerFederationEndpoints(r fiber.Router, store model.FederationEndpointStore, ctrl LighthouseController) {
	if store == nil || ctrl == nil {
		return
	}
	g := r.Group("/federation-endpoints")
	h := &federationEndpointsHandlers{store: store, controller: ctrl}

	g.Get("/", h.list)
	g.Post("/", h.create)
	g.Get("/:type", h.getByType)
	g.Put("/:type", h.update)
	g.Delete("/:type", h.delete)
	g.Put("/:type/auth-trust-anchors", h.setAuthTrustAnchors)
}
