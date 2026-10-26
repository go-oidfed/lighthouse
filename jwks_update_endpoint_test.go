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

// expiresStoredJWKS returns a signing key plus a stored JWKS containing its
// public key with its thumbprint kid and an `exp` claim strictly in the past.
// The thumbprint kid matches the kid the signing helpers emit in the JWT
// header, so signature verification would succeed for a non-expired key.
func expiresStoredJWKS(t *testing.T) (jwx.SigningKey, jwx.JWKS) {
	t.Helper()
	sk := rsaKey(t)
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	require.NoError(t, pub.Set("exp", unixtime.Unixtime{Time: time.Now().Add(-time.Hour)}))
	set := jwx.NewJWKS()
	require.NoError(t, set.AddKey(pub))
	return sk, set
}

// signECConfig signs an Entity Configuration for iss/sub with sk, embedding the
// given keys and exp. The header carries the signing key's thumbprint kid.
func signECConfig(
	t *testing.T, sk jwx.SigningKey, iss, sub string, exp time.Time, embedded jwx.JWKS,
) []byte {
	t.Helper()
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
			Issuer:    iss,
			Subject:   sub,
			IssuedAt:  unixtime.Unixtime{Time: now},
			ExpiresAt: unixtime.Unixtime{Time: exp},
			JWKS:      embedded,
		},
		headers,
		oidfedconst.JWTTypeEntityStatement,
	)
	require.NoError(t, err)
	return jwtBytes
}

// signEntityConfigurationFull signs an Entity Configuration for entityID with
// sk, embedding the given keys and exp. The header carries the signing key's
// thumbprint kid.
func signEntityConfigurationFull(
	t *testing.T, sk jwx.SigningKey, entityID string, exp time.Time, embedded jwx.JWKS,
) []byte {
	t.Helper()
	return signECConfig(t, sk, entityID, entityID, exp, embedded)
}

// signEntityConfigurationExp is signEntityConfiguration with a caller-supplied
// expiration for the signed Entity Configuration.
func signEntityConfigurationExp(
	t *testing.T, sk jwx.SigningKey, entityID string, exp time.Time,
) []byte {
	t.Helper()
	return signEntityConfigurationFull(t, sk, entityID, exp, pubJWKS(t, sk))
}

// TestJwksUpdateEndpoint_ExpiredStoredKey_Returns401InvalidClient verifies
// that the jwks_update endpoint rejects a signed JWK Set whose signature was
// made with an expired (but still stored) federation key. An expired key must
// never verify a signed JWK Set.
func TestJwksUpdateEndpoint_ExpiredStoredKey_Returns401InvalidClient(t *testing.T) {
	expiredSK, expiredStored := expiresStoredJWKS(t)
	entityID := "https://sub-expired-update.example"

	app := setupJwksUpdateTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(expiredStored),
	})

	newKeys := jwksWithKid(t, "new-kid")
	signed := signJWKSet(t, expiredSK, newKeys, entityID, entityID)

	req := httptest.NewRequest("POST", "/jwks-update", bytes.NewReader(signed))
	req.Header.Set("Content-Type", oidfedconst.ContentTypeJWKS)
	resp, body := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assertErrBody(t, body, "invalid_client")
	assert.Contains(t, string(body), "expired",
		"rejection must be due to the signing key being expired")
}

// TestJwksUpdateTriggerEndpoint_ExpiredStoredKey_Returns401InvalidClient
// verifies that the trigger endpoint rejects an Entity Configuration whose
// signature was made with an expired (but still stored) federation key.
func TestJwksUpdateTriggerEndpoint_ExpiredStoredKey_Returns401InvalidClient(t *testing.T) {
	expiredSK, expiredStored := expiresStoredJWKS(t)

	var entityID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
		// EC is time-valid but signed with the expired stored key.
		_, _ = w.Write(signEntityConfiguration(t, expiredSK, entityID))
	}))
	t.Cleanup(srv.Close)
	entityID = srv.URL

	app := setupJwksUpdateTriggerTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(expiredStored),
	})

	req := httptest.NewRequest(
		"POST", "/jwks-trigger",
		bytes.NewReader([]byte("sub="+entityID)),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, body := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assertErrBody(t, body, "invalid_client")
	assert.Contains(t, string(body), "expired",
		"rejection must be due to the signing key being expired")
}

// TestJwksUpdateTriggerEndpoint_ExpiredEC_Returns502ServerError verifies that
// an Entity Configuration that is already expired is rejected even though its
// signature would verify against a valid stored key.
func TestJwksUpdateTriggerEndpoint_ExpiredEC_Returns502ServerError(t *testing.T) {
	sk := rsaKey(t)
	storedKeys := pubJWKS(t, sk)

	var entityID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
		_, _ = w.Write(signEntityConfigurationExp(t, sk, entityID, time.Now().Add(-time.Hour)))
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

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assertErrBody(t, body, "server_error")
	assert.Contains(t, string(body), "time-valid",
		"rejection must be due to the EC being expired")
}

// TestJwksUpdateEndpoint_ValidStoredKey_Returns204 verifies that a signed JWK
// Set made with a non-expired stored key is still accepted (positive control
// for the expired-key rejection).
func TestJwksUpdateEndpoint_ValidStoredKey_Returns204(t *testing.T) {
	sk := rsaKey(t)
	storedKeys := pubJWKS(t, sk)
	entityID := "https://sub-valid-update.example"

	app := setupJwksUpdateTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.NewJWKS(storedKeys),
	})

	newKeys := jwksWithKid(t, "rotated-key")
	signed := signJWKSet(t, sk, newKeys, entityID, entityID)

	req := httptest.NewRequest("POST", "/jwks-update", bytes.NewReader(signed))
	req.Header.Set("Content-Type", oidfedconst.ContentTypeJWKS)
	resp, respBody := doRequestRaw(t, app, req)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode,
		"body: %s", string(respBody))
}

// TestJwksUpdateTriggerEndpoint_ValidKeys_Rotates verifies that an EC signed
// with a non-expired stored key, embedding a rotated key, is accepted and
// updates the stored JWKS (positive control for the expired-key/EC rejection).
func TestJwksUpdateTriggerEndpoint_ValidKeys_Rotates(t *testing.T) {
	sk := rsaKey(t)
	storedKeys := pubJWKS(t, sk)

	var entityID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
		_, _ = w.Write(signEntityConfigurationFull(
			t, sk, entityID, time.Now().Add(time.Hour), jwksWithKid(t, "rotated-key"),
		))
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

	assert.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))
	assert.Contains(t, string(body), `"changed":true`)
}

// TestJwksUpdateTriggerEndpoint_WrongIssSub_Returns502ServerError verifies
// that an Entity Configuration whose signature and exp are valid but whose
// `iss`/`sub` do not match the expected subordinate entity ID is rejected.
func TestJwksUpdateTriggerEndpoint_WrongIssSub_Returns502ServerError(t *testing.T) {
	sk := rsaKey(t)
	storedKeys := pubJWKS(t, sk)

	var entityID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", oidfedconst.ContentTypeEntityStatement)
		// Valid signature (signed with the stored key) + future exp, but the
		// EC claims an issuer that is not the expected subordinate.
		_, _ = w.Write(signECConfig(
			t, sk, "https://evil.example", entityID, time.Now().Add(time.Hour),
			jwksWithKid(t, "new-key"),
		))
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

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assertErrBody(t, body, "server_error")
	assert.Contains(t, string(body), "do not match",
		"rejection must be due to the issuer/subject mismatch")
}
