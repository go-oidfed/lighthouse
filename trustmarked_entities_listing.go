package lighthouse

import (
	"slices"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddTrustMarkedEntitiesListingEndpoint adds a trust marked entities endpoint
func (fed *LightHouse) AddTrustMarkedEntitiesListingEndpoint(
	endpoint EndpointConf,
	store model.TrustMarkedEntitiesStorageBackend,
) {
	fed.fedMetadata.FederationTrustMarkListEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return
	}
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			trustMarkType := ctx.Query("trust_mark_type")
			sub := ctx.Query("sub")
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
			entities := make([]string, 0)
			var err error
			if sub != "" {
				hasTM, err := store.HasTrustMark(trustMarkType, sub)
				if err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				if hasTM {
					entities = []string{sub}
				}
			} else {
				entities, err = store.Active(trustMarkType)
				if err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				if len(entities) == 0 {
					entities = make([]string, 0)
				}
			}
			return ctx.JSON(entities)
		},
	)
}
