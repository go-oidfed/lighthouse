package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerTrustMarkTypes wires handlers using a TrustMarkTypesStore abstraction.
func registerTrustMarkTypes(r fiber.Router, store model.TrustMarkTypesStore) {
	g := r.Group("/trust-marks/types")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	g.Get(
		"/", func(c *fiber.Ctx) error {
			items, err := store.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(items)
		},
	)

	withCacheWipe.Post(
		"/", func(c *fiber.Ctx) error {
			var req model.AddTrustMarkType
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
			}
			if req.TrustMarkType == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
			}
			item, err := store.Create(req)
			if err != nil {
				var alreadyExistsError model.AlreadyExistsError
				if errors.As(err, &alreadyExistsError) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust mark type already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(item)
		},
	)

	g.Get(
		"/:trustMarkTypeID", func(c *fiber.Ctx) error {
			item, err := store.Get(c.Params("trustMarkTypeID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
			}
			return c.JSON(item)
		},
	)

	withCacheWipe.Put(
		"/:trustMarkTypeID", func(c *fiber.Ctx) error {
			var req model.AddTrustMarkType
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.TrustMarkType == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
			}
			item, err := store.Update(c.Params("trustMarkTypeID"), req)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
				}
				var alreadyExistsError model.AlreadyExistsError
				if errors.As(err, &alreadyExistsError) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("trust mark type already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}

			// Optionally set or update owner
			if req.TrustMarkOwner != nil {
				if _, err := store.UpdateOwner(c.Params("trustMarkTypeID"), *req.TrustMarkOwner); err != nil {
					var nf model.NotFoundError
					if errors.As(err, &nf) {
						// If no current owner and request provides data to create a new one, create/link now.
						if req.TrustMarkOwner.OwnerID == nil {
							if _, err := store.CreateOwner(
								c.Params("trustMarkTypeID"), *req.TrustMarkOwner,
							); err != nil {
								return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
							}
						} else {
							// Referenced owner not found
							return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark owner not found"))
						}
					} else {
						return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
					}
				}
			}

			// Optionally replace issuers
			if len(req.TrustMarkIssuers) > 0 {
				if _, err := store.SetIssuers(c.Params("trustMarkTypeID"), req.TrustMarkIssuers); err != nil {
					var nf model.NotFoundError
					if errors.As(err, &nf) {
						return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
					}
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
			}
			return c.JSON(item)
		},
	)

	withCacheWipe.Delete(
		"/:trustMarkTypeID", func(c *fiber.Ctx) error {
			if err := store.Delete(c.Params("trustMarkTypeID")); err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Issuers
	r.Get(
		"/trust-marks/types/:trustMarkTypeID/issuers", func(c *fiber.Ctx) error {
			issuers, err := store.ListIssuers(c.Params("trustMarkTypeID"))
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(issuers)
		},
	)

	withCacheWipe.Put(
		"/trust-marks/types/:trustMarkTypeID/issuers", func(c *fiber.Ctx) error {
			var req []model.AddTrustMarkIssuer
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			issuers, err := store.SetIssuers(c.Params("trustMarkTypeID"), req)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(issuers)
		},
	)

	withCacheWipe.Post(
		"/trust-marks/types/:trustMarkTypeID/issuers",
		func(c *fiber.Ctx) error {
			var issuer model.AddTrustMarkIssuer
			if err := c.BodyParser(&issuer); err != nil || issuer.Issuer == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			issuers, err := store.AddIssuer(c.Params("trustMarkTypeID"), issuer)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(issuers)
		},
	)

	withCacheWipe.Delete(
		"/trust-marks/types/:trustMarkTypeID/issuers/:issuerID",
		func(c *fiber.Ctx) error {
			// Accept issuer identifier as numeric ID or issuer string
			var issuerID uint
			for _, ch := range c.Params("issuerID") {
				if ch < '0' || ch > '9' {
					issuerID = 0
					break
				}
				issuerID = issuerID*10 + uint(ch-'0')
			}
			// If not numeric, resolve via types storage helper by adding issuer relation then deleting
			if issuerID == 0 {
				// Attempt resolution via SetIssuers with a single issuer string to find ID through storage
				_, err := store.AddIssuer(
					c.Params("trustMarkTypeID"), model.AddTrustMarkIssuer{Issuer: c.Params("issuerID")},
				)
				if err != nil {
					var nf model.NotFoundError
					if errors.As(err, &nf) {
						return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
					}
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				// After ensuring exists, list issuers and find matching ID
				list, err := store.ListIssuers(c.Params("trustMarkTypeID"))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				for _, iss := range list {
					if iss.Issuer == c.Params("issuerID") {
						issuerID = iss.ID
						break
					}
				}
				if issuerID == 0 {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
				}
			}
			issuers, err := store.DeleteIssuerByID(c.Params("trustMarkTypeID"), issuerID)
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(issuers)
		},
	)

	// Owner
	r.Get(
		"/trust-marks/types/:trustMarkTypeID/owner", func(c *fiber.Ctx) error {
			owner, err := store.GetOwner(c.Params("trustMarkTypeID"))
			if err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark owner not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(owner)
		},
	)

	withCacheWipe.Put(
		"/trust-marks/types/:trustMarkTypeID/owner", func(c *fiber.Ctx) error {
			var req model.AddTrustMarkOwner
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.EntityID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
			}
			owner, err := store.UpdateOwner(c.Params("trustMarkTypeID"), req)
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
		},
	)

	withCacheWipe.Post(
		"/trust-marks/types/:trustMarkTypeID/owner",
		func(c *fiber.Ctx) error {
			var req model.AddTrustMarkOwner
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.EntityID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
			}
			owner, err := store.CreateOwner(c.Params("trustMarkTypeID"), req)
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
		},
	)

	withCacheWipe.Delete(
		"/trust-marks/types/:trustMarkTypeID/owner",
		func(c *fiber.Ctx) error {
			if err := store.DeleteOwner(c.Params("trustMarkTypeID")); err != nil {
				var notFoundError model.NotFoundError
				if errors.As(err, &notFoundError) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark owner not found"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)
}
