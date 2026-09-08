package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// mockLighthouseController is a minimal LighthouseController stub for tests.
type mockLighthouseController struct{}

func (mockLighthouseController) TAJWKSRefresher() *oidfed.TAJWKSRefresher { return nil }
func (mockLighthouseController) SubordinateJWKSRefresher() *oidfed.SubordinateJWKSRefresher {
	return nil
}
func (mockLighthouseController) SyncTrustAnchor(_ string)     {}
func (mockLighthouseController) RemoveTrustAnchor(_ string)   {}
func (mockLighthouseController) ReloadEndpointsFromDB() error { return nil }

// setupTrustAnchorApp creates a Fiber app with the trust anchor admin endpoints
// backed by an in-memory SQLite database.
func setupTrustAnchorApp(t *testing.T) (*fiber.App, model.TrustAnchorStore) {
	t.Helper()
	store := newSubordinateTestStorage(t)
	return setupTrustAnchorAppWithStorage(t, store)
}

func setupTrustAnchorAppWithStorage(t *testing.T, store *storage.Storage) (*fiber.App, model.TrustAnchorStore) {
	t.Helper()
	app := fiber.New()
	taStore := storage.NewTrustAnchorStorage(store.DB())
	registerTrustAnchors(app.Group("/api/v1/admin"), taStore, mockLighthouseController{})
	return app, taStore
}

func TestTrustAnchorGetUpdateDelete(t *testing.T) {
	t.Parallel()

	app, store := setupTrustAnchorApp(t)

	created, err := store.Create(model.AddTrustAnchor{
		EntityID:         "https://ta.example.org",
		EnableJWKSUpdate: true,
		KeyPollInterval:  300,
	})
	if err != nil {
		t.Fatalf("failed to seed trust anchor: %v", err)
	}

	t.Run(
		"GetByNumericID", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/admin/trust-anchors/%d", created.ID), http.NoBody)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var ta model.TrustAnchor
			if err := json.Unmarshal(body, &ta); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if ta.ID != created.ID {
				t.Errorf("expected id %d, got %d", created.ID, ta.ID)
			}
			if ta.EntityID != created.EntityID {
				t.Errorf("expected entity_id %q, got %q", created.EntityID, ta.EntityID)
			}
		},
	)

	t.Run(
		"GetNotFound", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/admin/trust-anchors/99999", http.NoBody)
			resp, body := doRequest(t, app, req)
			assertStatus(t, resp, body, http.StatusNotFound)
		},
	)

	t.Run(
		"UpdateByNumericID", func(t *testing.T) {
			body := `{"enable_jwks_update": false, "key_poll_interval": 600}`
			req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/admin/trust-anchors/%d", created.ID), strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, http.StatusOK)

			updated, err := store.Get(fmt.Sprintf("%d", created.ID))
			if err != nil {
				t.Fatalf("failed to reload trust anchor: %v", err)
			}
			if updated.EnableJWKSUpdate {
				t.Errorf("expected EnableJWKSUpdate=false, got true")
			}
			if updated.KeyPollInterval != 600 {
				t.Errorf("expected KeyPollInterval=600, got %d", updated.KeyPollInterval)
			}
		},
	)

	t.Run(
		"DeleteByNumericID", func(t *testing.T) {
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/trust-anchors/%d", created.ID), http.NoBody)
			resp, body := doRequest(t, app, req)
			assertStatus(t, resp, body, http.StatusNoContent)

			if _, err := store.Get(fmt.Sprintf("%d", created.ID)); err == nil {
				t.Errorf("expected trust anchor to be deleted, but Get succeeded")
			}
		},
	)
}

// TestOpenAPIAuthTrustAnchors verifies the OpenAPI document describes endpoint
// auth trust anchors as entity IDs (strings), not integers.
func TestOpenAPIAuthTrustAnchors(t *testing.T) {
	t.Parallel()

	raw, err := assets.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("failed to parse openapi.yaml: %v", err)
	}

	paths, _ := doc["paths"].(map[string]any)

	// /federation-endpoints/{type}/auth-trust-anchors request body must be string array.
	setPath, _ := paths["/api/v1/admin/federation-endpoints/{type}/auth-trust-anchors"].(map[string]any)
	put, _ := setPath["put"].(map[string]any)
	reqBody, _ := put["requestBody"].(map[string]any)
	content, _ := reqBody["content"].(map[string]any)
	jsonCfg, _ := content["application/json"].(map[string]any)
	schema, _ := jsonCfg["schema"].(map[string]any)
	if schema["type"] != "array" {
		t.Errorf("expected auth-trust-anchors request body type 'array', got %v", schema["type"])
	}
	items, _ := schema["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("expected auth-trust-anchors items type 'string', got %v", items["type"])
	}

	// AddFederationEndpoint must declare auth_trust_anchors as string array.
	schemas, _ := doc["components"].(map[string]any)
	schemas2, _ := schemas["schemas"].(map[string]any)
	afe, _ := schemas2["AddFederationEndpoint"].(map[string]any)
	props, _ := afe["properties"].(map[string]any)
	authTAs, ok := props["auth_trust_anchors"].(map[string]any)
	if !ok {
		t.Fatalf("expected AddFederationEndpoint property 'auth_trust_anchors', got %v", props)
	}
	if authTAs["type"] != "array" {
		t.Errorf("expected AddFederationEndpoint.auth_trust_anchors type 'array', got %v", authTAs["type"])
	}
	afeItems, _ := authTAs["items"].(map[string]any)
	if afeItems["type"] != "string" {
		t.Errorf("expected AddFederationEndpoint.auth_trust_anchors items type 'string', got %v", afeItems["type"])
	}

	// Trust-anchor paths must use {id}, not {entityID}.
	if _, ok := paths["/api/v1/admin/trust-anchors/{id}"]; !ok {
		t.Errorf("expected trust-anchors path with {id}, got none")
	}
	if _, ok := paths["/api/v1/admin/trust-anchors/{entityID}"]; ok {
		t.Errorf("trust-anchors path should not use {entityID}")
	}
}
