package lighthouse

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage/model"
)

type fetchRequest struct {
	Subject string `json:"sub" form:"sub" query:"sub"`
}

// AddFetchEndpoint adds a fetch endpoint
func (fed *LightHouse) AddFetchEndpoint(endpoint EndpointConf, store model.SubordinateStorageBackend) error {
	if endpoint.Path == "" {
		// No local route (externally hosted endpoint): still publish a
		// configured URL in the EC.
		if endpoint.URL != "" {
			fed.fedMetadata.FederationFetchEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
		}
		return nil
	}
	// Build the auth middleware before advertising so an auth misconfiguration
	// (e.g. auth_enabled with no auth_trust_anchors) fails loudly at load time
	// instead of publishing an EC entry for an endpoint that is never
	// registered.
	var auth *middleware.PrivateKeyJWTAuth
	if endpoint.AuthEnabled {
		var err error
		auth, err = middleware.NewPrivateKeyJWTAuth(
			fed.FederationEntity.EntityID(),
			fed.FederationEntity,
			endpoint.AuthTrustAnchors,
			fed.TAResolver(),
			fed.storages.JTI,
		)
		if err != nil {
			return errors.Wrap(err, "failed to create auth middleware for fetch endpoint")
		}
	}
	fed.fedMetadata.FederationFetchEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	handler := func(ctx fiber.Ctx) error {
		var req fetchRequest
		if err := parseRequest(ctx, &req); err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
		}
		if req.Subject == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
		}
		if req.Subject == fed.FederationEntity.EntityID() {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("are you looking for the entity configuration?"))
		}
		cacheKey := internal.SubordinateStatementCacheKey(req.Subject)
		var cached []byte
		set, err := cache.Get(cacheKey, &cached)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		if set {
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
			return ctx.Send(cached)
		}
		info, err := store.Get(req.Subject)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		if info == nil {
			ctx.Status(fiber.StatusNotFound)
			return ctx.JSON(oidfed.ErrorNotFound("the requested entity identifier is not found"))
		}
		if info.Status != model.StatusActive {
			ctx.Status(fiber.StatusNotFound)
			return ctx.JSON(oidfed.ErrorNotFound("the requested entity identifier is not found"))
		}
		payload := fed.CreateSubordinateStatement(info)
		jwt, err := fed.SignEntityStatement(payload)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		if ttl := subordinateStatementCacheTTL(payload); ttl > 0 {
			if cacheErr := cache.Set(cacheKey, jwt, ttl); cacheErr != nil {
				log.Error().Err(cacheErr).Str("subordinate", req.Subject).
					Msg("failed to cache subordinate statement")
			}
		}
		ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeEntityStatement)
		return ctx.Send(jwt)
	}

	if endpoint.AuthEnabled {
		fed.registerEndpoint(model.EndpointTypeFetch, endpoint.Path, fiber.MethodPost, handler, auth.EndpointAuth())
		fed.fedMetadata.FederationFetchEndpointAuthMethods = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.registerEndpoint(model.EndpointTypeFetch, endpoint.Path, fiber.MethodGet, handler, nil)
	}

	return nil
}
