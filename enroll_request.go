package lighthouse

import (
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
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
				case storage.StatusActive:
					ctx.Status(fiber.StatusNoContent)
					return nil
				case storage.StatusBlocked:
					ctx.Status(fiber.StatusForbidden)
					return ctx.JSON(
						oidfed.ErrorInvalidRequest(
							"the entity cannot enroll",
						),
					)
				case storage.StatusPending:
					ctx.Status(fiber.StatusAccepted)
					return nil
				case storage.StatusInactive:
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
			info := storage.SubordinateInfo{
				JWKS:        entityConfig.JWKS,
				EntityTypes: req.EntityTypes,
				EntityID:    entityConfig.Subject,
				Status:      storage.StatusPending,
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
