package lighthouse

import (
	"slices"

	"github.com/gofiber/fiber/v2"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddTrustMarkedEntitiesListingEndpoint adds a trust marked entities listing endpoint.
// Per OIDC Federation spec, this endpoint lists all entities for which trust marks
// have been issued and are still valid (non-revoked, non-expired).
func (fed *LightHouse) AddTrustMarkedEntitiesListingEndpoint(
	endpoint EndpointConf,
	instanceStore model.IssuedTrustMarkInstanceStore,
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
				// Check if specific entity has an active (valid) trust mark instance
				hasActive, err := instanceStore.HasActiveInstance(trustMarkType, sub)
				if err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				if hasActive {
					entities = []string{sub}
				}
			} else {
				// List all entities with active (valid) trust mark instances
				entities, err = instanceStore.ListActiveSubjects(trustMarkType)
				if err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				if entities == nil {
					entities = make([]string, 0)
				}
			}

			return ctx.JSON(entities)
		},
	)
}
