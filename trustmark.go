package lighthouse

import (
	"slices"

	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddTrustMarkEndpoint adds a trust mark endpoint
func (fed *LightHouse) AddTrustMarkEndpoint(
	endpoint EndpointConf,
	store storage.TrustMarkedEntitiesStorageBackend,
	checkers map[string]EntityChecker,
) {
	fed.Metadata.FederationEntity.FederationTrustMarkEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID)
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
				return issueAndSendTrustMark(ctx, fed, trustMarkType, sub)
			case model.StatusBlocked:
				ctx.Status(fiber.StatusForbidden)
				return ctx.JSON(oidfed.ErrorInvalidRequest("subject cannot obtain this trust mark"))
			case model.StatusPending:
				ctx.Status(fiber.StatusAccepted)
				return ctx.JSON(oidfed.ErrorInvalidRequest("approval pending"))
			case model.StatusInactive:
				// subject does not have the trust mark,
				// check if it is entitled to do so
				var checker EntityChecker
				if checkers != nil {
					checker = checkers[trustMarkType]
				}
				if checker == nil {
					ctx.Status(fiber.StatusNotFound)
					return ctx.JSON(
						oidfed.ErrorNotFound("subject does not have this trust mark"),
					)
				}
				entityConfig, err := oidfed.GetEntityConfiguration(sub)
				if err != nil {
					ctx.Status(fiber.StatusBadRequest)
					return ctx.JSON(oidfed.ErrorInvalidRequest("could not obtain entity configuration"))
				}
				ok, _, errResponse := checker.Check(
					entityConfig, entityConfig.Metadata.GuessEntityTypes(),
				)
				if !ok {
					ctx.Status(fiber.StatusNotFound)
					return ctx.JSON(
						oidfed.ErrorNotFound(
							"subject does not have this trust mark and is not" +
								" entitled to get it: " + errResponse.ErrorDescription,
						),
					)
				}
				// ok, so we add sub to the list and issue the trust mark
				if err = store.Approve(trustMarkType, sub); err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
			}
			return issueAndSendTrustMark(ctx, fed, trustMarkType, sub)
		},
	)
}

func issueAndSendTrustMark(
	ctx *fiber.Ctx, fedEntity *LightHouse, trustMarkType, sub string,
) error {
	tm, err := fedEntity.IssueTrustMark(trustMarkType, sub)
	if err != nil {
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
	}
	ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeTrustMark)
	return ctx.SendString(tm.TrustMarkJWT)
}
