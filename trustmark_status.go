package lighthouse

import (
	"slices"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
)

// AddTrustMarkStatusEndpoint adds a trust mark status endpoint
func (fed *LightHouse) AddTrustMarkStatusEndpoint(
	endpoint EndpointConf,
	store storage.TrustMarkedEntitiesStorageBackend,
) {
	fed.Metadata.FederationEntity.FederationTrustMarkStatusEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID)
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

			hasTM, err := store.HasTrustMark(trustMarkType, sub)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			return ctx.JSON(
				map[string]any{
					"active": hasTM,
				},
			)
		},
	)
}
