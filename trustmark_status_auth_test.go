package lighthouse

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/lestrrat-go/jwx/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// Regression tests for issue #123: an authenticated trust mark status request
// returned 404 "Cannot POST /trust-mark/status" because the private_key_jwt
// middleware ended its success path with ctx.Next(), which — invoked directly
// from the federation endpoint dispatcher inside the catch-all route — fell
// through fiber's route stack instead of reaching the endpoint handler.

// setupAuthenticatedStatusTestApp builds a LightHouse serving a trust mark
// status endpoint with private_key_jwt auth enabled, a self-signed trust
// anchor (JWKS set via TrustAnchorRepo), and a leaf entity whose entity
// configuration is served by a real httptest server so the auth middleware can
// resolve the client's trust chain.
func setupAuthenticatedStatusTestApp(t *testing.T) (*fiber.App, *LightHouse, jwx.SigningKey, string, string) {
	t.Helper()

	// Server entity (the trust anchor / status endpoint operator). Its entity
	// configuration is served over real HTTP: the trust resolver fetches the
	// authority EC during chain resolution, so the entity ID must be a
	// resolvable URL. The server also serves the fetch endpoint for the leaf's
	// subordinate statement.
	var serverEntityID string
	serverSK, _, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)
	serverJWKS := es256PubJWKS(t, serverSK)
	// Client leaf key and entity ID are declared before the server handler so
	// the fetch endpoint can embed the client's JWKS in the subordinate
	// statement.
	clientSK, _, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)
	clientJWKS := es256PubJWKS(t, clientSK)
	var clientEntityID string
	serverECServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
			if r.URL.Path == "/fetch" {
				// Subordinate statement: issued by the TA about the leaf.
				_, _ = w.Write(signTestSubordinateStatement(t, serverSK, clientEntityID, serverEntityID, clientJWKS))
				return
			}
			_, _ = w.Write(signTestEntityConfigurationWithMetadata(t, serverSK, serverEntityID, serverEntityID))
		},
	))
	t.Cleanup(serverECServer.Close)
	serverEntityID = serverECServer.URL
	// Client leaf entity, served over real HTTP for trust chain resolution.
	clientECServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
			_, _ = w.Write(signTestEntityConfigurationWithMetadata(t, clientSK, clientEntityID, serverEntityID))
		},
	))
	t.Cleanup(clientECServer.Close)
	clientEntityID = clientECServer.URL

	// Trust anchor = the server entity itself, with its JWKS so the leaf's
	// chain can be verified without network fetches of the TA's EC.
	repo := NewTrustAnchorRepo(nil)
	repo.Add(&model.TrustAnchor{
		EntityID: serverEntityID,
		JWKS:     model.JWKS{Keys: serverJWKS},
	})

	// General signer for the lighthouse entity (status response signing).
	versatile := testTrustMarkVersatileSigner{sk: serverSK, alg: jwa.ES256(), jwks: serverJWKS}
	gs := jwx.NewGeneralJWTSigner(versatile, []jwa.SignatureAlgorithm{jwa.ES256()})

	fed := &LightHouse{
		FederationEntity: authTestFedEntity{entityID: serverEntityID},
		GeneralJWTSigner: gs,
		TrustMarkIssuer:  oidfed.NewTrustMarkIssuer(serverEntityID, gs.TrustMarkSigner(), nil),
		trustAnchorRepo:  repo,
		storages: model.Backends{
			JTI: storage.NewJTICacheStorage(),
		},
		fedMetadata: oidfed.FederationEntityMetadata{},
	}

	require.NoError(t, fed.AddTrustMarkStatusEndpoint(
		EndpointConf{
			Path:             "/trust-mark/status",
			AuthEnabled:      true,
			AuthTrustAnchors: []string{serverEntityID},
		},
		TrustMarkStatusConfig{},
	))

	app := fiber.New()
	app.All("/*", fed.dispatch)
	return app, fed, clientSK, clientEntityID, serverEntityID
}

// authTestFedEntity is a federation entity stub with a configurable entity
// ID, used by the authenticated status endpoint tests.
type authTestFedEntity struct {
	entityID string
}

func (e authTestFedEntity) EntityID() string { return e.entityID }
func (authTestFedEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return nil, nil
}
func (authTestFedEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (authTestFedEntity) SignEntityStatement(oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (authTestFedEntity) SignEntityStatementWithHeaders(
	oidfed.EntityStatementPayload, jws.Headers,
) ([]byte, error) {
	return nil, nil
}

func es256PubJWKS(t *testing.T, sk jwx.SigningKey) jwx.JWKS {
	t.Helper()
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	require.NoError(t, pub.Set(jwk.AlgorithmKey, jwa.ES256()))
	set := jwx.NewJWKS()
	require.NoError(t, set.AddKey(pub))
	return set
}

// signTestEntityConfigurationWithMetadata signs an entity configuration with
// federation_entity metadata and the TA's entity ID in the authority hints.
func signTestEntityConfigurationWithMetadata(
	t *testing.T, sk jwx.SigningKey, entityID, taID string,
) []byte {
	t.Helper()
	signer := jwx.NewSingleKeyVersatileSigner(sk, jwa.ES256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.ES256()})
	now := time.Now()
	jwtBytes, err := gs.JWT(oidfed.EntityStatementPayload{
		Issuer:         entityID,
		Subject:        entityID,
		IssuedAt:       unixtime.Unixtime{Time: now},
		ExpiresAt:      unixtime.Unixtime{Time: now.Add(time.Hour)},
		JWKS:           es256PubJWKS(t, sk),
		AuthorityHints: []string{taID},
		Metadata: &oidfed.Metadata{
			FederationEntity: &oidfed.FederationEntityMetadata{
				FederationFetchEndpoint: taID + "/fetch",
			},
		},
	}, oidfedconst.JWTTypeEntityStatement)
	require.NoError(t, err)
	return jwtBytes
}

// signTestSubordinateStatement signs a subordinate statement: issued by the
// authority (taID) about the subordinate (subID), embedding the subordinate's
// public JWKS.
func signTestSubordinateStatement(
	t *testing.T, authoritySK jwx.SigningKey, subID, taID string, subJWKS jwx.JWKS,
) []byte {
	t.Helper()
	signer := jwx.NewSingleKeyVersatileSigner(authoritySK, jwa.ES256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.ES256()})
	now := time.Now()
	jwtBytes, err := gs.JWT(oidfed.EntityStatementPayload{
		Issuer:    taID,
		Subject:   subID,
		IssuedAt:  unixtime.Unixtime{Time: now},
		ExpiresAt: unixtime.Unixtime{Time: now.Add(time.Hour)},
		JWKS:      subJWKS,
	}, oidfedconst.JWTTypeEntityStatement)
	require.NoError(t, err)
	return jwtBytes
}

// clientAssertion builds a private_key_jwt client assertion signed by the
// client's federation key with aud = the status endpoint operator.
func clientAssertion(t *testing.T, sk jwx.SigningKey, clientEntityID, aud string) string {
	t.Helper()
	now := time.Now()
	token, err := jwt.NewBuilder().
		Issuer(clientEntityID).
		Subject(clientEntityID).
		Audience([]string{aud}).
		IssuedAt(now).
		Expiration(now.Add(5 * time.Minute)).
		JwtID(fmt.Sprintf("jti-e2e-%d", time.Now().UnixNano())).
		Build()
	require.NoError(t, err)

	headers := jws.NewHeaders()
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	kid, _ := pub.KeyID()
	require.NoError(t, headers.Set(jws.KeyIDKey, kid))

	signed, err := jwt.Sign(
		token,
		jwt.WithKey(jwa.ES256(), sk, jws.WithProtectedHeaders(headers)),
	)
	require.NoError(t, err)
	return string(signed)
}

// urlFormValues builds an x-www-form-urlencoded body from key/value pairs,
// URL-escaping the values.
func urlFormValues(kv ...string) string {
	pairs := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		v := strings.ReplaceAll(kv[i+1], "+", "%2B")
		v = strings.ReplaceAll(v, "&", "%26")
		pairs = append(pairs, kv[i]+"="+v)
	}
	return strings.Join(pairs, "&")
}

// TestTrustMarkStatus_AuthenticatedRequest_ReachesHandler is the regression
// test for issue #123: with auth enabled, a valid private_key_jwt client
// assertion must reach the status handler and return a signed status response
// (200), not fiber's 404 "Cannot POST".
func TestTrustMarkStatus_AuthenticatedRequest_ReachesHandler(t *testing.T) {
	app, fed, clientSK, clientEntityID, _ := setupAuthenticatedStatusTestApp(t)

	fed.TrustMarkIssuer.AddTrustMark(oidfed.TrustMarkSpec{TrustMarkType: "https://refeds.org/issue-123"})
	tm, _, err := fed.IssueTrustMark("https://refeds.org/issue-123", "https://subject.example")
	require.NoError(t, err)

	body := urlFormValues(
		"trust_mark", tm,
		"client_assertion_type", oidfedconst.OAuthClientAssertionJWTBearer,
		"client_assertion", clientAssertion(t, clientSK, clientEntityID, fed.FederationEntity.EntityID()),
	)
	req := httptest.NewRequest(fiber.MethodPost, "/trust-mark/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, respBody := doRequestRaw(t, app, req)

	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(respBody))
	require.Equal(
		t, oidfedconst.ContentTypeTrustMarkStatusResponse,
		resp.Header.Get("Content-Type"), "body: %s", string(respBody),
	)
	// The body is a signed trust-mark-status-response JWT; decode its payload
	// and assert the reported status.
	status := trustMarkStatusFromResponse(t, string(respBody))
	assert.Equal(t, "active", status)
}

// trustMarkStatusFromResponse decodes the payload of a trust mark status
// response JWT (signature not verified here; the client in this test is the
// signer itself) and returns its "status" claim.
func trustMarkStatusFromResponse(t *testing.T, signedJWT string) string {
	t.Helper()
	parts := strings.Split(signedJWT, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims.Status
}

// TestTrustMarkStatus_AuthFailure_Returns401 verifies that authentication
// rejects assertions with a wrong audience before the handler runs, and that
// the OAuth 2.0 error response is returned intact (not overwritten by the
// endpoint handler).
func TestTrustMarkStatus_AuthFailure_Returns401(t *testing.T) {
	app, _, clientSK, clientEntityID, _ := setupAuthenticatedStatusTestApp(t)

	body := urlFormValues(
		"trust_mark", "some.jwt.value",
		"client_assertion_type", oidfedconst.OAuthClientAssertionJWTBearer,
		"client_assertion", clientAssertion(t, clientSK, clientEntityID, "https://wrong-aud.example"),
	)
	req := httptest.NewRequest(fiber.MethodPost, "/trust-mark/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, respBody := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "body: %s", string(respBody))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "body: %s", string(respBody))
	var errBody struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	require.NoError(t, json.Unmarshal(respBody, &errBody), "body: %s", string(respBody))
	assert.Equal(t, "invalid_client", errBody.Error, "body: %s", string(respBody))
}

// TestEndpointAuth_UnresolvableClient_Returns401InvalidClient verifies that a
// client assertion whose issuer cannot resolve to a valid trust chain is
// rejected with a proper OAuth 2.0 error body (regression: the endpoint
// handler previously overwrote it).
func TestEndpointAuth_UnresolvableClient_Returns401InvalidClient(t *testing.T) {
	app, _, _, _, serverEntityID := setupAuthenticatedStatusTestApp(t)

	rogueSK, _, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)
	body := urlFormValues(
		"trust_mark", "whatever",
		"client_assertion_type", oidfedconst.OAuthClientAssertionJWTBearer,
		"client_assertion", clientAssertion(t, rogueSK, "https://rogue.example.org", serverEntityID),
	)
	req := httptest.NewRequest(fiber.MethodPost, "/trust-mark/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, respBody := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "body: %s", string(respBody))
	var errBody struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(respBody, &errBody), "body: %s", string(respBody))
	assert.Equal(t, "invalid_client", errBody.Error, "body: %s", string(respBody))
}
