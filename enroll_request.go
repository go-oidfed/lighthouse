package lighthouse

import (
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddEnrollRequestEndpoint adds an endpoint to request enrollment to this IA
// /TA (this does only add a request to the storage, no automatic enrollment)
func (fed *LightHouse) AddEnrollRequestEndpoint(
	endpoint EndpointConf,
	store storage.SubordinateStorageBackend,
) {
	if fed.Metadata.FederationEntity.Extra == nil {
		fed.Metadata.FederationEntity.Extra = make(map[string]interface{})
	}
	fed.Metadata.FederationEntity.Extra["federation_enroll_request_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID)
	if endpoint.Path == "" {
		return
	}
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			var req enrollRequest
			if err := ctx.QueryParser(&req); err != nil {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
			}
			if req.Subject == "" {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
			}
			storedInfo, err := store.Subordinate(req.Subject)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			if storedInfo != nil { // Already a subordinate
				switch storedInfo.Status {
				case model.StatusActive:
					ctx.Status(fiber.StatusNoContent)
					return nil
				case model.StatusBlocked:
					ctx.Status(fiber.StatusForbidden)
					return ctx.JSON(
						oidfed.ErrorInvalidRequest(
							"the entity cannot enroll",
						),
					)
				case model.StatusPending:
					ctx.Status(fiber.StatusAccepted)
					return nil
				case model.StatusInactive:
				default:
				}
			}

			entityConfig, err := oidfed.GetEntityConfiguration(req.Subject)
			if err != nil {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("could not obtain entity configuration"))
			}
			if len(req.EntityTypes) == 0 {
				req.EntityTypes = entityConfig.Metadata.GuessEntityTypes()
			}
			info := model.SubordinateInfo{
				JWKS: model.NewJWKS(entityConfig.JWKS),
				Entity: model.Entity{
					EntityTypes: model.NewEntityTypes(req.EntityTypes),
					EntityID:    entityConfig.Subject,
				},
				Status: model.StatusPending,
			}
			if err = store.Write(
				entityConfig.Subject, info,
			); err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			ctx.Status(fiber.StatusAccepted)
			return nil
		},
	)
}
