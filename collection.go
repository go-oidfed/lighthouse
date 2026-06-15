package lighthouse

import (
	"fmt"
	slices2 "slices"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/apimodel"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"tideland.dev/go/slices"

	"github.com/go-oidfed/lighthouse/middleware"
)

// AddEntityCollectionEndpoint adds an entity collection endpoint
func (fed *LightHouse) AddEntityCollectionEndpoint(
	endpoint EndpointConf, collector oidfed.EntityCollector,
	allowedTrustAnchors []string, paginationSupported bool,
) error {
	if fed.fedMetadata.Extra == nil {
		fed.fedMetadata.Extra = make(map[string]interface{})
	}
	fed.fedMetadata.Extra["federation_collection_endpoint"] = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return nil
	}
	handler := func(ctx *fiber.Ctx) error {
		var req apimodel.EntityCollectionRequest
		if err := parseRequest(ctx, &req); err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
		}
		if !paginationSupported && req.From != "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'from' is not supported"))
		}
		if req.TrustAnchor == "" {
			req.TrustAnchor = fed.FederationEntity.EntityID()
		}
		if len(allowedTrustAnchors) > 0 {
			if !slices2.Contains(allowedTrustAnchors, req.TrustAnchor) {
				ctx.Status(fiber.StatusNotFound)
				return ctx.JSON(oidfed.ErrorInvalidTrustAnchor("trust anchor not allowed for this endpoint"))
			}
		}
		if !paginationSupported && req.Limit != 0 {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorUnsupportedParameter("parameter 'limit' is not supported"))
		}
		if wantedButNotSupported := slices.Subtract(
			req.EntityClaims, []string{
				"entity_id",
				"entity_types",
				"ui_infos",
				"trust_marks",
			},
		); len(wantedButNotSupported) > 0 {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(
				oidfed.ErrorUnsupportedParameter(
					fmt.Sprintf(
						"parameter 'entity_claims' contains the following unsupported values: %+v",
						wantedButNotSupported,
					),
				),
			)
		}
		if wantedButNotSupported := slices.Subtract(
			req.UIClaims, []string{
				"display_name",
				"description",
				"keywords",
				"logo_uri",
				"policy_uri",
				"information_uri",
			},
		); len(wantedButNotSupported) > 0 {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(
				oidfed.ErrorUnsupportedParameter(
					fmt.Sprintf(
						"parameter 'ui_claims' contains the following unsupported values: %+v",
						wantedButNotSupported,
					),
				),
			)
		}
		res, errRes := collector.CollectEntities(req)
		if errRes != nil {
			ctx.Status(errRes.Status)
			return ctx.JSON(errRes)
		}
		return ctx.JSON(res)
	}

	if endpoint.AuthEnabled {
		auth, err := middleware.NewPrivateKeyJWTAuth(
			fed.FederationEntity.EntityID(),
			fed.FederationEntity,
			endpoint.AuthTrustAnchors,
			fed.storages.JTI,
		)
		if err != nil {
			return errors.Wrap(err, "failed to create auth middleware for entity collection endpoint")
		}

		fed.server.Post(endpoint.Path, auth.Middleware(), handler)
		fed.fedMetadata.Extra["federation_collection_endpoint_auth_methods"] = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.server.Get(endpoint.Path, handler)
	}

	return nil
}
