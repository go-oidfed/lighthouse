package lighthouse

import (
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"
)

// AddHistoricalKeysEndpoint adds the federation historical keys endpoint
func (fed *LightHouse) AddHistoricalKeysEndpoint(
	endpoint EndpointConf, historyFnc func() (jwx.JWKS, error),
) {
	fed.fedMetadata.FederationHistoricalLKeysEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return
	}
	signer := fed.GeneralJWTSigner.Typed(oidfedconst.JWTTypeJWKS)
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			keys, err := historyFnc()
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			jwt, err := signer.JWT(
				map[string]any{
					"iss":  fed.FederationEntity.EntityID(),
					"iat":  unixtime.Now(),
					"keys": keys,
				},
			)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeJWKS)
			return ctx.Send(jwt)
		},
	)
}
