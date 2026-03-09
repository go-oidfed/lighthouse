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

// --- POST /subordinates/:subordinateID/metadata-policies/:entityType TESTS ---

func TestPostSubordinateMetadataPolicyByEntityType(t *testing.T) {
	t.Run("Success/Merge", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://post-type.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"existing_claim": oidfed.MetadataPolicyEntry{"value": "kept"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://post-type.example.org")

		body := `{
			"new_claim": {
				"add": ["merged"]
			}
		}`

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update merged the policies
		updated, _ := backends.Subordinates.Get("https://post-type.example.org")
		rpPol := (*updated.MetadataPolicy).RelyingParty

		// Old claim should still exist
		if existing, ok := rpPol["existing_claim"]; !ok || existing["value"] != "kept" {
			t.Errorf("Expected existing claim to be kept during merge")
		}
		
		// New claim should be added
		if newClaim, ok := rpPol["new_claim"]; !ok || newClaim["add"] == nil {
			t.Errorf("Expected new claim to be merged in")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-post-type.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-post-type.example.org")

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// --- DELETE /subordinates/:subordinateID/metadata-policies/:entityType TESTS ---

func TestDeleteSubordinateMetadataPolicyByEntityType(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://delete-type.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{"value": "delete-me"},
				},
				OpenIDProvider: oidfed.MetadataPolicy{
					"issuer": oidfed.MetadataPolicyEntry{"value": "keep-me"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://delete-type.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://delete-type.example.org")
		
		if (*updated.MetadataPolicy).RelyingParty != nil {
			t.Errorf("Expected RelyingParty to be entirely deleted")
		}
		if (*updated.MetadataPolicy).OpenIDProvider == nil {
			t.Errorf("Expected OpenIDProvider to be safely kept")
		}
	})

	t.Run("NotFound/Subordinate", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)
		req := httptest.NewRequest("DELETE", "/subordinates/9999/metadata-policies/openid_relying_party", http.NoBody)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound/EntityType", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-delete-type.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{},
		})
		saved, _ := backends.Subordinates.Get("https://missing-delete-type.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_provider", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status 204 when trying to delete a missing entity type (idempotent), got %d", resp.StatusCode)
		}
	})
}

// --- GET /subordinates/:subordinateID/metadata-policies/:entityType/:claim TESTS ---

func TestGetSubordinateMetadataPolicyByClaim(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-get.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{
						"add": []any{"admin@example.org"},
					},
					"other": oidfed.MetadataPolicyEntry{"value": "ignored"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.MetadataPolicyEntry
		json.Unmarshal(body, &result)

		if addVal, ok := result["add"]; !ok || addVal.([]any)[0].(string) != "admin@example.org" {
			t.Errorf("Failed to retrieve claim policy correctly: %+v", result)
		}
	})

	t.Run("NotFound/Claim", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-claim.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{}, // exists but empty
			},
		})
		saved, _ := backends.Subordinates.Get("https://missing-claim.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/missing", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 when claim is missing, got %d", resp.StatusCode)
		}
	})
}

// --- PUT /subordinates/:subordinateID/metadata-policies/:entityType/:claim TESTS ---

func TestPutSubordinateMetadataPolicyByClaim(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://put-claim.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{
						"add": []any{"old@example.org"},
					},
					"safe_claim": oidfed.MetadataPolicyEntry{"value": "untouched"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://put-claim.example.org")

		body := `{
			"value": "new_direct_value"
		}`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://put-claim.example.org")
		rpPol := (*updated.MetadataPolicy).RelyingParty

		if rpPol["safe_claim"] == nil {
			t.Errorf("Expected other claims to remain untouched")
		}

		contacts := rpPol["contacts"]
		if contacts["add"] != nil {
			t.Errorf("Expected old operator add to be wiped by PUT replacement")
		}
		if contacts["value"] != "new_direct_value" {
			t.Errorf("Expected new operator value to be set")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-put-claim.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-put-claim.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// --- POST /subordinates/:subordinateID/metadata-policies/:entityType/:claim TESTS ---

func TestPostSubordinateMetadataPolicyByClaim(t *testing.T) {
	t.Run("Success/Merge", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://post-claim.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{
						"add": []any{"old@example.org"},
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://post-claim.example.org")

		body := `{
			"value": "merged_value"
		}`

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://post-claim.example.org")
		contacts := (*updated.MetadataPolicy).RelyingParty["contacts"]

		// POST merges, so both operators should exist in this specific claim
		if contacts["add"] == nil || contacts["add"].([]any)[0].(string) != "old@example.org" {
			t.Errorf("Expected old \"add\" operator to be kept")
		}
		if contacts["value"] != "merged_value" {
			t.Errorf("Expected new \"value\" operator to be merged in")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, _ := setupSubordinateMetadataPoliciesApp(t)
		req := httptest.NewRequest("POST", "/subordinates/1/metadata-policies/openid_relying_party/contacts", strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// --- DELETE /subordinates/:subordinateID/metadata-policies/:entityType/:claim TESTS ---

func TestDeleteSubordinateMetadataPolicyByClaim(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://delete-claim.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"delete_me": oidfed.MetadataPolicyEntry{"value": "bye"},
					"keep_me":   oidfed.MetadataPolicyEntry{"value": "staying"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://delete-claim.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/delete_me", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://delete-claim.example.org")
		rpPol := (*updated.MetadataPolicy).RelyingParty
		
		if _, ok := rpPol["delete_me"]; ok {
			t.Errorf("Expected claim \"delete_me\" to be deleted")
		}
		if _, ok := rpPol["keep_me"]; !ok {
			t.Errorf("Expected claim \"keep_me\" to be retained safely")
		}
	})

	t.Run("NotFound/Claim", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-delete-claim.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{},
			},
		})
		saved, _ := backends.Subordinates.Get("https://missing-delete-claim.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/not_here", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 when claim is missing, got %d", resp.StatusCode)
		}
	})
}

// --- GET /subordinates/:subordinateID/metadata-policies/:entityType/:claim/:operator TESTS ---

func TestGetSubordinateMetadataPolicyByOperator(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://operator-get.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{
						"add": []any{"admin@example.org"},
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://operator-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts/add", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result []any
		json.Unmarshal(body, &result)

		if len(result) == 0 || result[0].(string) != "admin@example.org" {
			t.Errorf("Failed to retrieve operator policy correctly: %+v", result)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-operator.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://missing-operator.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts/add", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 when operator is missing, got %d", resp.StatusCode)
		}
	})
}

// --- PUT & POST /subordinates/:subordinateID/metadata-policies/:entityType/:claim/:operator TESTS ---

func TestPutSubordinateMetadataPolicyByOperator(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://put-operator.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{
						"add": []any{"old@example.org"},
						"value": "untouched",
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://put-operator.example.org")

		body := `["new@example.org"]`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts/add", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://put-operator.example.org")
		contacts := (*updated.MetadataPolicy).RelyingParty["contacts"]

		if contacts["value"] != "untouched" {
			t.Errorf("Expected sibling operators to remain safely untouched")
		}
		
		addArr := contacts["add"].([]any)
		if len(addArr) == 0 || addArr[0].(string) != "new@example.org" {
			t.Errorf("Expected operator data to be fully replaced")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-put-op.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-put-op.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts/add", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// --- DELETE /subordinates/:subordinateID/metadata-policies/:entityType/:claim/:operator TESTS ---

func TestDeleteSubordinateMetadataPolicyByOperator(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://delete-operator.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{
						"delete_me": "gone",
						"keep_me":   "staying",
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://delete-operator.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts/delete_me", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://delete-operator.example.org")
		contacts := (*updated.MetadataPolicy).RelyingParty["contacts"]
		
		if _, ok := contacts["delete_me"]; ok {
			t.Errorf("Expected operator delete_me to be deleted")
		}
		if _, ok := contacts["keep_me"]; !ok {
			t.Errorf("Expected operator keep_me to be safely retained")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, backends := setupSubordinateMetadataPoliciesApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-delete-op.example.org",
			},
			MetadataPolicy: &oidfed.MetadataPolicies{
				RelyingParty: oidfed.MetadataPolicy{
					"contacts": oidfed.MetadataPolicyEntry{},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://missing-delete-op.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata-policies/openid_relying_party/contacts/not_here", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 when operator is missing, got %d", resp.StatusCode)
		}
	})
}

// ============================================================================
// GENERAL METADATA POLICIES TESTS
// ============================================================================

// setupGeneralMetadataPoliciesApp creates a Fiber app and registers general metadata policies endpoints.
func setupGeneralMetadataPoliciesApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	backends := model.Backends{
		KV: store.KeyValue(),
	}

	app := fiber.New()
	registerGeneralMetadataPolicies(app, backends.KV)
	return app, backends
}

// --- GET & PUT /subordinates/metadata-policies TESTS ---

func TestGetGeneralMetadataPolicies(t *testing.T) {
	t.Run("Success/WithPolicies", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		policy := &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"global-admin@example.org"},
				},
			},
		}
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, policy)

		req := httptest.NewRequest("GET", "/subordinates/metadata-policies", http.NoBody)
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
		if !ok || contacts["add"] == nil || contacts["add"].([]any)[0].(string) != "global-admin@example.org" {
			t.Errorf("Failed to retrieve correctly unmarshaled policy: %+v", result)
		}
	})

	t.Run("NoPolicies", func(t *testing.T) {
		app, _ := setupGeneralMetadataPoliciesApp(t)

		req := httptest.NewRequest("GET", "/subordinates/metadata-policies", http.NoBody)
		resp, _ := app.Test(req, -1)

		// General policies behave differently than subordinate-specific policies.
		// If no global policy is found in KV, the store returns an empty MetadataPolicies struct,
		// and the handler returns 200 OK with `{}`, not a 404.
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 (returning empty object) when global policies are missing, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "{}" {
			t.Errorf("Expected empty JSON object '{}', got %s", string(body))
		}
	})
}

func TestPutGeneralMetadataPolicies(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		body := `{
			"openid_relying_party": {
				"contacts": {
					"add": ["new-global-admin@example.org"]
				}
			}
		}`

		req := httptest.NewRequest("PUT", "/subordinates/metadata-policies", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		var updated oidfed.MetadataPolicies
		found, _ := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		if !found {
			t.Fatalf("Expected MetadataPolicy to be saved in KV")
		}
		
		rpPol := updated.RelyingParty
		contacts, ok := rpPol["contacts"]
		if !ok {
			t.Fatalf("Expected 'contacts' claim in policy")
		}
		addList := contacts["add"].([]any)
		if len(addList) == 0 || addList[0].(string) != "new-global-admin@example.org" {
			t.Errorf("Expected 'new-global-admin@example.org' in Add policy, got: %+v", addList)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, _ := setupGeneralMetadataPoliciesApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/metadata-policies", strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// --- /subordinates/metadata-policies/:entityType TESTS ---

func TestGeneralMetadataPolicyByEntityType(t *testing.T) {
	t.Run("GET Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		policy := &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"admin@example.org"},
				},
			},
		}
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, policy)

		req := httptest.NewRequest("GET", "/subordinates/metadata-policies/openid_relying_party", http.NoBody)
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

	t.Run("PUT Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"old_claim": oidfed.MetadataPolicyEntry{"value": "old"},
			},
			OpenIDProvider: oidfed.MetadataPolicy{
				"untouched": oidfed.MetadataPolicyEntry{"value": "safe"},
			},
		})

		body := `{"new_claim": {"value": "new"}}`
		req := httptest.NewRequest("PUT", "/subordinates/metadata-policies/openid_relying_party", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		rpPol := updated.RelyingParty
		opPol := updated.OpenIDProvider

		if opPol["untouched"] == nil {
			t.Errorf("Expected OpenIDProvider policy to remain untouched")
		}
		if rpPol["old_claim"] != nil {
			t.Errorf("Expected old RP claim to be replaced")
		}
		if newClaim, ok := rpPol["new_claim"]; !ok || newClaim["value"] != "new" {
			t.Errorf("Expected new RP claim to be set")
		}
	})

	t.Run("POST Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"existing_claim": oidfed.MetadataPolicyEntry{"value": "kept"},
			},
		})

		body := `{"new_claim": {"add": ["merged"]}}`
		req := httptest.NewRequest("POST", "/subordinates/metadata-policies/openid_relying_party", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		rpPol := updated.RelyingParty

		if existing, ok := rpPol["existing_claim"]; !ok || existing["value"] != "kept" {
			t.Errorf("Expected existing claim to be kept")
		}
		if newClaim, ok := rpPol["new_claim"]; !ok || newClaim["add"] == nil {
			t.Errorf("Expected new claim to be merged in")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{"value": "delete-me"},
			},
			OpenIDProvider: oidfed.MetadataPolicy{
				"issuer": oidfed.MetadataPolicyEntry{"value": "keep-me"},
			},
		})

		req := httptest.NewRequest("DELETE", "/subordinates/metadata-policies/openid_relying_party", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		if updated.RelyingParty != nil {
			t.Errorf("Expected RelyingParty to be deleted")
		}
		if updated.OpenIDProvider == nil {
			t.Errorf("Expected OpenIDProvider to be kept")
		}
	})
}

// --- /subordinates/metadata-policies/:entityType/:claim TESTS ---

func TestGeneralMetadataPolicyByClaim(t *testing.T) {
	t.Run("GET Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"admin@example.org"},
				},
			},
		})

		req := httptest.NewRequest("GET", "/subordinates/metadata-policies/openid_relying_party/contacts", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.MetadataPolicyEntry
		json.Unmarshal(body, &result)

		if addVal, ok := result["add"]; !ok || addVal.([]any)[0].(string) != "admin@example.org" {
			t.Errorf("Failed to retrieve claim policy: %+v", result)
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"old@example.org"},
				},
				"safe_claim": oidfed.MetadataPolicyEntry{"value": "untouched"},
			},
		})

		body := `{"value": "new_direct_value"}`
		req := httptest.NewRequest("PUT", "/subordinates/metadata-policies/openid_relying_party/contacts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		rpPol := updated.RelyingParty
		contacts := rpPol["contacts"]

		if contacts["add"] != nil {
			t.Errorf("Expected old operator add to be wiped")
		}
		if contacts["value"] != "new_direct_value" {
			t.Errorf("Expected new operator value to be set")
		}
	})

	t.Run("POST Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"old@example.org"},
				},
			},
		})

		body := `{"value": "merged_value"}`
		req := httptest.NewRequest("POST", "/subordinates/metadata-policies/openid_relying_party/contacts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		contacts := updated.RelyingParty["contacts"]

		if contacts["add"] == nil {
			t.Errorf("Expected old operator to be kept")
		}
		if contacts["value"] != "merged_value" {
			t.Errorf("Expected new operator to be merged in")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"delete_me": oidfed.MetadataPolicyEntry{"value": "bye"},
				"keep_me":   oidfed.MetadataPolicyEntry{"value": "staying"},
			},
		})

		req := httptest.NewRequest("DELETE", "/subordinates/metadata-policies/openid_relying_party/delete_me", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		rpPol := updated.RelyingParty
		
		if _, ok := rpPol["delete_me"]; ok {
			t.Errorf("Expected claim 'delete_me' to be deleted")
		}
		if _, ok := rpPol["keep_me"]; !ok {
			t.Errorf("Expected claim 'keep_me' to be retained")
		}
	})
}

// --- /subordinates/metadata-policies/:entityType/:claim/:operator TESTS ---

func TestGeneralMetadataPolicyByOperator(t *testing.T) {
	t.Run("GET Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"admin@example.org"},
				},
			},
		})

		req := httptest.NewRequest("GET", "/subordinates/metadata-policies/openid_relying_party/contacts/add", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result []any
		json.Unmarshal(body, &result)

		if len(result) == 0 || result[0].(string) != "admin@example.org" {
			t.Errorf("Failed to retrieve operator policy: %+v", result)
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"add": []any{"old@example.org"},
					"value": "untouched",
				},
			},
		})

		body := `["new@example.org"]`
		req := httptest.NewRequest("PUT", "/subordinates/metadata-policies/openid_relying_party/contacts/add", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		contacts := updated.RelyingParty["contacts"]

		if contacts["value"] != "untouched" {
			t.Errorf("Expected sibling operators to remain untouched")
		}
		addArr := contacts["add"].([]any)
		if len(addArr) == 0 || addArr[0].(string) != "new@example.org" {
			t.Errorf("Expected operator data to be fully replaced")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		app, backends := setupGeneralMetadataPoliciesApp(t)

		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &oidfed.MetadataPolicies{
			RelyingParty: oidfed.MetadataPolicy{
				"contacts": oidfed.MetadataPolicyEntry{
					"delete_me": "gone",
					"keep_me":   "staying",
				},
			},
		})

		req := httptest.NewRequest("DELETE", "/subordinates/metadata-policies/openid_relying_party/contacts/delete_me", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		var updated oidfed.MetadataPolicies
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, &updated)
		contacts := updated.RelyingParty["contacts"]
		
		if _, ok := contacts["delete_me"]; ok {
			t.Errorf("Expected operator delete_me to be deleted")
		}
		if _, ok := contacts["keep_me"]; !ok {
			t.Errorf("Expected operator keep_me to be safely retained")
		}
	})
}
