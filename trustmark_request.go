package lighthouse

import (
	"slices"

	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddTrustMarkRequestEndpoint adds an endpoint where entities can request to
// be entitled for a trust mark
func (fed *LightHouse) AddTrustMarkRequestEndpoint(
	endpoint EndpointConf,
	store model.TrustMarkedEntitiesStorageBackend,
) {
	if fed.fedMetadata.Extra == nil {
		fed.fedMetadata.Extra = make(map[string]interface{})
	}
	fed.fedMetadata.Extra["federation_trust_mark_request_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return
	}
	handler := func(ctx *fiber.Ctx) error {
		var req trustMarkQueryRequest
		if err := parseRequest(ctx, &req); err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
		}
		if req.Subject == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(
				oidfed.ErrorInvalidRequest(
					"required parameter 'sub' not given",
				),
			)
		}
		if req.TrustMarkType == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(
				oidfed.ErrorInvalidRequest(
					"required parameter 'trust_mark_type' not given",
				),
			)
		}
		if !slices.Contains(
			fed.TrustMarkIssuer.TrustMarkTypes(),
			req.TrustMarkType,
		) {
			ctx.Status(fiber.StatusNotFound)
			return ctx.JSON(
				oidfed.ErrorNotFound("'trust_mark_type' not known"),
			)
		}

		status, err := store.TrustMarkedStatus(req.TrustMarkType, req.Subject)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		switch status {
		case model.StatusActive:
			ctx.Status(fiber.StatusNoContent)
			return nil
		case model.StatusBlocked:
			ctx.Status(fiber.StatusForbidden)
			return ctx.JSON(oidfed.ErrorInvalidRequest("subject cannot obtain this trust mark"))
		case model.StatusPending:
			ctx.Status(fiber.StatusAccepted)
			return nil
		case model.StatusInactive:
			fallthrough
		default:
			if err = store.Request(req.TrustMarkType, req.Subject); err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			ctx.Status(fiber.StatusAccepted)
			return nil
		}
	}

	if endpoint.AuthEnabled {
		fed.server.Post(endpoint.Path, handler)
		if fed.fedMetadata.Extra == nil {
			fed.fedMetadata.Extra = make(map[string]interface{})
		}
		fed.fedMetadata.Extra["federation_trust_mark_request_endpoint_auth_methods"] = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.server.Get(endpoint.Path, handler)
	}
}
