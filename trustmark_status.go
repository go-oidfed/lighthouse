package lighthouse

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwt"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/oidfedconst"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustMarkStatusConfig holds configuration for the trust mark status endpoint
type TrustMarkStatusConfig struct {
	// InstanceStore for checking issued trust mark instances
	InstanceStore model.IssuedTrustMarkInstanceStore
}

// TrustMarkStatusResponse represents the JWT payload for trust mark status response
type TrustMarkStatusResponse struct {
	Issuer    string `json:"iss"`
	IssuedAt  int64  `json:"iat"`
	TrustMark string `json:"trust_mark"`
	Status    string `json:"status"`
}

// AddTrustMarkStatusEndpoint adds a trust mark status endpoint compliant with OIDC Federation spec.
// The endpoint accepts POST requests with a trust_mark parameter containing the JWT to validate.
// It returns a signed JWT response with the status of the trust mark.
func (fed *LightHouse) AddTrustMarkStatusEndpoint(
	endpoint EndpointConf,
	config TrustMarkStatusConfig,
) {
	fed.fedMetadata.FederationTrustMarkStatusEndpoint = endpoint.ValidateURL(fed.FederationEntity.EntityID())
	if endpoint.Path == "" {
		return
	}

	fed.server.Post(
		endpoint.Path, func(ctx *fiber.Ctx) error {
			return fed.handleTrustMarkStatusRequest(ctx, config)
		},
	)
}

// handleTrustMarkStatusRequest handles a trust mark status request per OIDC Federation spec.
// Request: POST with application/x-www-form-urlencoded body containing trust_mark parameter
// Response: Signed JWT with application/trust-mark-status-response+jwt content type
func (fed *LightHouse) handleTrustMarkStatusRequest(
	ctx *fiber.Ctx,
	config TrustMarkStatusConfig,
) error {
	// Parse the trust_mark parameter from POST body
	trustMarkJWT := ctx.FormValue("trust_mark")
	if trustMarkJWT == "" {
		ctx.Status(fiber.StatusBadRequest)
		return ctx.JSON(oidfed.ErrorInvalidRequest("required parameter 'trust_mark' not given"))
	}

	// Parse and validate the trust mark JWT
	status, err := fed.determineTrustMarkStatus(trustMarkJWT, config)
	if err != nil {
		// If we can't parse the trust mark at all, it's invalid
		return fed.sendTrustMarkStatusResponse(ctx, trustMarkJWT, model.TrustMarkStatusInvalid)
	}

	// If the trust mark is not found (unknown JTI), return 404
	if status == "" {
		ctx.Status(fiber.StatusNotFound)
		return ctx.JSON(oidfed.ErrorNotFound("trust mark not found"))
	}

	return fed.sendTrustMarkStatusResponse(ctx, trustMarkJWT, status)
}

// determineTrustMarkStatus parses the trust mark JWT and determines its status
func (fed *LightHouse) determineTrustMarkStatus(
	trustMarkJWT string,
	config TrustMarkStatusConfig,
) (model.TrustMarkInstanceStatus, error) {
	// Parse the trust mark JWT to extract claims
	parsedTM, err := oidfed.ParseTrustMark([]byte(trustMarkJWT))
	if err != nil {
		return model.TrustMarkStatusInvalid, err
	}

	// Check that we are the issuer
	if parsedTM.Issuer != fed.FederationEntity.EntityID() {
		return model.TrustMarkStatusInvalid, nil
	}

	// Verify the signature using our federation keys
	// Get the signer to access the JWKS for verification
	signer := fed.GeneralJWTSigner.TrustMarkSigner()
	if signer == nil {
		return model.TrustMarkStatusInvalid, nil
	}

	// Parse and verify the JWT signature
	jwks, err := signer.JWKS()
	if err != nil {
		return model.TrustMarkStatusInvalid, err
	}

	_, err = jwt.Parse([]byte(trustMarkJWT), jwt.WithKeySet(jwks.Set))
	if err != nil {
		// Signature verification failed
		return model.TrustMarkStatusInvalid, nil
	}

	// Extract JTI from the trust mark
	jti, ok := parsedTM.Extra["jti"].(string)
	if !ok || jti == "" {
		// Trust marks without JTI can still be validated based on expiration
		// Check expiration
		if parsedTM.ExpiresAt != nil && time.Now().After(parsedTM.ExpiresAt.Time) {
			return model.TrustMarkStatusExpired, nil
		}
		// No JTI but signature valid and not expired - consider active
		return model.TrustMarkStatusActive, nil
	}

	// Look up the instance in the database
	if config.InstanceStore == nil {
		// No instance store configured, fall back to basic validation
		if parsedTM.ExpiresAt != nil && time.Now().After(parsedTM.ExpiresAt.Time) {
			return model.TrustMarkStatusExpired, nil
		}
		return model.TrustMarkStatusActive, nil
	}

	// Get status from the instance store
	status, err := config.InstanceStore.GetStatus(jti)
	if err != nil {
		// Instance not found - this trust mark was not issued by us (or before tracking was enabled)
		var notFound model.NotFoundError
		if isNotFound := err; isNotFound != nil {
			// Try to determine status from the JWT itself
			if parsedTM.ExpiresAt != nil && time.Now().After(parsedTM.ExpiresAt.Time) {
				return model.TrustMarkStatusExpired, nil
			}
			// We verified the signature, so it's valid but we don't track it
			return model.TrustMarkStatusActive, nil
		}
		_ = notFound // suppress unused warning
		return "", err
	}

	return status, nil
}

// sendTrustMarkStatusResponse creates and sends a signed trust mark status response JWT
func (fed *LightHouse) sendTrustMarkStatusResponse(
	ctx *fiber.Ctx,
	trustMarkJWT string,
	status model.TrustMarkInstanceStatus,
) error {
	// Build the response payload
	response := TrustMarkStatusResponse{
		Issuer:    fed.FederationEntity.EntityID(),
		IssuedAt:  time.Now().Unix(),
		TrustMark: trustMarkJWT,
		Status:    string(status),
	}

	// Sign the response using our general JWT signer with the correct type header
	signedJWT, err := fed.GeneralJWTSigner.JWT(response, oidfedconst.JWTTypeTrustMarkStatusResponse)
	if err != nil {
		ctx.Status(fiber.StatusInternalServerError)
		return ctx.JSON(oidfed.ErrorServerError("failed to sign response: " + err.Error()))
	}

	ctx.Set(fiber.HeaderContentType, oidfedconst.ContentTypeTrustMarkStatusResponse)
	return ctx.Send(signedJWT)
}
