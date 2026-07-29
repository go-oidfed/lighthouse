package middleware

import (
	"slices"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jwt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TAResolver resolves trust anchor entity IDs to oidfed.TrustAnchors at
// request time. This allows JWKS updates to propagate live without restarting
// the middleware.
type TAResolver func(entityIDs []string) oidfed.TrustAnchors

// PrivateKeyJWTAuth implements private_key_jwt authentication middleware
type PrivateKeyJWTAuth struct {
	entityID       string
	fedEntity      oidfed.FederationEntity
	trustAnchorIDs []string
	taResolver     TAResolver
	jtiStorage     model.JTIStorageBackend
	logger         zerolog.Logger
}

// NewPrivateKeyJWTAuth creates a new private_key_jwt authentication middleware.
// The trustAnchorIDs are resolved to oidfed.TrustAnchors at request time via
// taResolver, so JWKS updates propagate live.
func NewPrivateKeyJWTAuth(
	entityID string,
	fedEntity oidfed.FederationEntity,
	trustAnchorIDs []string,
	taResolver TAResolver,
	jtiStorage model.JTIStorageBackend,
) (*PrivateKeyJWTAuth, error) {
	if entityID == "" {
		return nil, ErrInvalidConfig("entity ID is required")
	}
	if fedEntity == nil {
		return nil, ErrInvalidConfig("federation entity is required")
	}
	if len(trustAnchorIDs) == 0 {
		return nil, ErrInvalidConfig("at least one trust anchor is required")
	}
	if taResolver == nil {
		return nil, ErrInvalidConfig("trust anchor resolver is required")
	}
	if jtiStorage == nil {
		return nil, ErrInvalidConfig("JTI storage is required")
	}

	return &PrivateKeyJWTAuth{
		entityID:       entityID,
		fedEntity:      fedEntity,
		trustAnchorIDs: trustAnchorIDs,
		taResolver:     taResolver,
		jtiStorage:     jtiStorage,
		logger: log.With().
			Str("component", "auth_private_key_jwt").
			Str("entity_id", entityID).
			Logger(),
	}, nil
}

// Middleware returns the Fiber middleware handler for private_key_jwt authentication
func (a *PrivateKeyJWTAuth) Middleware() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		startTime := time.Now()

		// Extract client assertion from request
		clientAssertion := ctx.FormValue("client_assertion")
		assertionType := ctx.FormValue("client_assertion_type")

		// Validate client_assertion_type
		if assertionType != oidfedconst.OAuthClientAssertionJWTBearer {
			a.logger.Debug().Str("assertion_type", assertionType).Msg("missing or invalid client_assertion_type")
			return ctx.Status(fiber.StatusBadRequest).JSON(
				oidfed.ErrorInvalidRequest("missing or invalid client_assertion_type, expected: " + oidfedconst.OAuthClientAssertionJWTBearer),
			)
		}

		// Validate client_assertion is present
		if clientAssertion == "" {
			a.logger.Debug().Msg("missing client_assertion parameter")
			return ctx.Status(fiber.StatusBadRequest).JSON(
				oidfed.ErrorInvalidRequest("missing client_assertion parameter"),
			)
		}

		// Parse and validate the JWT
		clientEntityID, entityStatement, err := a.validateAssertion(clientAssertion)
		if err != nil {
			// Convert AuthError to proper OAuth 2.0 error response
			if authErr, ok := err.(*AuthError); ok {
				status := a.getErrorStatus(authErr.Code)
				return ctx.Status(status).JSON(
					oidfed.Error{
						Error:            authErr.Code,
						ErrorDescription: authErr.Description,
					},
				)
			}
			// For non-AuthError, return server error
			a.logger.Error().Err(err).Msg("unexpected authentication error")
			return ctx.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError("authentication failed"))
		}

		// Set context values for downstream handlers
		ctx.Locals("client_entity_id", clientEntityID)
		ctx.Locals("client_entity_statement", entityStatement)
		ctx.Locals("auth_method", oidfedconst.AuthMethodPrivateKeyJWT)

		a.logger.Debug().
			Str("client_entity_id", clientEntityID).
			Int64("duration_ms", time.Since(startTime).Milliseconds()).
			Msg("successful private_key_jwt authentication")

		return ctx.Next()
	}
}

// getErrorStatus maps OAuth 2.0 error codes to HTTP status codes
func (*PrivateKeyJWTAuth) getErrorStatus(errorCode string) int {
	switch errorCode {
	case "invalid_request":
		return fiber.StatusBadRequest
	case "invalid_client":
		return fiber.StatusUnauthorized
	case "invalid_grant":
		return fiber.StatusBadRequest
	case "server_error":
		return fiber.StatusInternalServerError
	default:
		return fiber.StatusBadRequest
	}
}

// validateAssertion validates the client assertion JWT and returns the client entity ID and entity statement
func (a *PrivateKeyJWTAuth) validateAssertion(clientAssertion string) (
	string, *oidfed.EntityStatement, error,
) {
	// First pass: parse without verification to extract claims
	token, err := jwt.ParseInsecure([]byte(clientAssertion))
	if err != nil {
		return "", nil, ErrInvalidRequest("invalid client assertion JWT format")
	}

	// Extract and validate required claims
	iss, ok := token.Issuer()
	if !ok || iss == "" {
		return "", nil, ErrInvalidRequest("missing 'iss' claim in client assertion")
	}

	sub, ok := token.Subject()
	if !ok || sub == "" {
		return "", nil, ErrInvalidRequest("missing 'sub' claim in client assertion")
	}

	// Per spec, iss and sub MUST be identical
	if iss != sub {
		return "", nil, ErrInvalidClient("client assertion 'iss' and 'sub' claims must be identical")
	}

	aud, ok := token.Audience()
	if !ok || len(aud) == 0 {
		return "", nil, ErrInvalidRequest("missing 'aud' claim in client assertion")
	}

	// Validate audience matches this entity
	audMatch := slices.Contains(aud, a.entityID)
	if !audMatch {
		return "", nil, ErrInvalidClient("client assertion audience does not match this entity")
	}

	iat, ok := token.IssuedAt()
	if !ok {
		return "", nil, ErrInvalidRequest("missing 'iat' claim in client assertion")
	}

	exp, ok := token.Expiration()
	if !ok {
		return "", nil, ErrInvalidRequest("missing 'exp' claim in client assertion")
	}

	now := time.Now()

	// Validate iat is not in the future (with small clock skew allowance)
	if iat.After(now.Add(5 * time.Second)) {
		return "", nil, ErrInvalidGrant("client assertion 'iat' claim is in the future")
	}

	// Validate exp is not in the past (with small clock skew allowance)
	if exp.Before(now.Add(-5 * time.Second)) {
		return "", nil, ErrInvalidGrant("client assertion has expired")
	}

	// Get jti from token claims using the proper jwx v4 API
	jti, err := jwt.Get[string](token, "jti")
	if err != nil || jti == "" {
		return "", nil, ErrInvalidRequest("missing or invalid 'jti' claim in client assertion")
	}

	// Check for JTI replay
	jtiExists, err := a.jtiStorage.Exists(jti)
	if err != nil {
		return "", nil, ErrServerError("server error during authentication")
	}
	if jtiExists {
		return "", nil, ErrInvalidGrant("client assertion JTI has already been used (replay attack prevented)")
	}

	// Resolve client's trust chain to get entity statement
	clientEntityStmt, resolveErr := a.resolveClientTrustChain(iss)
	if resolveErr != nil {
		if authErr, ok := resolveErr.(*AuthError); ok {
			return "", nil, authErr
		}
		return "", nil, ErrServerError("trust chain resolution failed")
	}

	// Verify JWT signature using client's public keys
	if err = a.verifySignature(clientAssertion, clientEntityStmt); err != nil {
		return "", nil, ErrInvalidClient("invalid client assertion signature")
	}

	// Store JTI to prevent replay
	// Use the assertion expiration time
	if err = a.jtiStorage.Store(jti, exp); err != nil {
		a.logger.Error().Err(err).Str("jti", jti).Msg("failed to store JTI")
	}

	return iss, clientEntityStmt, nil
}

// resolveClientTrustChain resolves the client's trust chain and returns the leaf entity statement
func (a *PrivateKeyJWTAuth) resolveClientTrustChain(clientEntityID string) (*oidfed.EntityStatement, error) {
	// Resolve trust anchors live at request time so JWKS updates propagate.
	trustAnchors := a.taResolver(a.trustAnchorIDs)
	if len(trustAnchors) == 0 {
		return nil, ErrInvalidClient("no resolvable trust anchors configured")
	}
	// Create trust resolver with configured trust anchors
	resolver := oidfed.TrustResolver{
		TrustAnchors:   trustAnchors,
		StartingEntity: clientEntityID,
		Types:          nil, // Accept any entity type
	}

	// Resolve to valid chains (without metadata validation first)
	chains := resolver.ResolveToValidChainsWithoutVerifyingMetadata()
	if len(chains) == 0 {
		return nil, ErrInvalidClient("client is not a member of any trusted federation")
	}

	// Filter chains with valid metadata
	chains = chains.Filter(oidfed.TrustChainsFilterValidMetadata)
	if len(chains) == 0 {
		return nil, ErrInvalidClient("client metadata validation failed")
	}

	// Select the shortest valid chain
	filteredChains := chains.Filter(oidfed.TrustChainsFilterMinPathLength)
	if len(filteredChains) == 0 {
		return nil, ErrInvalidClient("no chains after min path filter")
	}
	selectedChain := filteredChains[0]

	// Get the leaf entity configuration (first in chain)
	if len(selectedChain) == 0 {
		return nil, ErrInvalidClient("selected chain is empty")
	}
	leafEntityConfig := selectedChain[0]

	if leafEntityConfig == nil {
		return nil, ErrInvalidClient("leaf entity config is nil")
	}

	return leafEntityConfig, nil
}

// verifySignature verifies the JWT signature using the client's public keys from entity statement
func (*PrivateKeyJWTAuth) verifySignature(clientAssertion string, entityStmt *oidfed.EntityStatement) error {
	// Get the client's federation signing keys from the entity statement
	if entityStmt == nil {
		return ErrInvalidClient("entity statement is nil")
	}

	// The JWKS is directly in the entity statement payload
	if entityStmt.JWKS.Set == nil || entityStmt.JWKS.Len() == 0 {
		return ErrInvalidClient("client entity statement missing jwks")
	}

	// Parse and verify the JWT
	_, err := jwt.Parse(
		[]byte(clientAssertion),
		jwt.WithKeySet(entityStmt.JWKS.Set),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(5*time.Second),
	)
	if err != nil {
		return ErrSignatureVerification(err)
	}

	return nil
}

// Helper functions for creating typed errors

// AuthError represents an authentication error with OAuth 2.0 error codes
type AuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e AuthError) Error() string {
	return e.Description
}

// ToFiberError converts an AuthError to a Fiber error with appropriate HTTP status
func (e AuthError) ToFiberError(status int) *fiber.Error {
	return fiber.NewError(status, e.Description)
}

// ErrInvalidConfig creates an error for invalid middleware configuration
func ErrInvalidConfig(msg string) error {
	return &AuthError{
		Code:        "server_error",
		Description: "authentication configuration error: " + msg,
	}
}

// ErrInvalidRequest creates an error for invalid request parameters
func ErrInvalidRequest(msg string) error {
	return &AuthError{
		Code:        "invalid_request",
		Description: msg,
	}
}

// ErrInvalidClient creates an error for client authentication failures
func ErrInvalidClient(msg string) error {
	return &AuthError{
		Code:        "invalid_client",
		Description: msg,
	}
}

// ErrInvalidGrant creates an error for grant-related issues (expired, replayed)
func ErrInvalidGrant(msg string) error {
	return &AuthError{
		Code:        "invalid_grant",
		Description: msg,
	}
}

// ErrServerError creates an error for server-side errors
func ErrServerError(msg string) error {
	return &AuthError{
		Code:        "server_error",
		Description: msg,
	}
}

// ErrInvalidMetadata creates an error for invalid client metadata
func ErrInvalidMetadata(msg string) error {
	return &AuthError{
		Code:        "invalid_client",
		Description: "invalid client metadata: " + msg,
	}
}

// ErrSignatureVerification creates an error for signature verification failures
func ErrSignatureVerification(underlying error) error {
	return &AuthError{
		Code:        "invalid_client",
		Description: "signature verification failed: " + underlying.Error(),
	}
}
