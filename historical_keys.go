package lighthouse

import (
	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddHistoricalKeysEndpoint adds the federation historical keys endpoint
func (fed *LightHouse) AddHistoricalKeysEndpoint(endpoint EndpointConf) error {
	fed.fedMetadata.FederationHistoricalLKeysEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return nil
	}
	signer := fed.GeneralJWTSigner.Typed(oidfedconst.JWTTypeJWKS)
	handler := func(ctx *fiber.Ctx) error {
		kmsHistory, err := fed.keyManagement.KMSManagedPKs.GetHistorical()
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		apiHistory, err := fed.keyManagement.APIManagedPKs.GetHistorical()
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		allEntries := append(kmsHistory, apiHistory...)
		keys := jwx.NewJWKS()
		for _, k := range allEntries {
			kk, err := k.JWK()
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			_ = keys.AddKey(kk)
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
	}

	if endpoint.AuthEnabled {
		auth, err := middleware.NewPrivateKeyJWTAuth(
			fed.FederationEntity.EntityID(),
			fed.FederationEntity,
			endpoint.AuthTrustAnchors,
			fed.TAResolver(),
			fed.storages.JTI,
		)
		if err != nil {
			return errors.Wrap(err, "failed to create auth middleware for historical keys endpoint")
		}

		fed.registerEndpoint(model.EndpointTypeHistoricalKeys, endpoint.Path, fiber.MethodPost, handler, auth.Middleware())
		fed.fedMetadata.FederationHistoricalLKeysEndpointAuthMethods = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.registerEndpoint(model.EndpointTypeHistoricalKeys, endpoint.Path, fiber.MethodGet, handler, nil)
	}

	return nil
}
