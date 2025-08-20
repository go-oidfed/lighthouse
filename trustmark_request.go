package lighthouse

import (
	"slices"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddTrustMarkRequestEndpoint adds an endpoint where entities can request to
// be entitled for a trust mark
func (fed *LightHouse) AddTrustMarkRequestEndpoint(
	endpoint EndpointConf,
	store storage.TrustMarkedEntitiesStorageBackend,
) {
	if fed.Metadata.FederationEntity.Extra == nil {
		fed.Metadata.FederationEntity.Extra = make(map[string]interface{})
	}
	fed.Metadata.FederationEntity.Extra["federation_trust_mark_request_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID)
	if endpoint.Path == "" {
		return
	}
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			trustMarkType := ctx.Query("trust_mark_type")
			sub := ctx.Query("sub")
			if sub == "" {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(
					oidfed.ErrorInvalidRequest(
						"required parameter 'sub' not given",
					),
				)
			}
			if trustMarkType == "" {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(
					oidfed.ErrorInvalidRequest(
						"required parameter 'trust_mark_type' not given",
					),
				)
			}
			if !slices.Contains(
				fed.TrustMarkIssuer.TrustMarkTypes(),
				trustMarkType,
			) {
				ctx.Status(fiber.StatusNotFound)
				return ctx.JSON(
					oidfed.ErrorNotFound("'trust_mark_type' not known"),
				)
			}

			status, err := store.TrustMarkedStatus(trustMarkType, sub)
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
				if err = store.Request(trustMarkType, sub); err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				ctx.Status(fiber.StatusAccepted)
				return nil
			}
		},
	)
}
