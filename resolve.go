package lighthouse

import (
	"time"

	go2 "github.com/adam-hanna/arrayOperations"
	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/apimodel"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/middleware"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// AddResolveEndpoint adds a resolve endpoint
func (fed *LightHouse) AddResolveEndpoint(
	endpoint EndpointConf, allowedTrustAnchors []string, proactiveResolver *oidfed.ProactiveResolver,
) error {
	if endpoint.Path == "" {
		// No local route — e.g. the resolve endpoint is served by an external
		// resolver. Still publish the configured URL in the EC.
		if endpoint.URL != "" {
			fed.fedMetadata.FederationResolveEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
		}
		return nil
	}

	writeResponse := func(ctx *fiber.Ctx, res *oidfed.ResolveResponse) error {
		jwt, err := fed.GeneralJWTSigner.ResolveResponseSigner().JWT(res)
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeResolveResponse)
		return ctx.Send(jwt)
	}

	handler := func(ctx *fiber.Ctx) error {
		var req apimodel.ResolveRequest
		if err := parseRequest(ctx, &req); err != nil {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
		}
		if req.Subject == "" {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
		}
		if len(req.TrustAnchor) == 0 {
			ctx.Status(fiber.StatusBadRequest)
			return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'trust_anchor' not given"))
		}
		if len(allowedTrustAnchors) > 0 {
			req.TrustAnchor = go2.Intersect(allowedTrustAnchors, req.TrustAnchor)
			if len(req.TrustAnchor) == 0 {
				ctx.Status(fiber.StatusNotFound)
				return ctx.JSON(
					oidfed.ErrorInvalidTrustAnchor(
						"all provided trust anchors are not allowed for this endpoint",
					),
				)
			}
		}
		if proactiveResolver != nil {
			for _, ta := range req.TrustAnchor {
				jwt, err := proactiveResolver.Store.ReadJWT(req.Subject, ta, req.EntityTypes)
				if err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				if jwt != nil {
					ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeResolveResponse)
					return ctx.Send(jwt)
				}
				res, err := proactiveResolver.Store.ReadJSON(req.Subject, ta, req.EntityTypes)
				if err != nil {
					ctx.Status(fiber.StatusInternalServerError)
					return ctx.JSON(oidfed.ErrorServerError(err.Error()))
				}
				if res != nil {
					return writeResponse(ctx, res)
				}
			}
		}
		res, err := createResolveResponse(ctx, fed.FederationEntity.EntityID(), req)
		if err != nil {
			return err
		}
		if res != nil {
			return writeResponse(ctx, res)
		}
		// we are here only if createResolveResponse returned send an
		// error (ctx.JSON(Error)), but that was successful.
		return nil
	}

	// Build the auth middleware before advertising anything. If its
	// prerequisites are missing (e.g. auth_enabled with no auth_trust_anchors),
	// fail loudly at load time: advertising a resolve endpoint that is never
	// registered would yield a 404 "no endpoint registered for path ..." on
	// every call while the EC claims the endpoint exists. The published URL is
	// only set after the endpoint is actually registered.
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
			return errors.Wrap(err, "failed to create auth middleware for resolve endpoint")
		}

		fed.registerEndpoint(model.EndpointTypeResolve, endpoint.Path, fiber.MethodPost, handler, auth.EndpointAuth())
		fed.fedMetadata.FederationResolveEndpointAuthMethods = []string{oidfedconst.AuthMethodPrivateKeyJWT}
		fed.fedMetadata.EndpointAuthSigningAlgValuesSupported = jwx.SupportedAlgsStrings()
	} else {
		fed.registerEndpoint(model.EndpointTypeResolve, endpoint.Path, fiber.MethodGet, handler, nil)
	}

	fed.fedMetadata.FederationResolveEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	return nil
}

func createResolveResponse(
	ctx *fiber.Ctx, issuer string,
	req apimodel.ResolveRequest,
) (*oidfed.ResolveResponse, error) {
	// The request's context bounds and cancels the resolution: the lib
	// threads ResolveRequest.Context through the trust tree build and all
	// HTTP fetches, so a client disconnect or deadline aborts the
	// resolution instead of letting it run out its own 15s default.
	req.Context = ctx.UserContext()

	// LocalMetadataResolver is the library's canonical local resolution
	// path: it builds the trust tree, verifies signatures, applies
	// metadata policy, selects the shortest valid chain, and verifies
	// trust marks — replacing the previous manual TrustResolver +
	// chain-filter + selection code here. Its sentinel errors separate the
	// trust path stage from the metadata stage.
	payload, err := oidfed.LocalMetadataResolver{}.ResolveResponsePayload(req)
	if err != nil {
		switch {
		case errors.Is(err, oidfed.ErrNoTrustChainWithValidMetadata):
			ctx.Status(fiber.StatusNotFound)
			return nil, ctx.JSON(
				oidfed.ErrorInvalidMetadata(
					"no trust path with valid metadata found between sub and anchor",
				),
			)
		case errors.Is(err, oidfed.ErrNoTrustChainFound):
			ctx.Status(fiber.StatusNotFound)
			return nil, ctx.JSON(
				oidfed.ErrorInvalidTrustChain(
					"no valid trust path between sub and anchor found",
				),
			)
		default:
			ctx.Status(fiber.StatusInternalServerError)
			return nil, ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
	}
	res := &oidfed.ResolveResponse{
		Issuer:                 issuer,
		Subject:                req.Subject,
		IssuedAt:               unixtime.Unixtime{Time: time.Now()},
		ExpiresAt:              payload.ValidUntil,
		ResolveResponsePayload: payload,
	}
	// Trim the response lifetime to the earliest trust mark expiry, as
	// before; payload.TrustMarks already contains the anchor-verified marks.
	for i := range payload.TrustMarks {
		mark, err := payload.TrustMarks[i].TrustMark()
		if err != nil {
			ctx.Status(fiber.StatusInternalServerError)
			return nil, ctx.JSON(oidfed.ErrorServerError(err.Error()))
		}
		if mark.ExpiresAt != nil && mark.ExpiresAt.Before(res.ExpiresAt.Time) {
			res.ExpiresAt = *mark.ExpiresAt
		}
	}
	return res, nil
}
