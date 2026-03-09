package adminapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// setupSubordinateMetadataPoliciesApp creates a Fiber app and registers metadata policies endpoints.
func setupSubordinateMetadataPoliciesApp(t *testing.T) (*fiber.App, model.Backends) {
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
	registerSubordinateMetadataPolicies(app, backends)
	return app, backends
}

// --- GET /subordinates/:subordinateID/metadata-policies TESTS ---

func TestGetSubordinateMetadataPolicies(t *testing.T) {
	t.Run("Success/WithPolicies", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		policy := &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"admin@example.org"},
				},
			},
		}

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://has-policy.example.org",
			},
			MetadataPolicy: policy,
		})
		saved, _ := backends.Subordinates.Get("https://has-policy.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.MetadataPolicies
		json.Unmarshal(body, &result)

		if result.RelyingParty == nil {
			t.Fatalf("Expected RelyingParty policy to be set")
		}
		contacts, ok := result.RelyingParty["contacts"]
		if !ok || contacts["add"] == nil {
			t.Errorf("Failed to retrieve correctly unmarshaled policy: %+v", result)
		}
	})

	t.Run("NoPolicies", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://no-policy.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://no-policy.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("Expected status 404 when policies are missing, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999/metadata-policies", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}

// --- PUT /subordinates/:subordinateID/metadata-policies TESTS ---

func TestPutSubordinateMetadataPolicies(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://put-policy.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://put-policy.example.org")

		body := `{
			"openid_relying_party": {
				"contacts": {
					"add": ["new-admin@example.org"]
				}
			}
		}`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://put-policy.example.org")
		if updated.MetadataPolicy == nil {
			t.Fatalf("Expected MetadataPolicy to be saved in DB, got nil")
		}
		
		rpPol := (*updated.MetadataPolicy).RelyingParty
		contacts, ok := rpPol["contacts"]
		if !ok {
			t.Fatalf("Expected 'contacts' claim in policy")
		}
		addList := contacts["add"].([]any)
		if len(addList) == 0 || addList[0].(string) != "new-admin@example.org" {
			t.Errorf("Expected 'new-admin@example.org' in Add policy, got: %+v", addList)
		}

		// Verify Event logging
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypePolicyUpdated {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected PolicyUpdated event to be logged")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/9999/metadata-policies", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}
// --- POST /subordinates/:subordinateID/metadata-policies TESTS ---

func TestPostSubordinateMetadataPolicies(t *testing.T) {
	t.Run("Success/CopyFromGeneral", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		// Seed a global policy in KV
		globalPolicy := &oidfed.MetadataPolicies{
			OpenIDProvider: oidfed.MetadataPolicy{
				"issuer": oidfed.MetadataPolicyEntry{
					"value": "https://global.op.example.org",
				},
			},
		}
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, globalPolicy)

		// Create a mock record with no policy
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://post-policy.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://post-policy.example.org")

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/metadata-policies", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update copied the global policy
		updated, _ := backends.Subordinates.Get("https://post-policy.example.org")
		if updated.MetadataPolicy == nil {
			t.Fatalf("Expected MetadataPolicy to be saved in DB, got nil")
		}
		
		opPol := (*updated.MetadataPolicy).OpenIDProvider
		if opPol == nil {
			t.Errorf("Expected OpenIDProvider policy to exist")
		}
		
		issuer, ok := opPol["issuer"]
		if !ok || issuer["value"] != "https://global.op.example.org" {
			t.Errorf("Failed to retrieve correctly copied policy: %+v", updated.MetadataPolicy)
		}

		// Verify Event logging
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypePolicyUpdated {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected PolicyUpdated event to be logged")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)

		req := httptest.NewRequest("POST", "/subordinates/9999/metadata-policies", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}

// --- DELETE /subordinates/:subordinateID/metadata-policies TESTS ---

func TestDeleteSubordinateMetadataPolicies(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		// Create a mock record with an existing policy
		initialPolicy := &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"old@example.org"},
				},
			},
		}
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://delete-policy.example.org",
			},
			MetadataPolicy: initialPolicy,
		})
		saved, _ := backends.Subordinates.Get("https://delete-policy.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		// Verify DB update (policy should be nil)
		updated, _ := backends.Subordinates.Get("https://delete-policy.example.org")
		if updated.MetadataPolicy != nil {
			t.Fatalf("Expected MetadataPolicy to be nil after deletion")
		}

		// Verify Event logging
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypePolicyDeleted {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected PolicyDeleted event to be logged")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)

		req := httptest.NewRequest("DELETE", "/subordinates/9999/metadata-policies", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}

// --- GET /subordinates/:subordinateID/metadata-policies/:entityType TESTS ---

func TestGetSubordinateMetadataPolicyByEntityType(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		policy := &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"admin@example.org"},
				},
			},
		}

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://entity-type-get.example.org",
			},
			MetadataPolicy: policy,
		})
		saved, _ := backends.Subordinates.Get("https://entity-type-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.MetadataPolicy
		json.Unmarshal(body, &result)

		if contacts, ok := result["contacts"]; !ok || contacts["add"] == nil {
			t.Errorf("Failed to retrieve entity type policy: %+v", result)
		}
	})

	t.Run("NotFound/Subordinate", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/metadata-policies/openid_relying_party", http.NoBody)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound/EntityType", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-type.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{},
		})
		saved, _ := backends.Subordinates.Get("https://missing-type.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_provider", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 when entity type is missing, got %d", resp.StatusCode)
		}
	})
}

// --- PUT /subordinates/:subordinateID/metadata-policies/:entityType TESTS ---

func TestPutSubordinateMetadataPolicyByEntityType(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://put-type.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"old_claim": oidfed.MetadataPolicyEntry{"value": "old"},
				},
				OpenIDProvider: oidfed.MetadataPolicy{
					"untouched": oidfed.MetadataPolicyEntry{"value": "safe"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://put-type.example.org")

		body := `{
			"new_claim": {
				"value": "new"
			}
		}`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://put-type.example.org")
		rpPol := (*updated.MetadataPolicy).RelyingParty
		opPol := (*updated.MetadataPolicy).OpenIDProvider

		// Verify OP was untouched
		if opPol["untouched"] == nil {
			t.Errorf("Expected OpenIDProvider policy to remain untouched")
		}

		// Verify RP was entirely replaced
		if rpPol["old_claim"] != nil {
			t.Errorf("Expected old RP claim to be replaced and deleted")
		}
		if newClaim, ok := rpPol["new_claim"]; !ok || newClaim["value"] != "new" {
			t.Errorf("Expected new RP claim to be set")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-put-type.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-put-type.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}
