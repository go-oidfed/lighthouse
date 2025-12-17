package adminapi

import (
	"errors"
	"strconv"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// Owners: list and manage global owners and link to types
func registerTrustMarkOwners(r fiber.Router, owners model.TrustMarkOwnersStore, types model.TrustMarkTypesStore) {
	g := r.Group("/trust-marks/owners")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)
	g.Get(
		"/", func(c *fiber.Ctx) error {
			list, err := owners.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(list)
		},
	)
	g.Post(
		"/", func(c *fiber.Ctx) error {
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
			item, err := owners.Create(req)
			if err != nil {
				var exists model.AlreadyExistsError
				if errors.As(err, &exists) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("owner already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(item)
		},
	)
	g.Get(
		"/:ownerID", func(c *fiber.Ctx) error {
			item, err := owners.Get(c.Params("ownerID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
			}
			return c.JSON(item)
		},
	)
	g.Put(
		"/:ownerID", func(c *fiber.Ctx) error {
			var req model.AddTrustMarkOwner
			if err := c.BodyParser(&req); err != nil || req.EntityID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			item, err := owners.Update(c.Params("ownerID"), req)
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
		},
	)
	withCacheWipe.Delete(
		"/:ownerID", func(c *fiber.Ctx) error {
			if err := owners.Delete(c.Params("ownerID")); err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Owner relations to types
	g.Get(
		"/:ownerID/types", func(c *fiber.Ctx) error {
			ids, err := owners.Types(c.Params("ownerID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("owner not found"))
			}
			// Load full TrustMarkType objects
			typesOut := make([]model.TrustMarkType, 0, len(ids))
			for _, id := range ids {
				item, err := types.Get(strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				typesOut = append(typesOut, *item)
			}
			return c.JSON(typesOut)
		},
	)
	withCacheWipe.Put(
		"/:ownerID/types", func(c *fiber.Ctx) error {
			// Accept list of type identifiers (numeric IDs or trust_mark_type strings)
			var typeIdents []string
			if err := c.BodyParser(&typeIdents); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			ids, err := owners.SetTypes(c.Params("ownerID"), typeIdents)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			// Return full objects
			typesOut := make([]model.TrustMarkType, 0, len(ids))
			for _, id := range ids {
				item, err := types.Get(strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				typesOut = append(typesOut, *item)
			}
			return c.JSON(typesOut)
		},
	)
	withCacheWipe.Post(
		"/:ownerID/types", func(c *fiber.Ctx) error {
			var ident uint
			if err := c.BodyParser(&ident); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			ids, err := owners.AddType(c.Params("ownerID"), ident)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			typesOut := make([]model.TrustMarkType, 0, len(ids))
			for _, id := range ids {
				item, err := types.Get(strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				typesOut = append(typesOut, *item)
			}
			return c.Status(fiber.StatusCreated).JSON(typesOut)
		},
	)
	withCacheWipe.Delete(
		"/:ownerID/types/:typeID", func(c *fiber.Ctx) error {
			t, err := types.Get(c.Params("typeID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
			}
			ids, err := owners.DeleteType(c.Params("ownerID"), t.ID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(ids)
		},
	)
}

// Issuers: list and manage global issuers and link to types
func registerTrustMarkIssuers(r fiber.Router, issuers model.TrustMarkIssuersStore, types model.TrustMarkTypesStore) {
	g := r.Group("/trust-marks/issuers")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)
	g.Get(
		"/", func(c *fiber.Ctx) error {
			list, err := issuers.List()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(list)
		},
	)
	g.Post(
		"/", func(c *fiber.Ctx) error {
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
			item, err := issuers.Create(
				model.AddTrustMarkIssuer{
					Issuer:      req.Issuer,
					Description: req.Description,
				},
			)
			if err != nil {
				var exists model.AlreadyExistsError
				if errors.As(err, &exists) {
					return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("issuer already exists"))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(item)
		},
	)
	g.Get(
		"/:issuerID", func(c *fiber.Ctx) error {
			item, err := issuers.Get(c.Params("issuerID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
			}
			return c.JSON(item)
		},
	)
	g.Put(
		"/:issuerID", func(c *fiber.Ctx) error {
			var req model.AddTrustMarkIssuer
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			item, err := issuers.Update(c.Params("issuerID"), req)
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
		},
	)
	withCacheWipe.Delete(
		"/:issuerID", func(c *fiber.Ctx) error {
			if err := issuers.Delete(c.Params("issuerID")); err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
			}
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// Issuer relations to types
	g.Get(
		"/:issuerID/types", func(c *fiber.Ctx) error {
			ids, err := issuers.Types(c.Params("issuerID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("issuer not found"))
			}
			// Load full TrustMarkType objects
			typesOut := make([]model.TrustMarkType, 0, len(ids))
			for _, id := range ids {
				item, err := types.Get(strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				typesOut = append(typesOut, *item)
			}
			return c.JSON(typesOut)
		},
	)
	withCacheWipe.Put(
		"/:issuerID/types", func(c *fiber.Ctx) error {
			var typeIdents []string
			if err := c.BodyParser(&typeIdents); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			ids, err := issuers.SetTypes(c.Params("issuerID"), typeIdents)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			// Return full objects
			typesOut := make([]model.TrustMarkType, 0, len(ids))
			for _, id := range ids {
				item, err := types.Get(strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				typesOut = append(typesOut, *item)
			}
			return c.JSON(typesOut)
		},
	)
	withCacheWipe.Post(
		"/:issuerID/types", func(c *fiber.Ctx) error {
			var ident uint
			if err := c.BodyParser(&ident); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			ids, err := issuers.AddType(c.Params("issuerID"), ident)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			typesOut := make([]model.TrustMarkType, 0, len(ids))
			for _, id := range ids {
				item, err := types.Get(strconv.FormatUint(uint64(id), 10))
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
				}
				typesOut = append(typesOut, *item)
			}
			return c.Status(fiber.StatusCreated).JSON(typesOut)
		},
	)
	withCacheWipe.Delete(
		"/:issuerID/types/:typeID", func(c *fiber.Ctx) error {
			t, err := types.Get(c.Params("typeID"))
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("trust mark type not found"))
			}
			ids, err := issuers.DeleteType(c.Params("issuerID"), t.ID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(ids)
		},
	)
}
