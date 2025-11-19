package lighthouse

import (
	"time"

	go2 "github.com/adam-hanna/arrayOperations"
	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/apimodel"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"
)

// AddResolveEndpoint adds a resolve endpoint
func (fed *LightHouse) AddResolveEndpoint(
	endpoint EndpointConf, allowedTrustAnchors []string, proactiveResolver *oidfed.ProactiveResolver,
) {
	fed.Metadata.FederationEntity.FederationResolveEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID)
	if endpoint.Path == "" {
		return
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

	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			var req apimodel.ResolveRequest
			if err := ctx.QueryParser(&req); err != nil {
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
			res, err := createResolveResponse(ctx, fed.FederationEntity.EntityID, req)
			if err != nil {
				return err
			}
			if res != nil {
				return writeResponse(ctx, res)
			}
			// we are here only if createResolveResponse returned send an
			// error (ctx.JSON(Error)), but that was successful.
			return nil
		},
	)
}

func createResolveResponse(
	ctx *fiber.Ctx, issuer string,
	req apimodel.ResolveRequest,
) (*oidfed.ResolveResponse, error) {
	resolver := oidfed.TrustResolver{
		TrustAnchors:   oidfed.NewTrustAnchorsFromEntityIDs(req.TrustAnchor...),
		StartingEntity: req.Subject,
		Types:          req.EntityTypes,
	}
	chains := resolver.ResolveToValidChainsWithoutVerifyingMetadata()
	if len(chains) == 0 {
		ctx.Status(fiber.StatusNotFound)
		return nil, ctx.JSON(
			oidfed.ErrorInvalidTrustChain(
				"no valid trust path between sub and anchor found",
			),
		)
	}
	chains = chains.Filter(oidfed.TrustChainsFilterValidMetadata)
	if len(chains) == 0 {
		ctx.Status(fiber.StatusNotFound)
		return nil, ctx.JSON(
			oidfed.ErrorInvalidMetadata(
				"no trust path with valid metadata found between sub and anchor",
			),
		)
	}
	selectedChain := chains.Filter(oidfed.TrustChainsFilterMinPathLength)[0]
	metadata, _ := selectedChain.Metadata()
	// err cannot be != nil, since ResolveToValidChains only gives chains with valid metadata
	leaf := selectedChain[0]
	ta := selectedChain[len(selectedChain)-1]
	res := &oidfed.ResolveResponse{
		Issuer:    issuer,
		Subject:   req.Subject,
		IssuedAt:  unixtime.Unixtime{Time: time.Now()},
		ExpiresAt: selectedChain.ExpiresAt(),
		ResolveResponsePayload: oidfed.ResolveResponsePayload{
			Metadata:   metadata,
			TrustChain: selectedChain.Messages(),
		},
	}
	if leaf.TrustMarks != nil {
		res.ResolveResponsePayload.TrustMarks = leaf.TrustMarks.VerifiedFederation(&ta.EntityStatementPayload)
	}
	return res, nil
}
