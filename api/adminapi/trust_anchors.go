package adminapi

import (
	"errors"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// LighthouseController is the interface the admin API uses to interact with
// the running LightHouse instance for trust anchor and endpoint management.
// The lighthouse.LightHouse type implements this interface.
type LighthouseController interface {
	// TAJWKSRefresher returns the TA JWKS refresher (may be nil).
	TAJWKSRefresher() *oidfed.TAJWKSRefresher
	// SyncTrustAnchor reloads a TA from the database and updates the in-memory
	// repository. Called after a DB mutation on a trust anchor.
	SyncTrustAnchor(entityID string)
	// RemoveTrustAnchor removes a TA from the in-memory repository.
	RemoveTrustAnchor(entityID string)
	// ReloadEndpointsFromDB reloads all federation endpoints from the database
	// into the in-memory endpoint registry.
	ReloadEndpointsFromDB() error
}

// trustAnchorsHandlers groups handlers for trust anchor endpoints.
type trustAnchorsHandlers struct {
	store      model.TrustAnchorStore
	controller LighthouseController
}

func (h *trustAnchorsHandlers) list(c *fiber.Ctx) error {
	list, err := h.store.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(list)
}

func (h *trustAnchorsHandlers) get(c *fiber.Ctx) error {
	item, err := h.store.Get(c.Params("entityID"))
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust anchor not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(item)
}

func (h *trustAnchorsHandlers) create(c *fiber.Ctx) error {
	var req model.AddTrustAnchor
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	item, err := h.store.Create(req)
	if err != nil {
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust anchor already exists"))
		}
		var ve model.ValidationError
		if errors.As(err, &ve) {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.syncRepo(item.EntityID)
	h.syncRefresher(item.EntityID)
	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *trustAnchorsHandlers) update(c *fiber.Ctx) error {
	var req model.AddTrustAnchor
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.EntityID == "" {
		req.EntityID = c.Params("entityID")
	}
	item, err := h.store.Update(c.Params("entityID"), req)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust anchor not found"))
		}
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust anchor already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.syncRepo(item.EntityID)
	h.syncRefresher(item.EntityID)
	return c.JSON(item)
}

func (h *trustAnchorsHandlers) delete(c *fiber.Ctx) error {
	entityID := c.Params("entityID")
	// Stop refresher polling for this TA before deleting.
	if r := h.controller.TAJWKSRefresher(); r != nil {
		r.Remove(entityID)
	}
	if err := h.store.Delete(entityID); err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust anchor not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.controller.RemoveTrustAnchor(entityID)
	return c.SendStatus(fiber.StatusNoContent)
}

// syncRepo adds or updates the TA in the in-memory repo after a DB mutation.
func (h *trustAnchorsHandlers) syncRepo(entityID string) {
	h.controller.SyncTrustAnchor(entityID)
}

// syncRefresher dynamically adds/updates the TA in the refresher if it has
// EnableJWKSUpdate=true and the refresher is running.
func (h *trustAnchorsHandlers) syncRefresher(entityID string) {
	r := h.controller.TAJWKSRefresher()
	if r == nil || !r.IsStarted() {
		return
	}
	// Load the (possibly updated) TA from the store and add it to the refresher.
	ta, err := h.store.Get(entityID)
	if err != nil {
		log.WithError(err).WithField("entity_id", entityID).
			Warn("failed to load TA for refresher sync")
		return
	}
	if !ta.EnableJWKSUpdate {
		// If JWKS update was disabled, remove from refresher.
		r.Remove(entityID)
		return
	}
	// Build an oidfed.TrustAnchor from the DB row.
	oidfedTA := &oidfed.TrustAnchor{
		EntityID:         ta.EntityID,
		EnableJWKSUpdate: ta.EnableJWKSUpdate,
		KeyPollInterval:  duration.DurationOption(time.Duration(ta.KeyPollInterval) * time.Second),
	}
	if ta.JWKSID != nil && ta.JWKS.Keys.Set != nil {
		oidfedTA.SetJWKS(ta.JWKS.Keys)
	}
	if err := r.Update(oidfedTA); err != nil {
		log.WithError(err).WithField("entity_id", entityID).
			Warn("failed to add/update TA in refresher")
	}
}

// registerTrustAnchors registers trust anchor management endpoints.
func registerTrustAnchors(r fiber.Router, store model.TrustAnchorStore, ctrl LighthouseController) {
	if store == nil || ctrl == nil {
		return
	}
	g := r.Group("/trust-anchors")
	h := &trustAnchorsHandlers{store: store, controller: ctrl}

	g.Get("/", h.list)
	g.Post("/", h.create)
	g.Get("/:entityID", h.get)
	g.Put("/:entityID", h.update)
	g.Delete("/:entityID", h.delete)
}
