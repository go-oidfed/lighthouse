package lighthouse

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// signJWKSet builds a signed JWK Set (application/jwk-set+jwt, typ
// jwk-set+jwt) containing newKeys, signed with sk, with the given iss/sub.
func signJWKSet(
	t *testing.T, sk jwx.SigningKey, newKeys jwx.JWKS, iss, sub string,
) []byte {
	t.Helper()
	signer := jwx.NewSingleKeyVersatileSigner(sk, jwa.RS256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.RS256()})
	headers := jws.NewHeaders()
	if pub, err := jwk.PublicKeyOf(sk); err == nil {
		if kid, ok := pub.KeyID(); ok && kid != "" {
			_ = headers.Set(jws.KeyIDKey, kid)
		}
	}
	jwtBytes, err := gs.JWTWithHeaders(
		map[string]any{
			"keys": newKeys,
			"iss":  iss,
			"sub":  sub,
		},
		headers,
		oidfedconst.JWTTypeJWKS,
	)
	require.NoError(t, err)
	return jwtBytes
}

// signEntityConfiguration builds a signed Entity Configuration JWT (typ
// entity-statement+jwt) for the given entityID, signed with sk, embedding
// sk's public JWKS and an exp of now + 1h.
func signEntityConfiguration(
	t *testing.T, sk jwx.SigningKey, entityID string,
) []byte {
	t.Helper()
	pubJWKS := pubJWKS(t, sk)
	signer := jwx.NewSingleKeyVersatileSigner(sk, jwa.RS256())
	gs := jwx.NewGeneralJWTSigner(signer, []jwa.SignatureAlgorithm{jwa.RS256()})
	headers := jws.NewHeaders()
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	kid, _ := pub.KeyID()
	_ = headers.Set(jws.KeyIDKey, kid)
	now := time.Now()
	jwtBytes, err := gs.JWTWithHeaders(
		oidfed.EntityStatementPayload{
			Issuer:    entityID,
			Subject:   entityID,
			IssuedAt:  unixtime.Unixtime{Time: now},
			ExpiresAt: unixtime.Unixtime{Time: now.Add(time.Hour)},
			JWKS:      pubJWKS,
		},
		headers,
		oidfedconst.JWTTypeEntityStatement,
	)
	require.NoError(t, err)
	return jwtBytes
}

// assertErrBody unmarshals an OAuth error response and asserts the error code.
func assertErrBody(t *testing.T, body []byte, expectedError string) {
	t.Helper()
	var errBody struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	require.NoError(t, json.Unmarshal(body, &errBody), "body: %s", string(body))
	assert.Equal(t, expectedError, errBody.Error, "body: %s", string(body))
}

// setupJwksUpdateTestApp builds a minimal LightHouse + Fiber app with the
// federation_jwks_update_endpoint registered at /jwks-update.
func setupJwksUpdateTestApp(
	t *testing.T, subs ...model.ExtendedSubordinateInfo,
) *fiber.App {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)

	for _, s := range subs {
		require.NoError(t, store.SubordinateStorage().Add(s))
	}

	fed := &LightHouse{
		FederationEntity: stubFedEntity{},
		storages: model.Backends{
			Subordinates: store.SubordinateStorage(),
		},
		fedMetadata: oidfed.FederationEntityMetadata{},
	}

	require.NoError(t, fed.AddJWKSUpdateEndpoint(
		EndpointConf{Path: "/jwks-update"},
		store.SubordinateStorage(),
	))

	app := fiber.New()
	app.All("/*", fed.dispatch)
	return app
}

// TestJwksUpdateEndpoint_BadSignature_Returns401InvalidClient verifies that a
// signed JWK Set whose signature does not verify against the subordinate's
// currently stored federation keys is rejected with 401 invalid_client (the
// signature is the client-auth mechanism on this endpoint).
func TestJwksUpdateEndpoint_BadSignature_Returns401InvalidClient(t *testing.T) {
	entityID := "https://sub-badsig.example"

	// Subordinate's currently stored federation keys.
	subKeys := pubJWKS(t, rsaKey(t))

	app := setupJwksUpdateTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(subKeys),
	})

	// Build a signed JWK Set signed with a DIFFERENT (unknown) key.
	attackerSK := rsaKey(t)
	newKeys := jwksWithKid(t, "new-kid")
	signed := signJWKSet(t, attackerSK, newKeys, entityID, entityID)

	req := httptest.NewRequest("POST", "/jwks-update", bytes.NewReader(signed))
	req.Header.Set("Content-Type", oidfedconst.ContentTypeJWKS)
	resp, body := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assertErrBody(t, body, "invalid_client")
}

// TestJwksUpdateEndpoint_NoKnownJWKS_Returns401InvalidClient verifies that
// when the subordinate has no stored JWKS to verify the signed JWK Set against,
// the endpoint returns 401 invalid_client (cannot establish authenticity).
func TestJwksUpdateEndpoint_NoKnownJWKS_Returns401InvalidClient(t *testing.T) {
	entityID := "https://sub-nojwks.example"

	app := setupJwksUpdateTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(jwx.NewJWKS()), // empty JWKS
	})

	// A well-formed signed JWK Set (signed with any key); it never reaches
	// signature verification because there are no stored keys to verify
	// against.
	signerSK := rsaKey(t)
	newKeys := jwksWithKid(t, "new-kid")
	signed := signJWKSet(t, signerSK, newKeys, entityID, entityID)

	req := httptest.NewRequest("POST", "/jwks-update", bytes.NewReader(signed))
	req.Header.Set("Content-Type", oidfedconst.ContentTypeJWKS)
	resp, body := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assertErrBody(t, body, "invalid_client")
}

// setupJwksUpdateTriggerTestApp builds a minimal LightHouse + Fiber app with
// the federation_jwks_update_trigger_endpoint (AuthEnabled=false) registered at
// /jwks-trigger. The subordinate's entityID MUST equal the httptest server URL
// so RefreshSubordinateJWKSFromEC fetches the EC from that server.
func setupJwksUpdateTriggerTestApp(
	t *testing.T, sub model.ExtendedSubordinateInfo,
) *fiber.App {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	require.NoError(t, store.SubordinateStorage().Add(sub))

	fed := &LightHouse{
		FederationEntity: stubFedEntity{},
		storages: model.Backends{
			Subordinates: store.SubordinateStorage(),
		},
		fedMetadata: oidfed.FederationEntityMetadata{},
	}

	require.NoError(t, fed.AddJWKSUpdateTriggerEndpoint(
		EndpointConf{Path: "/jwks-trigger"},
		store.SubordinateStorage(),
	))

	app := fiber.New()
	app.All("/*", fed.dispatch)
	return app
}

// TestJwksUpdateTriggerEndpoint_ECSignatureFailure_Returns401InvalidClient
// verifies that when the fetched Entity Configuration's signature cannot be
// verified against the subordinate's currently stored JWKS, the trigger
// endpoint returns 401 invalid_client (mirroring the jwks_update endpoint).
func TestJwksUpdateTriggerEndpoint_ECSignatureFailure_Returns401InvalidClient(t *testing.T) {
	// EC signing key (key A) — the one the subordinate signs its EC with.
	ecSK := rsaKey(t)
	// Stored JWKS is a DIFFERENT key (key B), so EC.Verify(stored) fails.
	storedKeys := pubJWKS(t, rsaKey(t))

	// Stand up a server that serves an EC signed with key A. The entityID is
	// the server URL, set after the server is created.
	var entityID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
		_, _ = w.Write(signEntityConfiguration(t, ecSK, entityID))
	}))
	t.Cleanup(srv.Close)
	entityID = srv.URL

	app := setupJwksUpdateTriggerTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(storedKeys),
	})

	req := httptest.NewRequest(
		"POST", "/jwks-trigger",
		bytes.NewReader([]byte("sub="+entityID)),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, body := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assertErrBody(t, body, "invalid_client")
}

// TestJwksUpdateTriggerEndpoint_ECFetchFailure_Returns502ServerError verifies
// that when the Entity Configuration cannot be fetched (upstream returns 500),
// the trigger endpoint returns 502 server_error.
func TestJwksUpdateTriggerEndpoint_ECFetchFailure_Returns502ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	entityID := srv.URL

	app := setupJwksUpdateTriggerTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(pubJWKS(t, rsaKey(t))),
	})

	req := httptest.NewRequest(
		"POST", "/jwks-trigger",
		bytes.NewReader([]byte("sub="+entityID)),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, body := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assertErrBody(t, body, "server_error")
}
