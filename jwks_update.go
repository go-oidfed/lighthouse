package lighthouse

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddJWKSUpdateEndpoint adds the federation_jwks_update_endpoint.
// A subordinate POSTs a signed JWK Set (media type application/jwk-set+jwt,
// typ jwk-set+jwt) containing its new federation keys. Lighthouse validates
// that the JWT is signed with one of the subordinate's currently known
// federation keys and, if so, replaces the stored JWKS.
//
// The endpoint is published in the federation_entity metadata under
// "federation_jwks_update_endpoint". No private_key_jwt client auth is used;
// authenticity is established by the signed JWK Set signature.
func (fed *LightHouse) AddJWKSUpdateEndpoint(
	endpoint EndpointConf, store model.SubordinateStorageBackend,
) error {
	if fed.fedMetadata.Extra == nil {
		fed.fedMetadata.Extra = make(map[string]interface{})
	}
	fed.fedMetadata.Extra["federation_jwks_update_endpoint"] = endpoint.ValidateURL(
		fed.FederationEntity.EntityID(),
	)
	if endpoint.Path == "" {
		return nil
	}
	handler := func(ctx *fiber.Ctx) error {
		// Content-Type MUST be application/jwk-set+jwt.
		if ctx.Get(fiber.HeaderContentType) != oidfedconst.ContentTypeJWKS {
			ctx.Status(fiber.StatusUnsupportedMediaType)
			return ctx.JSON(
				oidfed.ErrorInvalidRequest(
					"content type must be " + oidfedconst.ContentTypeJWKS,
				),
			)
		}

		body := ctx.Body()
		if len(body) == 0 {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("request body is empty"))
		}

		signed, err := oidfed.ParseSignedJWKS(body)
		if err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("invalid signed JWK Set: " + err.Error()))
		}

		// The sub claim identifies the subordinate (owner of the keys).
		target := signed.Subject
		if target == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("signed JWK Set missing 'sub' claim"))
		}
		if target != signed.Issuer {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("'iss' and 'sub' claim must be equal"))
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

		// Verify the signed JWK Set signature against the subordinate's
		// currently stored federation JWKS. This enforces that the update is
		// signed with one of the old federation keys known to Lighthouse. The
		// signature on the signed JWK Set is the client-auth mechanism for this
		// endpoint, so a verification failure is an authentication failure
		// (401 invalid_client), not a forbidden request.
		if info.JWKS.Keys.Set == nil || info.JWKS.Keys.Len() == 0 {
			ctx.Status(fiber.StatusUnauthorized)
			return ctx.JSON(
				oidfed.ErrorInvalidClient(
					"subordinate has no known JWKS to verify the signed JWK Set against",
				),
			)
		}
		if !signed.Verify(info.JWKS.Keys) {
			ctx.Status(fiber.StatusUnauthorized)
			return ctx.JSON(
				oidfed.ErrorInvalidClient(
					"signed JWK Set signature could not be verified with the subordinate's known keys",
				),
			)
		}

		// Replace the stored JWKS.
		if err := store.UpdateJWKSByEntityID(target, model.NewJWKS(signed.Keys)); err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError("failed to update JWKS: " + err.Error()))
		}
		_ = cache.Delete(internal.SubordinateStatementCacheKey(target))

		// Record an event.
		if fed.storages.SubordinateEvents != nil {
			_ = fed.storages.SubordinateEvents.Add(
				model.SubordinateEvent{
					Actor:         &info.EntityID,
					SubordinateID: info.ID,
					Timestamp:     nowUnix(),
					Type:          model.EventTypeJWKSUpdated,
				},
			)
		}

		// Notify the refresher so its in-memory KID state stays consistent.
		fed.notifySubordinateJWKSRefresher(target)

		return ctx.SendStatus(fiber.StatusNoContent)
	}

	fed.registerEndpoint(model.EndpointTypeJwksUpdate, endpoint.Path, fiber.MethodPost, handler, nil)
	return nil
}
