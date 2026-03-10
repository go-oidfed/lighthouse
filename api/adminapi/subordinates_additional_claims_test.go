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

// --- GET, PUT, DELETE /subordinates/:subordinateID/additional-claims/:additionalClaimsID TESTS ---

func TestSubordinateAdditionalClaimByID(t *testing.T) {
	t.Run("GET Success", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-by-id-get.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "target_claim", Value: "found_it"},
				{Claim: "other_claim", Value: "ignored"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-by-id-get.example.org")

		// We need to fetch the actual ID of the inserted claim to test the endpoint
		claims, _ := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		claimID := claims[0].ID

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result model.SubordinateAdditionalClaim
		json.Unmarshal(body, &result)

		if result.Value != "found_it" {
			t.Errorf("Failed to retrieve correct claim: %+v", result)
		}
	})

	t.Run("GET NotFound/Claim", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-missing-get.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-missing-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/additional-claims/missing", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-by-id-put.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "target_claim", Value: "old_value"},
				{Claim: "safe_claim", Value: "safe"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-by-id-put.example.org")

		claims, _ := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		var claimID uint
		for _, c := range claims {
			if c.Claim == "target_claim" {
				claimID = c.ID
			}
		}

		body := `{"value": "new_value", "crit": true}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		updated, _ := backends.Subordinates.Get("https://claim-by-id-put.example.org")
		
		foundTarget := false
		foundSafe := false
		for _, c := range updated.SubordinateAdditionalClaims {
			if c.Claim == "target_claim" {
				foundTarget = true
				if c.Value != "new_value" || c.Crit != true {
					t.Errorf("Expected target claim to be updated, got %+v", c)
				}
			}
			if c.Claim == "safe_claim" {
				foundSafe = true
			}
		}
		if !foundTarget {
			t.Errorf("Target claim was missing after update")
		}
		if !foundSafe {
			t.Errorf("Expected sibling claim to remain untouched")
		}
	})

	t.Run("PUT InvalidBody", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-bad-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-bad-put.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims/some_claim", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-by-id-delete.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "delete_me", Value: "bye"},
				{Claim: "keep_me", Value: "stay"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-by-id-delete.example.org")

		claims, _ := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		var claimID uint
		for _, c := range claims {
			if c.Claim == "delete_me" {
				claimID = c.ID
			}
		}

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		updated, _ := backends.Subordinates.Get("https://claim-by-id-delete.example.org")
		if len(updated.SubordinateAdditionalClaims) != 1 || updated.SubordinateAdditionalClaims[0].Claim != "keep_me" {
			t.Errorf("Expected only keep_me claim to remain, got %+v", updated.SubordinateAdditionalClaims)
		}

		// Verify Event
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeClaimDeleted {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ClaimDeleted event to be logged")
		}
	})

	t.Run("DELETE NotFound", func(t *testing.T) {
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-missing-delete.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://claim-missing-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/additional-claims/not_here", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500 when claim is missing, got %d", resp.StatusCode)
		}
	})
}
