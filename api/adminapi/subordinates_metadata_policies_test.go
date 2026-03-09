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