package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// mockFedEntity implements oidfed.FederationEntity just enough for the statement generation to not panic
type mockFedEntity struct{}

func (mockFedEntity) EntityID() string {
	return "https://lighthouse.example.org"
}

func (mockFedEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return nil, nil
}
func (mockFedEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (mockFedEntity) SignEntityStatement(_ oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (mockFedEntity) SignEntityStatementWithHeaders(_ oidfed.EntityStatementPayload, _ jws.Headers) ([]byte, error) {
	return nil, nil
}

func setupSubordinateStatementApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	backends := model.Backends{
		Subordinates:      store.SubordinateStorage(),
		SubordinateEvents: store.SubordinateEventsStorage(),
		KV:                store.KeyValue(),
		Transaction: func(fn model.TransactionFunc) error {
			return fn(&model.Backends{
				Subordinates:      store.SubordinateStorage(),
				SubordinateEvents: store.SubordinateEventsStorage(),
				KV:                store.KeyValue(),
			})
		},
	}

	app := fiber.New()

	// registerSubordinateStatement takes the router, subordinate backend, KV, and the FederationEntity.
	g := app.Group("/subordinates/:subordinateID/statement")
	g.Get("/", handleGetSubordinateStatement(backends.Subordinates, backends.KV, mockFedEntity{}))

	return app, backends
}

// --- GET /subordinates/:subordinateID/statement TESTS ---

func TestSubordinateStatement(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateStatementApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://statement.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://statement.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/statement", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["iss"] != "https://lighthouse.example.org" {
			t.Errorf("Expected issuer to be lighthouse, got %v", result["iss"])
		}
		if result["sub"] != "https://statement.example.org" {
			t.Errorf("Expected subject to be subordinate entity ID, got %v", result["sub"])
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateStatementApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999/statement", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusNotFound)
	})

	t.Run("CriticalExtensions", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateStatementApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://crit-ext.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "custom_claim", Value: "custom_val", Crit: true},
				{Claim: "normal_claim", Value: "normal_val", Crit: false},
			},
		})
		saved, err := backends.Subordinates.Get("https://crit-ext.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/statement", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify critical extensions contains only the crit=true claim
		critExts, ok := result["crit"].([]any)
		if !ok || len(critExts) != 1 {
			t.Fatalf("Expected crit to have 1 entry, got %v", result["crit"])
		}
		if critExts[0] != "custom_claim" {
			t.Errorf("Expected crit[0] = 'custom_claim', got %v", critExts[0])
		}

		// Verify both claims appear in the extra payload
		if result["custom_claim"] != "custom_val" {
			t.Errorf("Expected custom_claim in extra, got %v", result["custom_claim"])
		}
		if result["normal_claim"] != "normal_val" {
			t.Errorf("Expected normal_claim in extra, got %v", result["normal_claim"])
		}
	})

	t.Run("LifetimeDefault", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateStatementApp(t)

		// No lifetime set in KV — GetSubordinateStatementLifetime returns default with no error.
		// This covers the "missing config" path, NOT the error fallback.
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://lifetime-default.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://lifetime-default.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/statement", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify iat and exp are set — exp should be iat + default lifetime
		iat, ok1 := result["iat"].(float64)
		exp, ok2 := result["exp"].(float64)
		if !ok1 || !ok2 {
			t.Fatalf("Expected iat and exp as numbers, got iat=%v exp=%v", result["iat"], result["exp"])
		}
		// Default lifetime is 600000 seconds. Verify exp-iat is approximately that.
		diff := exp - iat
		if diff < 599999 || diff > 600001 {
			t.Errorf("Expected exp-iat ≈ 600000 (default), got %.0f", diff)
		}
	})

	t.Run("LifetimeErrorFallback", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateStatementApp(t)

		// Seed invalid JSON into the lifetime KV key so that GetSubordinateStatementLifetime
		// returns an error. This exercises the `if err != nil` fallback at statement.go:50-51.
		if err := backends.KV.Set(
			model.KeyValueScopeSubordinateStatement,
			model.KeyValueKeyLifetime,
			[]byte(`"not-an-integer"`),
		); err != nil {
			t.Fatalf("Failed to seed invalid lifetime: %v", err)
		}

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://lifetime-error.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://lifetime-error.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/statement", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Even with a KV error, the handler should fall back to DefaultSubordinateStatementLifetime
		iat, ok1 := result["iat"].(float64)
		exp, ok2 := result["exp"].(float64)
		if !ok1 || !ok2 {
			t.Fatalf("Expected iat and exp as numbers, got iat=%v exp=%v", result["iat"], result["exp"])
		}
		diff := exp - iat
		if diff < 599999 || diff > 600001 {
			t.Errorf("Expected exp-iat ≈ 600000 (default fallback on error), got %.0f", diff)
		}
	})

	t.Run("FilterExpiredKeys", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateStatementApp(t)

		now := time.Now()
		jwks := jwx.NewJWKS()
		noExp := stmtTestKeyWithKID(t, "no-exp")
		require.NoError(t, jwks.AddKey(noExp))
		future := stmtTestKeyWithKID(t, "future")
		require.NoError(t, future.Set("exp", unixtime.Unixtime{Time: now.Add(time.Hour)}))
		require.NoError(t, jwks.AddKey(future))
		past := stmtTestKeyWithKID(t, "past")
		require.NoError(t, past.Set("exp", unixtime.Unixtime{Time: now.Add(-time.Hour)}))
		require.NoError(t, jwks.AddKey(past))

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://stmt-filter.example.org",
			},
			JWKS: model.JWKS{Keys: jwks},
		})
		saved, err := backends.Subordinates.Get("https://stmt-filter.example.org")
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/statement", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var result map[string]any
		require.NoError(t, json.Unmarshal(body, &result))

		jwksField, ok := result["jwks"].(map[string]any)
		require.True(t, ok, "expected jwks object, got %T", result["jwks"])
		keys, ok := jwksField["keys"].([]any)
		require.True(t, ok, "expected keys array, got %T", jwksField["keys"])
		assert.Len(t, keys, 2, "expired key should be omitted from preview")

		kids := make(map[string]bool)
		for _, k := range keys {
			if entry, ok := k.(map[string]any); ok {
				if kid, ok := entry["kid"].(string); ok {
					kids[kid] = true
				}
			}
		}
		assert.True(t, kids["no-exp"])
		assert.True(t, kids["future"])
		assert.False(t, kids["past"], "expired key must not appear in preview JWKS")
	})
}

// stmtTestKeyWithKID generates an ES256 public JWK with the given kid for the
// statement preview tests.
func stmtTestKeyWithKID(t *testing.T, kid string) jwk.Key {
	t.Helper()
	_, pk, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)
	require.NoError(t, pk.Set(jwk.KeyIDKey, kid))
	return pk
}
