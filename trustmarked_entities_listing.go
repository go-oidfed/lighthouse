package lighthouse

import (
	"slices"

	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddTrustMarkedEntitiesListingEndpoint adds a trust marked entities listing endpoint.
// Per OIDC Federation spec, this endpoint lists all entities for which trust marks
// have been issued and are still valid (non-revoked, non-expired).
func (fed *LightHouse) AddTrustMarkedEntitiesListingEndpoint(
	endpoint EndpointConf,
	instanceStore model.IssuedTrustMarkInstanceStore,
) error {
	fed.fedMetadata.FederationTrustMarkListEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return nil
	}
	handler := func(ctx *fiber.Ctx) error {
		var req trustMarkQueryRequest
		if err := parseRequest(ctx, &req); err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
		}
		if req.TrustMarkType == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(
				oidfed.ErrorInvalidRequest(
					"required parameter 'trust_mark_type' not given",
				),
			)
		}
		if !slices.Contains(
			fed.TrustMarkIssuer.TrustMarkTypes(),
			req.TrustMarkType,
		) {
			ctx.Status(fiber.StatusNotFound)
			return ctx.JSON(
				oidfed.ErrorNotFound("'trust_mark_type' not known"),
			)
		}

		entities := make([]string, 0)
		var err error

		if req.Subject != "" {
			// Check if specific entity has an active (valid) trust mark instance
			hasActive, err := instanceStore.HasActiveInstance(req.TrustMarkType, req.Subject)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			if hasActive {
				entities = []string{req.Subject}
			}
		} else {
			// List all entities with active (valid) trust mark instances
			entities, err = instanceStore.ListActiveSubjects(req.TrustMarkType)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			if entities == nil {
				entities = make([]string, 0)
			}
		}

		return ctx.JSON(entities)
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
			return errors.Wrap(err, "failed to create auth middleware for trust marked entities listing endpoint")
		}

		fed.registerEndpoint(model.EndpointTypeTrustMarkListing, endpoint.Path, fiber.MethodPost, handler, auth.Middleware())
		fed.fedMetadata.FederationTrustMarkListEndpointAuthMethods = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.registerEndpoint(model.EndpointTypeTrustMarkListing, endpoint.Path, fiber.MethodGet, handler, nil)
	}

	return nil
}
