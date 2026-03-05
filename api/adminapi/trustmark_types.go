package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// trustMarkTypesHandlers groups handlers for trust mark type endpoints.
type trustMarkTypesHandlers struct {
	storages model.Backends
}

func (h *trustMarkTypesHandlers) store() model.TrustMarkTypesStore {
	return h.storages.TrustMarkTypes
}

func (h *trustMarkTypesHandlers) list(c *fiber.Ctx) error {
	items, err := h.store().List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(items)
}

func (h *trustMarkTypesHandlers) create(c *fiber.Ctx) error {
	var req model.AddTrustMarkType
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if req.TrustMarkType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
	}
	item, err := h.store().Create(req)
	if err != nil {
		var alreadyExistsError model.AlreadyExistsError
		if errors.As(err, &alreadyExistsError) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust mark type already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *trustMarkTypesHandlers) get(c *fiber.Ctx) error {
	item, err := h.store().Get(c.Params("trustMarkTypeID"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
	}
	return c.JSON(item)
}

func (h *trustMarkTypesHandlers) update(c *fiber.Ctx) error {
	trustMarkTypeID := c.Params("trustMarkTypeID")
	var req model.AddTrustMarkType
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.TrustMarkType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
	}

	var item *model.TrustMarkType
	err := h.storages.InTransaction(func(tx *model.Backends) error {
		txStore := tx.TrustMarkTypes
		var err error
		item, err = txStore.Update(trustMarkTypeID, req)
		if err != nil {
			return err
		}

		if req.TrustMarkOwner != nil {
			if err := h.updateOwnerInTx(txStore, trustMarkTypeID, req); err != nil {
				return err
			}
		}

		if len(req.TrustMarkIssuers) > 0 {
			if _, err := txStore.SetIssuers(trustMarkTypeID, req.TrustMarkIssuers); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return h.handleTxError(c, err)
	}
	return c.JSON(item)
}

func (*trustMarkTypesHandlers) updateOwnerInTx(txStore model.TrustMarkTypesStore, trustMarkTypeID string, req model.AddTrustMarkType) error {
	if _, err := txStore.UpdateOwner(trustMarkTypeID, *req.TrustMarkOwner); err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			if req.TrustMarkOwner.OwnerID == nil {
				if _, err := txStore.CreateOwner(trustMarkTypeID, *req.TrustMarkOwner); err != nil {
					return err
				}
				return nil
			}
			return model.NotFoundError("trust mark owner not found")
		}
		return err
	}
	return nil
}

func (h *trustMarkTypesHandlers) delete(c *fiber.Ctx) error {
	if err := h.store().Delete(c.Params("trustMarkTypeID")); err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (*trustMarkTypesHandlers) handleTxError(c *fiber.Ctx, err error) error {
	var notFoundError model.NotFoundError
	if errors.As(err, &notFoundError) {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(err.Error()))
	}
	var alreadyExistsError model.AlreadyExistsError
	if errors.As(err, &alreadyExistsError) {
		return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust mark type already exists"))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
}

// trustMarkIssuersHandlers groups handlers for trust mark issuers endpoints.
type trustMarkIssuersHandlers struct {
	store model.TrustMarkTypesStore
}

func (h *trustMarkIssuersHandlers) list(c *fiber.Ctx) error {
	issuers, err := h.store.ListIssuers(c.Params("trustMarkTypeID"))
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(issuers)
}

func (h *trustMarkIssuersHandlers) set(c *fiber.Ctx) error {
	var req []model.AddTrustMarkIssuer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	issuers, err := h.store.SetIssuers(c.Params("trustMarkTypeID"), req)
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(issuers)
}

func (h *trustMarkIssuersHandlers) add(c *fiber.Ctx) error {
	var issuer model.AddTrustMarkIssuer
	if err := c.BodyParser(&issuer); err != nil || (issuer.Issuer == "" && issuer.IssuerID == nil) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	issuers, err := h.store.AddIssuer(c.Params("trustMarkTypeID"), issuer)
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(issuers)
}

func (h *trustMarkIssuersHandlers) delete(c *fiber.Ctx) error {
	trustMarkTypeID := c.Params("trustMarkTypeID")
	issuerIDParam := c.Params("issuerID")

	issuerID, err := h.resolveIssuerID(trustMarkTypeID, issuerIDParam)
	if err != nil {
		var nf model.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if issuerID == 0 {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
	}

	issuers, err := h.store.DeleteIssuerByID(trustMarkTypeID, issuerID)
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(issuers)
}

func (h *trustMarkIssuersHandlers) resolveIssuerID(trustMarkTypeID, issuerIDParam string) (uint, error) {
	// Try parsing as numeric ID
	var issuerID uint
	isNumeric := true
	for _, ch := range issuerIDParam {
		if ch < '0' || ch > '9' {
			isNumeric = false
			break
		}
		issuerID = issuerID*10 + uint(ch-'0')
	}
	if isNumeric && issuerID > 0 {
		return issuerID, nil
	}

	// Not numeric - resolve by issuer string
	if _, err := h.store.AddIssuer(trustMarkTypeID, model.AddTrustMarkIssuer{Issuer: issuerIDParam}); err != nil {
		return 0, err
	}

	list, err := h.store.ListIssuers(trustMarkTypeID)
	if err != nil {
		return 0, err
	}
	for _, iss := range list {
		if iss.Issuer == issuerIDParam {
			return iss.ID, nil
		}
	}
	return 0, nil
}

// trustMarkOwnerHandlers groups handlers for trust mark owner endpoints.
type trustMarkOwnerHandlers struct {
	store model.TrustMarkTypesStore
}

func (h *trustMarkOwnerHandlers) get(c *fiber.Ctx) error {
	owner, err := h.store.GetOwner(c.Params("trustMarkTypeID"))
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark owner not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(owner)
}

func (h *trustMarkOwnerHandlers) update(c *fiber.Ctx) error {
	var req model.AddTrustMarkOwner
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.EntityID == "" && req.OwnerID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id or owner_id is required"))
	}
	owner, err := h.store.UpdateOwner(c.Params("trustMarkTypeID"), req)
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark owner not found"))
		}
		var alreadyExistsError model.AlreadyExistsError
		if errors.As(err, &alreadyExistsError) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust mark owner already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(owner)
}

func (h *trustMarkOwnerHandlers) create(c *fiber.Ctx) error {
	var req model.AddTrustMarkOwner
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	owner, err := h.store.CreateOwner(c.Params("trustMarkTypeID"), req)
	if err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
		}
		var alreadyExistsError model.AlreadyExistsError
		if errors.As(err, &alreadyExistsError) {
			return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust mark owner already exists"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(owner)
}

func (h *trustMarkOwnerHandlers) delete(c *fiber.Ctx) error {
	if err := h.store.DeleteOwner(c.Params("trustMarkTypeID")); err != nil {
		var notFoundError model.NotFoundError
		if errors.As(err, &notFoundError) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark owner not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// registerTrustMarkTypes wires handlers for trust mark types, issuers, and owners.
func registerTrustMarkTypes(r fiber.Router, storages model.Backends) {
	store := storages.TrustMarkTypes
	g := r.Group("/trust-marks/types")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	typesH := &trustMarkTypesHandlers{storages: storages}
	issuersH := &trustMarkIssuersHandlers{store: store}
	ownerH := &trustMarkOwnerHandlers{store: store}

	// Trust mark types
	g.Get("/", typesH.list)
	withCacheWipe.Post("/", typesH.create)
	g.Get("/:trustMarkTypeID", typesH.get)
	withCacheWipe.Put("/:trustMarkTypeID", typesH.update)
	withCacheWipe.Delete("/:trustMarkTypeID", typesH.delete)

	// Issuers
	g.Get("/:trustMarkTypeID/issuers", issuersH.list)
	withCacheWipe.Put("/:trustMarkTypeID/issuers", issuersH.set)
	withCacheWipe.Post("/:trustMarkTypeID/issuers", issuersH.add)
	withCacheWipe.Delete("/:trustMarkTypeID/issuers/:issuerID", issuersH.delete)

	// Owner
	g.Get("/:trustMarkTypeID/owner", ownerH.get)
	withCacheWipe.Put("/:trustMarkTypeID/owner", ownerH.update)
	withCacheWipe.Post("/:trustMarkTypeID/owner", ownerH.create)
	withCacheWipe.Delete("/:trustMarkTypeID/owner", ownerH.delete)
}
