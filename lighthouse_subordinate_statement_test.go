package lighthouse

import (
	"io"
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
	"github.com/go-oidfed/lib/cache"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/go-oidfed/lib/unixtime"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// testKeyWithKID generates an ES256 public JWK with the given kid.
func testKeyWithKID(t *testing.T, kid string) jwk.Key {
	t.Helper()
	_, pk, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)
	require.NoError(t, pk.Set(jwk.KeyIDKey, kid))
	return pk
}

// newMinimalLightHouse builds a LightHouse with just enough wired up for
// CreateSubordinateStatement to work (KV storage + FederationEntity.EntityID).
func newMinimalLightHouse(t *testing.T) *LightHouse {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	return &LightHouse{
		FederationEntity: stubFedEntity{},
		storages: model.Backends{
			KV: store.KeyValue(),
		},
	}
}

type stubFedEntity struct{}

func (stubFedEntity) EntityID() string { return "https://lighthouse.example.org" }
func (stubFedEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return nil, nil
}
func (stubFedEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (stubFedEntity) SignEntityStatement(oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (stubFedEntity) SignEntityStatementWithHeaders(oidfed.EntityStatementPayload, jws.Headers) ([]byte, error) {
	return nil, nil
}

// TestCreateSubordinateStatement_FilterExpiredKeys verifies that expired keys
// are omitted from the published JWKS while non-expired and no-exp keys are
// kept.
func TestCreateSubordinateStatement_FilterExpiredKeys(t *testing.T) {
	now := time.Now()
	fed := newMinimalLightHouse(t)

	jwks := jwx.NewJWKS()
	noExp := testKeyWithKID(t, "no-exp")
	require.NoError(t, jwks.AddKey(noExp))
	future := testKeyWithKID(t, "future")
	require.NoError(t, future.Set("exp", unixtime.Unixtime{Time: now.Add(time.Hour)}))
	require.NoError(t, jwks.AddKey(future))
	past := testKeyWithKID(t, "past")
	require.NoError(t, past.Set("exp", unixtime.Unixtime{Time: now.Add(-time.Hour)}))
	require.NoError(t, jwks.AddKey(past))

	sub := &model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: "https://sub.example.org",
		},
		JWKS: model.JWKS{Keys: jwks},
	}

	payload := fed.CreateSubordinateStatement(sub)

	assert.Equal(t, 2, payload.JWKS.Len(), "expired key should be omitted")
	kids := make(map[string]bool)
	for _, k := range payload.JWKS.All() {
		if kid, ok := k.KeyID(); ok {
			kids[kid] = true
		}
	}
	assert.True(t, kids["no-exp"])
	assert.True(t, kids["future"])
	assert.False(t, kids["past"], "expired key must not appear in published JWKS")
}

// TestCreateSubordinateStatement_ExpCapToMaxKeyExp verifies that the statement
// ExpiresAt is capped to the maximal key expiration when it would otherwise
// outlive all published keys, and that no cap is applied when no published key
// has an exp.
func TestCreateSubordinateStatement_ExpCapToMaxKeyExp(t *testing.T) {
	now := time.Now()
	fed := newMinimalLightHouse(t)

	t.Run("capped to max key exp", func(t *testing.T) {
		jwks := jwx.NewJWKS()
		k := testKeyWithKID(t, "short-lived")
		keyExp := now.Add(5 * time.Minute)
		require.NoError(t, k.Set("exp", unixtime.Unixtime{Time: keyExp}))
		require.NoError(t, jwks.AddKey(k))

		sub := &model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://sub-cap.example.org"},
			JWKS:                 model.JWKS{Keys: jwks},
		}
		payload := fed.CreateSubordinateStatement(sub)

		// Default lifetime is 600000s; the statement exp must be capped to ~5m.
		assert.True(t, payload.ExpiresAt.Time.Before(now.Add(storage.DefaultSubordinateStatementLifetime)))
		assert.WithinDuration(t, keyExp, payload.ExpiresAt.Time, 2*time.Second)
	})

	t.Run("no cap when keys have no exp", func(t *testing.T) {
		jwks := jwx.NewJWKS()
		require.NoError(t, jwks.AddKey(testKeyWithKID(t, "no-exp")))

		sub := &model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://sub-nocap.example.org"},
			JWKS:                 model.JWKS{Keys: jwks},
		}
		payload := fed.CreateSubordinateStatement(sub)

		assert.WithinDuration(
			t,
			now.Add(storage.DefaultSubordinateStatementLifetime),
			payload.ExpiresAt.Time,
			2*time.Second,
			"exp should remain at now + default lifetime when no keys have exp",
		)
	})
}

// TestCreateSubordinateStatement_AllExpiredEmptyJWKS verifies that when every
// key is expired the published JWKS is empty (and the statement is still
// issued, not errored).
func TestCreateSubordinateStatement_AllExpiredEmptyJWKS(t *testing.T) {
	now := time.Now()
	fed := newMinimalLightHouse(t)

	jwks := jwx.NewJWKS()
	k := testKeyWithKID(t, "expired")
	require.NoError(t, k.Set("exp", unixtime.Unixtime{Time: now.Add(-time.Hour)}))
	require.NoError(t, jwks.AddKey(k))

	sub := &model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{EntityID: "https://sub-empty.example.org"},
		JWKS:                 model.JWKS{Keys: jwks},
	}
	payload := fed.CreateSubordinateStatement(sub)

	assert.Equal(t, 0, payload.JWKS.Len(), "all-expired set should publish empty JWKS")
	assert.Equal(t, "https://sub-empty.example.org", payload.Subject, "statement is still issued")
}

// TestSubordinateStatementCacheTTL verifies the TTL computation for cached
// subordinate statements, including the min-key-exp cap that guarantees no
// expired key is served from cache.
func TestSubordinateStatementCacheTTL(t *testing.T) {
	now := time.Now()

	t.Run("no keys with exp — capped to statement exp", func(t *testing.T) {
		payload := oidfed.EntityStatementPayload{
			ExpiresAt: unixtime.Unixtime{Time: now.Add(time.Hour)},
			JWKS:      jwx.NewJWKS(),
		}
		ttl := subordinateStatementCacheTTL(payload)
		assert.InDelta(t, time.Until(now.Add(time.Hour).Add(-time.Minute)), ttl, float64(2*time.Second))
	})

	t.Run("min key exp caps TTL below statement exp", func(t *testing.T) {
		jwks := jwx.NewJWKS()
		k := testKeyWithKID(t, "short")
		keyExp := now.Add(5 * time.Minute)
		require.NoError(t, k.Set("exp", unixtime.Unixtime{Time: keyExp}))
		require.NoError(t, jwks.AddKey(k))

		payload := oidfed.EntityStatementPayload{
			ExpiresAt: unixtime.Unixtime{Time: now.Add(time.Hour)},
			JWKS:      jwks,
		}
		ttl := subordinateStatementCacheTTL(payload)
		assert.InDelta(t, time.Until(keyExp), ttl, float64(2*time.Second))
	})

	t.Run("empty JWKS — capped to statement exp only", func(t *testing.T) {
		payload := oidfed.EntityStatementPayload{
			ExpiresAt: unixtime.Unixtime{Time: now.Add(30 * time.Minute)},
			JWKS:      jwx.NewJWKS(),
		}
		ttl := subordinateStatementCacheTTL(payload)
		assert.InDelta(t, time.Until(now.Add(30*time.Minute).Add(-time.Minute)), ttl, float64(2*time.Second))
	})

	t.Run("key exp at now — TTL non-positive (skip caching)", func(t *testing.T) {
		jwks := jwx.NewJWKS()
		k := testKeyWithKID(t, "boundary")
		require.NoError(t, k.Set("exp", unixtime.Unixtime{Time: now}))
		require.NoError(t, jwks.AddKey(k))

		payload := oidfed.EntityStatementPayload{
			ExpiresAt: unixtime.Unixtime{Time: now.Add(time.Hour)},
			JWKS:      jwks,
		}
		ttl := subordinateStatementCacheTTL(payload)
		assert.LessOrEqual(t, ttl, time.Duration(0), "TTL should be non-positive when a key exp is at or near now")
	})
}

// stubSigningFedEntity is a stubFedEntity whose SignEntityStatement returns a
// deterministic byte slice, so fetch-handler cache tests can verify cache
// hits without a real signer.
type stubSigningFedEntity struct {
	stubFedEntity
	jwt []byte
}

func (e stubSigningFedEntity) SignEntityStatement(oidfed.EntityStatementPayload) ([]byte, error) {
	return e.jwt, nil
}

// setupFetchTestApp builds a minimal LightHouse + Fiber app with the fetch
// endpoint registered, for testing subordinate-statement cache behavior.
func setupFetchTestApp(t *testing.T, sub model.ExtendedSubordinateInfo) (*fiber.App, *LightHouse) {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)

	require.NoError(t, store.SubordinateStorage().Add(sub))

	fed := &LightHouse{
		FederationEntity: stubSigningFedEntity{jwt: []byte("signed-statement-jwt")},
		storages: model.Backends{
			KV:           store.KeyValue(),
			Subordinates: store.SubordinateStorage(),
		},
		fedMetadata: oidfed.FederationEntityMetadata{},
	}

	require.NoError(t, fed.AddFetchEndpoint(
		EndpointConf{Path: "/fetch"},
		store.SubordinateStorage(),
	))

	app := fiber.New()
	app.All("/*", fed.dispatch)
	return app, fed
}

// TestFetchEndpoint_CacheHitMiss verifies that the fetch endpoint caches the
// signed JWT on the first request and serves the cached value on subsequent
// requests. Must NOT use t.Parallel() — operates on the global cache.
func TestFetchEndpoint_CacheHitMiss(t *testing.T) {
	entityID := "https://fetch-cache.example.org"
	cacheKey := internal.SubordinateStatementCacheKey(entityID)
	_ = cache.Delete(cacheKey)
	t.Cleanup(func() { _ = cache.Delete(cacheKey) })

	jwks := jwx.NewJWKS()
	require.NoError(t, jwks.AddKey(testKeyWithKID(t, "key-1")))

	app, _ := setupFetchTestApp(t, model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: entityID,
			Status:   model.StatusActive,
		},
		JWKS: model.JWKS{Keys: jwks},
	})

	// First request: cache miss — handler builds, signs, caches.
	req := httptest.NewRequest("GET", "/fetch?sub="+entityID, nil)
	resp, body1 := doRequestRaw(t, app, req)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "signed-statement-jwt", string(body1))
	assert.Equal(t, oidfedconst.ContentTypeEntityStatement, resp.Header.Get("Content-Type"))

	// Verify the JWT was cached.
	var cached []byte
	set, err := cache.Get(cacheKey, &cached)
	require.NoError(t, err)
	require.True(t, set, "statement should be cached after first fetch")
	assert.Equal(t, "signed-statement-jwt", string(cached))

	// Second request: cache hit — serves cached JWT directly.
	req2 := httptest.NewRequest("GET", "/fetch?sub="+entityID, nil)
	resp2, body2 := doRequestRaw(t, app, req2)
	assert.Equal(t, fiber.StatusOK, resp2.StatusCode)
	assert.Equal(t, "signed-statement-jwt", string(body2))

	// Invalidate and fetch again — should rebuild.
	_ = cache.Delete(cacheKey)
	req3 := httptest.NewRequest("GET", "/fetch?sub="+entityID, nil)
	resp3, body3 := doRequestRaw(t, app, req3)
	assert.Equal(t, fiber.StatusOK, resp3.StatusCode)
	assert.Equal(t, "signed-statement-jwt", string(body3))

	// And the cache should be populated again.
	set, err = cache.Get(cacheKey, &cached)
	require.NoError(t, err)
	require.True(t, set)
}

// TestFetchEndpoint_NonActiveSubordinate verifies that the fetch endpoint does
// not issue a subordinate statement for a blocked, pending or inactive
// subordinate: it returns 404 (indistinguishable from an unknown entity) and
// does not populate the statement cache. Must NOT use t.Parallel() — operates
// on the global cache.
func TestFetchEndpoint_NonActiveSubordinate(t *testing.T) {
	jwks := jwx.NewJWKS()
	require.NoError(t, jwks.AddKey(testKeyWithKID(t, "key-1")))

	statuses := []model.Status{model.StatusBlocked, model.StatusPending, model.StatusInactive}
	for _, status := range statuses {
		status := status
		t.Run(status.String(), func(t *testing.T) {
			entityID := "https://fetch-nonactive.example.org/" + status.String()
			cacheKey := internal.SubordinateStatementCacheKey(entityID)
			_ = cache.Delete(cacheKey)
			t.Cleanup(func() { _ = cache.Delete(cacheKey) })

			app, _ := setupFetchTestApp(t, model.ExtendedSubordinateInfo{
				BasicSubordinateInfo: model.BasicSubordinateInfo{
					EntityID: entityID,
					Status:   status,
				},
				JWKS: model.JWKS{Keys: jwks},
			})

			req := httptest.NewRequest("GET", "/fetch?sub="+entityID, nil)
			resp, body := doRequestRaw(t, app, req)
			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
			assert.NotEqual(t, "signed-statement-jwt", string(body))

			var cached []byte
			set, err := cache.Get(cacheKey, &cached)
			require.NoError(t, err)
			assert.False(t, set, "no statement should be cached for a non-active subordinate")
		})
	}
}

// doRequestRaw executes a request against the app and returns the raw response.
func doRequestRaw(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, body
}
