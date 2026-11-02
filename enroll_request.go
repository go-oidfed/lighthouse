package lighthouse

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v3"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddEnrollRequestEndpoint adds an endpoint to request enrollment to this IA
// /TA (this does only add a request to the storage, no automatic enrollment)
func (fed *LightHouse) AddEnrollRequestEndpoint(
	endpoint EndpointConf,
	store model.SubordinateStorageBackend,
) error {
	if fed.fedMetadata.Extra == nil {
		fed.fedMetadata.Extra = make(map[string]any)
	}
	if endpoint.Path == "" {
		// No local route (externally hosted endpoint): still publish a
		// configured URL in the EC.
		if endpoint.URL != "" {
			fed.fedMetadata.Extra["federation_enroll_request_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID())
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
			return errors.Wrap(err, "failed to create auth middleware for enroll request endpoint")
		}
	}
	fed.fedMetadata.Extra["federation_enroll_request_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	handler := func(ctx fiber.Ctx) error {
		var req enrollRequest
		if err := parseRequest(ctx, &req); err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
		}
		if req.Subject == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
		}
		storedInfo, err := store.Get(req.Subject)
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

		subEntityTypes := make([]model.SubordinateEntityType, len(req.EntityTypes))
		for i, t := range req.EntityTypes {
			subEntityTypes[i] = model.SubordinateEntityType{EntityType: t}
		}
		info := model.ExtendedSubordinateInfo{
			JWKS: model.NewJWKS(entityConfig.JWKS),
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID:               entityConfig.Subject,
				SubordinateEntityTypes: subEntityTypes,
				Status:                 model.StatusPending,
			},
		}
		if err = store.Update(
			entityConfig.Subject, info,
		); err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		fed.notifySubordinateJWKSRefresher(entityConfig.Subject)
		ctx.Status(fiber.StatusAccepted)
		return nil
	}

	if endpoint.AuthEnabled {
		fed.registerEndpoint(
			model.EndpointTypeEnrollRequest, endpoint.Path, fiber.MethodPost, handler, auth.EndpointAuth(),
		)
		fed.fedMetadata.Extra["federation_enroll_request_endpoint_auth_methods"] = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.registerEndpoint(model.EndpointTypeEnrollRequest, endpoint.Path, fiber.MethodGet, handler, nil)
	}

	return nil
}
