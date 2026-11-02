package adminapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

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
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-get.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "custom_claim", Value: "foo"},
			},
		})
		saved, err := backends.Subordinates.Get("https://claims-get.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)

		var result []model.SubordinateAdditionalClaim
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(result) == 0 || result[0].Claim != "custom_claim" || result[0].Value != "foo" {
			t.Errorf("Failed to retrieve additional claims: %+v", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateAdditionalClaimsApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		// The ListAdditionalClaims endpoint returns an empty array when the subordinate has no claims
		// or doesn't exist, so we expect a 200 instead of a 404 here.
		assertStatus(t, resp, body, http.StatusOK)

		if string(body) != "[]" {
			t.Errorf("Expected empty JSON array '[]', got %s", string(body))
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-put.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://claims-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		claimsList := []model.SubordinateAdditionalClaim{
			{Claim: "new_claim_1", Value: "val1"},
			{Claim: "new_claim_2", Value: "val2"},
		}
		data, err := json.Marshal(claimsList)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusOK)

		updated, err := backends.Subordinates.Get("https://claims-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if len(updated.SubordinateAdditionalClaims) != 2 {
			t.Errorf("Expected 2 additional claims, got %d", len(updated.SubordinateAdditionalClaims))
		}

		// Verify Event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
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
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-bad-put.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://claims-bad-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("POST Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-post.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "old_claim", Value: "old_val"},
			},
		})
		saved, err := backends.Subordinates.Get("https://claims-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		body := `{"claim": "new_claim", "value": "new_val", "crit": true}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusCreated)

		updated, err := backends.Subordinates.Get("https://claims-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if len(updated.SubordinateAdditionalClaims) != 2 {
			t.Errorf("Expected exactly 2 claims after POST merge, got %d", len(updated.SubordinateAdditionalClaims))
		}

		// Verify Event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
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
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-bad-post.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://claims-bad-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("POST Duplicate/Conflict", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claims-dup-post.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "existing_claim", Value: "val1"},
			},
		})
		saved, err := backends.Subordinates.Get("https://claims-dup-post.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		// POST with same claim name → should return 409 Conflict
		body := `{"claim": "existing_claim", "value": "val2"}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/additional-claims", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)
		assertErrorResponse(t, resp, respBody, http.StatusConflict, "invalid_request")
	})

	t.Run("POST NotFound/Subordinate", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateAdditionalClaimsApp(t)

		body := `{"claim": "test", "value": "test"}`
		req := httptest.NewRequest("POST", "/subordinates/9999/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)
		assertErrorResponse(t, resp, respBody, http.StatusNotFound, "not_found")
	})
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/additional-claims/:additionalClaimsID TESTS ---

func TestSubordinateAdditionalClaimByID(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
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
		saved, err := backends.Subordinates.Get("https://claim-by-id-get.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		// We need to fetch the actual ID of the inserted claim to test the endpoint
		claims, err := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		if err != nil {
			t.Fatalf("Failed to list additional claims: %v", err)
		}
		claimID := claims[0].ID

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)

		var result model.SubordinateAdditionalClaim
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result.Value != "found_it" {
			t.Errorf("Failed to retrieve correct claim: %+v", result)
		}
	})

	t.Run("GET NotFound/Claim", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-missing-get.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://claim-missing-get.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/additional-claims/missing", saved.ID), http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusNotFound)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
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
		saved, err := backends.Subordinates.Get("https://claim-by-id-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		claims, err := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		if err != nil {
			t.Fatalf("Failed to list additional claims: %v", err)
		}
		var claimID uint
		for _, c := range claims {
			if c.Claim == "target_claim" {
				claimID = c.ID
			}
		}

		body := `{"claim": "target_claim", "value": "new_value", "crit": true}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusOK)

		updated, err := backends.Subordinates.Get("https://claim-by-id-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

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
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-bad-put.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://claim-bad-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims/some_claim", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("PUT MissingClaim", func(t *testing.T) {
		// claim is a required identity field and is never derived from the
		// row id.
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-missing-put.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "existing_claim", Value: "old"},
			},
		})
		saved, err := backends.Subordinates.Get("https://claim-missing-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		claims, err := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		if err != nil {
			t.Fatalf("Failed to list claims: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claims[0].ID), strings.NewReader(`{"value":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)
		assertStatus(t, resp, bodyBytes, http.StatusBadRequest)
	})

	t.Run("PUT Duplicate/Conflict", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-dup-put.example.org",
			},
			SubordinateAdditionalClaims: []model.SubordinateAdditionalClaim{
				{Claim: "claim_a", Value: "val_a"},
				{Claim: "claim_b", Value: "val_b"},
			},
		})
		saved, err := backends.Subordinates.Get("https://claim-dup-put.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		claims, err := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		if err != nil {
			t.Fatalf("Failed to list claims: %v", err)
		}
		// Find claim_b's ID, then try to rename it to "claim_a" which already exists
		var claimBID uint
		for _, c := range claims {
			if c.Claim == "claim_b" {
				claimBID = c.ID
			}
		}

		body := `{"claim": "claim_a", "value": "new_val"}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimBID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)
		assertErrorResponse(t, resp, respBody, http.StatusConflict, "invalid_request")
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
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
		saved, err := backends.Subordinates.Get("https://claim-by-id-delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		claims, err := backends.Subordinates.ListAdditionalClaims(fmt.Sprintf("%d", saved.ID))
		if err != nil {
			t.Fatalf("Failed to list additional claims: %v", err)
		}
		var claimID uint
		for _, c := range claims {
			if c.Claim == "delete_me" {
				claimID = c.ID
			}
		}

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/additional-claims/%d", saved.ID, claimID), http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusNoContent)

		updated, err := backends.Subordinates.Get("https://claim-by-id-delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if len(updated.SubordinateAdditionalClaims) != 1 || updated.SubordinateAdditionalClaims[0].Claim != "keep_me" {
			t.Errorf("Expected only keep_me claim to remain, got %+v", updated.SubordinateAdditionalClaims)
		}

		// Verify Event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
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
		t.Parallel()
		app, backends := setupSubordinateAdditionalClaimsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://claim-missing-delete.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://claim-missing-delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/additional-claims/not_here", saved.ID), http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusNotFound)
	})
}

// ============================================================================
// GENERAL ADDITIONAL CLAIMS TESTS
// ============================================================================

func setupGeneralAdditionalClaimsApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	backends := model.Backends{
		KV: store.KeyValue(),
	}

	app := fiber.New()
	registerGeneralAdditionalClaims(app, backends.KV)
	return app, backends
}

// --- GET, PUT, POST /subordinates/additional-claims TESTS ---

func TestGeneralAdditionalClaimsAll(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "custom_global", Value: "bar", Crit: false},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		req := httptest.NewRequest("GET", "/subordinates/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)

		var result []generalAdditionalClaim
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(result) == 0 || result[0].Claim != "custom_global" || result[0].Value != "bar" {
			t.Errorf("Failed to retrieve general additional claims: %+v", result)
		}
	})

	t.Run("GET NoClaims", func(t *testing.T) {
		t.Parallel()
		app, _ := setupGeneralAdditionalClaimsApp(t)
		req := httptest.NewRequest("GET", "/subordinates/additional-claims", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)

		if string(body) != "[]" {
			t.Errorf("Expected empty JSON object, got %s", string(body))
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "old_global", Value: "old_val", Crit: false},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		body := `[
			{"claim": "new_global_1", "value": "val1", "crit": false},
			{"claim": "new_global_2", "value": "val2", "crit": true}
		]`

		req := httptest.NewRequest("PUT", "/subordinates/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusOK)

		var updated []generalAdditionalClaim
		if _, err := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, &updated); err != nil {
			t.Fatalf("Failed to get KV value: %v", err)
		}

		if len(updated) != 2 {
			t.Errorf("Expected exactly 2 claims after PUT replacement, got %d", len(updated))
		}
		if updated[0].Claim == "old_global" || updated[1].Claim == "old_global" {
			t.Errorf("Expected old global claim to be completely replaced")
		}
		if updated[1].Claim != "new_global_2" || !updated[1].Crit {
			t.Errorf("Expected new global claim 2 to be correctly set")
		}
	})

	t.Run("POST Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "old_global", Value: "old_val", Crit: false},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		body := `{"claim": "merged_global", "value": "merged_val", "crit": true}`
		req := httptest.NewRequest("POST", "/subordinates/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusCreated)

		var updated []generalAdditionalClaim
		if _, err := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, &updated); err != nil {
			t.Fatalf("Failed to get KV value: %v", err)
		}

		if len(updated) != 2 {
			t.Errorf("Expected 2 claims after POST merge, got %d", len(updated))
		}
		if updated[0].Claim != "old_global" {
			t.Errorf("Expected old global claim to be kept")
		}
		if updated[1].Claim != "merged_global" || updated[1].Value != "merged_val" {
			t.Errorf("Expected new global claim to be merged in")
		}
	})
}

// --- GET, PUT, DELETE /subordinates/additional-claims/:additionalClaimsID TESTS ---

func TestGeneralAdditionalClaimByID(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "target_global", Value: "found_global", Crit: false},
			{ID: 2, Claim: "other_global", Value: "ignored_global", Crit: true},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		// Note: The global endpoints use the integer ID in the URL, not the string claim name!
		req := httptest.NewRequest("GET", "/subordinates/additional-claims/1", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, body, http.StatusOK)

		var result generalAdditionalClaim
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result.Value != "found_global" {
			t.Errorf("Failed to retrieve correct global claim: %+v", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, []generalAdditionalClaim{}); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		req := httptest.NewRequest("GET", "/subordinates/additional-claims/999", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		assertStatus(t, resp, bodyBytes, http.StatusNotFound)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "target_global", Value: "old_val", Crit: false},
			{ID: 2, Claim: "safe_global", Value: "safe", Crit: true},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		body := `{"claim": "target_global", "value": "new_global_val", "crit": true}`
		req := httptest.NewRequest("PUT", "/subordinates/additional-claims/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusOK)

		var updated []generalAdditionalClaim
		if _, err := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, &updated); err != nil {
			t.Fatalf("Failed to get KV value: %v", err)
		}

		if len(updated) != 2 {
			t.Fatalf("Expected 2 claims to remain")
		}
		if updated[0].Value != "new_global_val" || !updated[0].Crit {
			t.Errorf("Expected target global claim to be updated, got %+v", updated[0])
		}
		if updated[1].Claim != "safe_global" {
			t.Errorf("Expected sibling global claim to remain untouched")
		}
	})

	t.Run("PUT Duplicate/Conflict", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "claim_x", Value: "val_x", Crit: false},
			{ID: 2, Claim: "claim_y", Value: "val_y", Crit: true},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		// Try renaming claim_y (ID=2) to "claim_x" which already exists
		body := `{"claim": "claim_x", "value": "new_val"}`
		req := httptest.NewRequest("PUT", "/subordinates/additional-claims/2", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)
		assertErrorResponse(t, resp, respBody, http.StatusConflict, "invalid_request")
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralAdditionalClaimsApp(t)

		claimsList := []generalAdditionalClaim{
			{ID: 1, Claim: "delete_global", Value: "bye", Crit: false},
			{ID: 2, Claim: "keep_global", Value: "stay", Crit: true},
		}
		if err := backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, claimsList); err != nil {
			t.Fatalf("Failed to set KV value: %v", err)
		}

		req := httptest.NewRequest("DELETE", "/subordinates/additional-claims/1", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)

		requireStatus(t, resp, bodyBytes, http.StatusNoContent)

		var updated []generalAdditionalClaim
		if _, err := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyAdditionalClaims, &updated); err != nil {
			t.Fatalf("Failed to get KV value: %v", err)
		}

		if len(updated) != 1 || updated[0].Claim != "keep_global" {
			t.Errorf("Expected only keep_global to remain, got %+v", updated)
		}
	})
}
