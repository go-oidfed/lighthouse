package lighthouse

import (
	"time"

	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/apimodel"
	"github.com/go-oidfed/lib/unixtime"
)

// TODO allow limiting the resolve endpoint to certain trust anchors

// AddResolveEndpoint adds a resolve endpoint
func (fed *LightHouse) AddResolveEndpoint(endpoint EndpointConf) {
	fed.Metadata.FederationEntity.FederationResolveEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID)
	if endpoint.Path == "" {
		return
	}
	fed.server.Get(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			var req apimodel.ResolveRequest
			if err := ctx.QueryParser(&req); err != nil {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("could not parse request parameters: " + err.Error()))
			}
			if len(req.TrustAnchor) == 0 {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'trust_anchor' not given"))
			}
			if req.Subject == "" {
				ctx.Status(fiber.StatusBadRequest)
				return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'sub' not given"))
			}
			resolver := oidfed.TrustResolver{
				TrustAnchors:   oidfed.NewTrustAnchorsFromEntityIDs(req.TrustAnchor...),
				StartingEntity: req.Subject,
				Types:          req.EntityTypes,
			}
			chains := resolver.ResolveToValidChainsWithoutVerifyingMetadata()
			if len(chains) == 0 {
				ctx.Status(fiber.StatusNotFound)
				return ctx.JSON(oidfed.ErrorInvalidTrustChain("no valid trust path between sub and anchor found"))
			}
			chains = chains.Filter(oidfed.TrustChainsFilterValidMetadata)
			if len(chains) == 0 {
				ctx.Status(fiber.StatusNotFound)
				return ctx.JSON(
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
			res := oidfed.ResolveResponse{
				Issuer:    fed.FederationEntity.EntityID,
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
			jwt, err := fed.GeneralJWTSigner.ResolveResponseSigner().JWT(res)
			if err != nil {
				ctx.Status(fiber.StatusInternalServerError)
				return ctx.JSON(oidfed.ErrorServerError(err.Error()))
			}
			ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeResolveResponse)
			return ctx.Send(jwt)
		},
	)
}
