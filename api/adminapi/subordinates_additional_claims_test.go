package adminapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// setupSubordinateAdditionalClaimsApp creates a Fiber app and registers the endpoints.
func setupSubordinateAdditionalClaimsApp(t *testing.T) (*fiber.App, model.Backends) {
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
	registerSubordinateAdditionalClaims(app, backends)
	return app, backends
}

// --- GET, PUT, POST /subordinates/:subordinateID/additional-claims TESTS ---

func TestSubordinateAdditionalClaimsAll(t *testing.T) {
	t.Run("GET Success", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-get.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "custom_claim", Value: "foo"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://claims-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result []model.SubordinateAdditionalClaim
		json.Unmarshal(body, &result)

		if len(result) == 0 || result[0].Claim != "custom_claim" || result[0].Value != "foo" {
			t.Errorf("Failed to retrieve additional claims: %+v", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		app, _ := setupSubordinateAdditionalClaimsApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/additional-claims", http.NoBody)
		resp, _ := app.Test(req, -1)
		
		// The ListAdditionalClaims endpoint returns an empty array when the subordinate has no claims
		// or doesn't exist, so we expect a 200 instead of a 404 here.
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "[]" {
			t.Errorf("Expected empty JSON array '[]', got %s", string(body))
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://claims-put.example.org")

		claimsList := []model.SubordinateAdditionalClaim{
			{Claim: "new_claim_1", Value: "val1"},
			{Claim: "new_claim_2", Value: "val2"},
		}
		data, _ := json.Marshal(claimsList)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		updated, _ := backends.Subordinates.Get("https://claims-put.example.org")
		if len(updated.SubordinateAdditionalClaims) != 2 {
			t.Errorf("Expected 2 additional claims, got %d", len(updated.SubordinateAdditionalClaims))
		}

		// Verify Event
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeClaimsUpdated {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ClaimsUpdated event")
		}
	})

	t.Run("PUT InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-bad-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://claims-bad-put.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("POST Success", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-post.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "old_claim", Value: "old_val"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://claims-post.example.org")

		body := `{"claim": "new_claim", "value": "new_val", "crit": true}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(b))
		}

		updated, _ := backends.Subordinates.Get("https://claims-post.example.org")
		if len(updated.SubordinateAdditionalClaims) != 2 {
			t.Errorf("Expected exactly 2 claims after POST merge, got %d", len(updated.SubordinateAdditionalClaims))
		}
		
		// Verify Event
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeClaimsUpdated {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ClaimsUpdated event")
		}
	})
	
	t.Run("POST InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-bad-post.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://claims-bad-post.example.org")

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}
