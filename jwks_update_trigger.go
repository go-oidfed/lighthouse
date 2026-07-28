package lighthouse

import (
	stderrors "errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// jwksUpdateTriggerRequest is the body for the trigger endpoint when client
// auth is not used. The "sub" field identifies the subordinate whose JWKS
// should be refreshed.
type jwksUpdateTriggerRequest struct {
	Subject string `json:"sub" form:"sub" query:"sub"`
}

// AddJWKSUpdateTriggerEndpoint adds the federation_jwks_update_trigger_endpoint.
// A POST to this endpoint tells Lighthouse to re-fetch the
// subordinate's JWKS from its Entity Configuration and update the stored keys
// if they changed.
//
// When AuthEnabled is true, private_key_jwt client authentication is required
// and the authenticated client entity is used as the target subordinate (any
// body "sub" is ignored). When AuthEnabled is false, the subordinate must be
// identified via the "sub" parameter in the request body.
//
// The endpoint is published in the federation_entity metadata under
// "federation_jwks_update_trigger_endpoint".
func (fed *LightHouse) AddJWKSUpdateTriggerEndpoint(
	endpoint EndpointConf, store model.SubordinateStorageBackend,
) error {
	if fed.fedMetadata.Extra == nil {
		fed.fedMetadata.Extra = make(map[string]interface{})
	}
	fed.fedMetadata.Extra["federation_jwks_update_trigger_endpoint"] = endpoint.ValidateURL(
		fed.FederationEntity.EntityID(),
	)
	if endpoint.Path == "" {
		return nil
	}
	handler := func(ctx *fiber.Ctx) error {
		var target string
		if endpoint.AuthEnabled {
			// The private_key_jwt middleware stores the authenticated client
			// entity ID in ctx.Locals("client_entity_id").
			if v, ok := ctx.Locals("client_entity_id").(string); ok && v != "" {
				target = v
			} else {
				ctx.Status(fiber.StatusUnauthorized)
				return ctx.JSON(oidfed.ErrorInvalidClient("client authentication required"))
			}
		} else {
			var req jwksUpdateTriggerRequest
			if err := parseRequest(ctx, &req); err != nil {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
			}
			target = req.Subject
		}
		if target == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
		}

		info, err := store.Get(target)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		if info == nil || info.Status == model.StatusBlocked || info.Status == model.StatusInactive {
			ctx.Status(fiber.StatusNotFound)
			return ctx.JSON(oidfed.ErrorNotFound("subordinate not known"))
		}

		changed, err := fed.RefreshSubordinateJWKSFromEC(target)
		if err != nil {
			// The EC signature failing to verify against the stored JWKS is an
			// authenticity failure of the subordinate, mirroring the jwks_update
			// endpoint semantics: 401 invalid_client.
			if stderrors.Is(err, errECSignatureFailed) {
				ctx.Status(fiber.StatusUnauthorized)
				return ctx.JSON(oidfed.ErrorInvalidClient("failed to refresh JWKS: " + err.Error()))
			}
			// A fetch failure (network error or malformed EC) is an upstream
			// problem: 502 server_error.
			if stderrors.Is(err, errECFetchFailed) {
				ctx.Status(fiber.StatusBadGateway)
				return ctx.JSON(oidfed.ErrorServerError("failed to refresh JWKS: " + err.Error()))
			}
			// Anything else (internal store errors, etc.) is a server fault.
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError("failed to refresh JWKS: " + err.Error()))
		}

		// Record an event for the trigger.
		if fed.storages.SubordinateEvents != nil {
			var actor *string
			if endpoint.AuthEnabled {
				actor = &target
			}
			_ = fed.storages.SubordinateEvents.Add(
				model.SubordinateEvent{
					SubordinateID: info.ID,
					Actor:         actor,
					Timestamp:     nowUnix(),
					Type:          model.EventTypeJWKSUpdateTriggered,
					Message:       boolPtrIfTrue(changed, "jwks changed"),
				},
			)
		}

		ctx.Status(fiber.StatusOK)
		return ctx.JSON(
			fiber.Map{
				"sub":     target,
				"changed": changed,
			},
		)
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
			return errors.Wrap(err, "failed to create auth middleware for jwks_update_trigger endpoint")
		}
		fed.registerEndpoint(
			model.EndpointTypeJwksUpdateTrigger, endpoint.Path, fiber.MethodPost, handler, auth.Middleware(),
		)
		fed.fedMetadata.Extra["federation_jwks_update_trigger_endpoint_auth_methods"] = []string{
			oidfedconst.AuthMethodPrivateKeyJWT,
		}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.registerEndpoint(model.EndpointTypeJwksUpdateTrigger, endpoint.Path, fiber.MethodPost, handler, nil)
	}
	return nil
}

func boolPtrIfTrue(b bool, msg string) *string {
	if !b {
		return nil
	}
	return &msg
}
