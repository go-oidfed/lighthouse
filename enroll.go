package lighthouse

import (
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

type enrollRequest struct {
	Subject     string   `json:"sub" form:"sub" query:"sub"`
	EntityTypes []string `json:"entity_type" form:"entity_type" query:"entity_type"`
}

// AddEnrollEndpoint adds an endpoint to enroll to this IA/TA
func (fed *LightHouse) AddEnrollEndpoint(
	endpoint EndpointConf,
	store storage.SubordinateStorageBackend,
	checker EntityChecker,
) {
	if fed.Metadata.FederationEntity.Extra == nil {
		fed.Metadata.FederationEntity.Extra = make(map[string]interface{})
	}
	fed.Metadata.FederationEntity.Extra["federation_enroll_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID)
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
					// This is not necessarily needed, but we return a fetch response
					payload := fed.CreateSubordinateStatement(storedInfo)
					jwt, err := fed.SignEntityStatement(payload)
					if err != nil {
						ctx.Status(fiber.StatusInternalServerError)
						return ctx.JSON(oidfed.ErrorServerError(err.Error()))
					}
					ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
					ctx.Status(fiber.StatusCreated)
					return ctx.Send(jwt)
				case model.StatusPending:
					ctx.Status(fiber.StatusAccepted)
					return ctx.JSON(
						oidfed.ErrorInvalidRequest(
							"the enrollment needs to be approved by an administrator",
						),
					)
				case model.StatusBlocked:
					ctx.Status(fiber.StatusForbidden)
					return ctx.JSON(oidfed.ErrorInvalidRequest("the entity cannot enroll"))
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
			if checker != nil {
				ok, errStatus, errResponse := checker.Check(entityConfig, req.EntityTypes)
				if !ok {
					ctx.Status(errStatus)
					return ctx.JSON(errResponse)
				}
			}

			info := model.SubordinateInfo{
				JWKS: model.NewJWKS(entityConfig.JWKS),
				Entity: model.Entity{
					EntityTypes: model.NewEntityTypes(req.EntityTypes),
					EntityID:    entityConfig.Subject,
				},
				Status: model.StatusActive,
			}
			if err = store.Write(
				entityConfig.Subject, info,
			); err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			// This is not necessarily needed, but we return a fetch response
			payload := fed.CreateSubordinateStatement(&info)
			jwt, err := fed.SignEntityStatement(payload)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
			ctx.Status(fiber.StatusCreated)
			return ctx.Send(jwt)
		},
	)
}
