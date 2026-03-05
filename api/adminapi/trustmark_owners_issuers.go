package adminapi

import (
	"errors"
	"strconv"
	"strings"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// trustMarkOwnersHandlers groups handlers for trust mark owner endpoints.
type trustMarkOwnersHandlers struct {
	owners model.TrustMarkOwnersStore
	types  model.TrustMarkTypesStore
}

func (h *trustMarkOwnersHandlers) list(c *fiber.Ctx) error {
	list, err := h.owners.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(list)
}

func (h *trustMarkOwnersHandlers) create(c *fiber.Ctx) error {
	var req model.AddTrustMarkOwner
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.OwnerID != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("owner_id not allowed when creating"))
	}
	if req.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	item, err := h.owners.Create(req)
	if err != nil {
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("owner already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *trustMarkOwnersHandlers) get(c *fiber.Ctx) error {
	item, err := h.owners.Get(c.Params("ownerID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
	}
	return c.JSON(item)
}

func (h *trustMarkOwnersHandlers) update(c *fiber.Ctx) error {
	var req model.AddTrustMarkOwner
	if err := c.BodyParser(&req); err != nil || req.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	item, err := h.owners.Update(c.Params("ownerID"), req)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
		}
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("owner already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(item)
}

func (h *trustMarkOwnersHandlers) delete(c *fiber.Ctx) error {
	if err := h.owners.Delete(c.Params("ownerID")); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *trustMarkOwnersHandlers) listTypes(c *fiber.Ctx) error {
	ids, err := h.owners.Types(c.Params("ownerID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
	}
	typesOut, err := h.loadTrustMarkTypes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(typesOut)
}

func (h *trustMarkOwnersHandlers) setTypes(c *fiber.Ctx) error {
	var typeIdents []string
	if err := c.BodyParser(&typeIdents); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	ids, err := h.owners.SetTypes(c.Params("ownerID"), typeIdents)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	typesOut, err := h.loadTrustMarkTypes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(typesOut)
}

func (h *trustMarkOwnersHandlers) addType(c *fiber.Ctx) error {
	identInt, err := strconv.ParseUint(strings.TrimSpace(string(c.Body())), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body: expected integer"))
	}
	ids, err := h.owners.AddType(c.Params("ownerID"), uint(identInt))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	typesOut, err := h.loadTrustMarkTypes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(typesOut)
}

func (h *trustMarkOwnersHandlers) deleteType(c *fiber.Ctx) error {
	t, err := h.types.Get(c.Params("typeID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
	}
	ids, err := h.owners.DeleteType(c.Params("ownerID"), t.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(ids)
}

func (h *trustMarkOwnersHandlers) loadTrustMarkTypes(ids []uint) ([]model.TrustMarkType, error) {
	typesOut := make([]model.TrustMarkType, 0, len(ids))
	for _, id := range ids {
		item, err := h.types.Get(strconv.FormatUint(uint64(id), 10))
		if err != nil {
			return nil, err
		}
		typesOut = append(typesOut, *item)
	}
	return typesOut, nil
}

// globalTrustMarkIssuersHandlers groups handlers for global trust mark issuer endpoints.
type globalTrustMarkIssuersHandlers struct {
	issuers model.TrustMarkIssuersStore
	types   model.TrustMarkTypesStore
}

func (h *globalTrustMarkIssuersHandlers) list(c *fiber.Ctx) error {
	list, err := h.issuers.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(list)
}

func (h *globalTrustMarkIssuersHandlers) create(c *fiber.Ctx) error {
	var req model.AddTrustMarkIssuer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.IssuerID != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("issuer_id not allowed when creating"))
	}
	if req.Issuer == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("issuer is required"))
	}
	item, err := h.issuers.Create(model.AddTrustMarkIssuer{
		Issuer:      req.Issuer,
		Description: req.Description,
	})
	if err != nil {
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("issuer already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *globalTrustMarkIssuersHandlers) get(c *fiber.Ctx) error {
	item, err := h.issuers.Get(c.Params("issuerID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
	}
	return c.JSON(item)
}

func (h *globalTrustMarkIssuersHandlers) update(c *fiber.Ctx) error {
	var req model.AddTrustMarkIssuer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	item, err := h.issuers.Update(c.Params("issuerID"), req)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
		}
		var exists model.AlreadyExistsError
		if errors.As(err, &exists) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("issuer already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(item)
}

func (h *globalTrustMarkIssuersHandlers) delete(c *fiber.Ctx) error {
	if err := h.issuers.Delete(c.Params("issuerID")); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *globalTrustMarkIssuersHandlers) listTypes(c *fiber.Ctx) error {
	ids, err := h.issuers.Types(c.Params("issuerID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
	}
	typesOut, err := h.loadTrustMarkTypes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(typesOut)
}

func (h *globalTrustMarkIssuersHandlers) setTypes(c *fiber.Ctx) error {
	var typeIdents []string
	if err := c.BodyParser(&typeIdents); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	ids, err := h.issuers.SetTypes(c.Params("issuerID"), typeIdents)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	typesOut, err := h.loadTrustMarkTypes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(typesOut)
}

func (h *globalTrustMarkIssuersHandlers) addType(c *fiber.Ctx) error {
	identInt, err := strconv.ParseUint(strings.TrimSpace(string(c.Body())), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body: expected integer"))
	}
	ids, err := h.issuers.AddType(c.Params("issuerID"), uint(identInt))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	typesOut, err := h.loadTrustMarkTypes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(typesOut)
}

func (h *globalTrustMarkIssuersHandlers) deleteType(c *fiber.Ctx) error {
	t, err := h.types.Get(c.Params("typeID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
	}
	ids, err := h.issuers.DeleteType(c.Params("issuerID"), t.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(ids)
}

func (h *globalTrustMarkIssuersHandlers) loadTrustMarkTypes(ids []uint) ([]model.TrustMarkType, error) {
	typesOut := make([]model.TrustMarkType, 0, len(ids))
	for _, id := range ids {
		item, err := h.types.Get(strconv.FormatUint(uint64(id), 10))
		if err != nil {
			return nil, err
		}
		typesOut = append(typesOut, *item)
	}
	return typesOut, nil
}

// registerTrustMarkOwners registers trust mark owner endpoints.
func registerTrustMarkOwners(r fiber.Router, owners model.TrustMarkOwnersStore, types model.TrustMarkTypesStore) {
	g := r.Group("/trust-marks/owners")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	h := &trustMarkOwnersHandlers{owners: owners, types: types}

	g.Get("/", h.list)
	g.Post("/", h.create)
	g.Get("/:ownerID", h.get)
	g.Put("/:ownerID", h.update)
	withCacheWipe.Delete("/:ownerID", h.delete)

	g.Get("/:ownerID/types", h.listTypes)
	withCacheWipe.Put("/:ownerID/types", h.setTypes)
	withCacheWipe.Post("/:ownerID/types", h.addType)
	withCacheWipe.Delete("/:ownerID/types/:typeID", h.deleteType)
}

// registerTrustMarkIssuers registers trust mark issuer endpoints.
func registerTrustMarkIssuers(r fiber.Router, issuers model.TrustMarkIssuersStore, types model.TrustMarkTypesStore) {
	g := r.Group("/trust-marks/issuers")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	h := &globalTrustMarkIssuersHandlers{issuers: issuers, types: types}

	g.Get("/", h.list)
	g.Post("/", h.create)
	g.Get("/:issuerID", h.get)
	g.Put("/:issuerID", h.update)
	withCacheWipe.Delete("/:issuerID", h.delete)

	g.Get("/:issuerID/types", h.listTypes)
	withCacheWipe.Put("/:issuerID/types", h.setTypes)
	withCacheWipe.Post("/:issuerID/types", h.addType)
	withCacheWipe.Delete("/:issuerID/types/:typeID", h.deleteType)
}
