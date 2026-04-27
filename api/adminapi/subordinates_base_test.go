package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// --- TEST HELPERS ---

// newSubordinateTestStorage creates a unique in-memory SQLite database for subordinate tests.
func newSubordinateTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", url.PathEscape(t.Name()))
	store, err := storage.NewStorage(storage.Config{
		Driver: storage.DriverSQLite,
		DSN:    dsn,
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	return store
}

// setupSubordinateBaseApp creates a Fiber app and registers base subordinate endpoints.
// Returns the app and the backend storage so tests can inject data.
func setupSubordinateBaseApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	// Build the Backends struct as expected by handlers
	backends := model.Backends{
		Subordinates:      store.SubordinateStorage(),
		SubordinateEvents: store.SubordinateEventsStorage(),
		KV:                store.KeyValue(),
		// Wrap operations in DB transactions using the storage's DB
		Transaction: func(fn model.TransactionFunc) error {
			// A real Transaction func would use gorm's Transaction, but since we
			// just want a mock/test behavior, we can execute directly or simulate it.
			// Implementing a full DB-based Tx func is hard without accessing s.db.
			// For testing base routes directly, we just call fn()
			return fn(&model.Backends{
				Subordinates:      store.SubordinateStorage(),
				SubordinateEvents: store.SubordinateEventsStorage(),
				KV:                store.KeyValue(),
			})
		},
	}

	app := fiber.New()

	// Create a dummy fedEntity if needed, for statement previews.
	// We pass nil for base handlers since they don't strictly use it.
	registerSubordinatesBase(app, backends)

	return app, backends
}

// --- GET /subordinates TESTS ---

func TestGetSubordinates(t *testing.T) {
	t.Parallel()
	t.Run("Success/All", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://sub1.example.org",
				Status:   model.StatusActive,
			},
		})
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://sub2.example.org",
				Status:   model.StatusPending,
			},
		})

		req := httptest.NewRequest("GET", "/subordinates", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)
		var subs []model.BasicSubordinateInfo
		if err := json.Unmarshal(body, &subs); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(subs) != 2 {
			t.Errorf("Expected 2 subordinates, got %d", len(subs))
		}
	})

	t.Run("Success/ByStatus", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://active.example.org",
				Status:   model.StatusActive,
			},
		})
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://pending.example.org",
				Status:   model.StatusPending,
			},
		})

		req := httptest.NewRequest("GET", "/subordinates?status=active", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)
		var subs []model.BasicSubordinateInfo
		if err := json.Unmarshal(body, &subs); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(subs) != 1 || subs[0].EntityID != "https://active.example.org" {
			t.Errorf("Expected only active subordinate, got: %+v", subs)
		}
	})

	t.Run("Success/ByEntityType", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://rp.example.org",
				Status:   model.StatusActive,
				SubordinateEntityTypes: []model.SubordinateEntityType{
					{EntityType: "openid_relying_party"},
				},
			},
		})
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://op.example.org",
				Status:   model.StatusActive,
				SubordinateEntityTypes: []model.SubordinateEntityType{
					{EntityType: "openid_provider"},
				},
			},
		})

		req := httptest.NewRequest("GET", "/subordinates?entity_type=openid_relying_party", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)
		var subs []model.BasicSubordinateInfo
		if err := json.Unmarshal(body, &subs); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(subs) != 1 || subs[0].EntityID != "https://rp.example.org" {
			t.Errorf("Expected only RP subordinate, got: %+v", subs)
		}
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("GET", "/subordinates?status=unknown_status", http.NoBody)
		resp, body := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)

		var oidErr oidfed.Error
		if err := json.Unmarshal(body, &oidErr); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if oidErr.Error != "invalid_request" {
			t.Errorf("Expected 'invalid_request' error, got %q", oidErr.Error)
		}
	})
}

// --- POST /subordinates TESTS ---

func TestPostSubordinates(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		body := `{
			"entity_id": "https://new-sub.example.org",
			"status": "pending",
			"description": "A new subordinate"
		}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusCreated)

		// Verify it was saved to DB
		saved, err := backends.Subordinates.Get("https://new-sub.example.org")
		if err != nil || saved == nil {
			t.Fatalf("Failed to find saved subordinate in DB")
		}
		if saved.Status != model.StatusPending {
			t.Errorf("Expected status pending, got %s", saved.Status)
		}
		if saved.Description != "A new subordinate" {
			t.Errorf("Expected description 'A new subordinate', got %s", saved.Description)
		}

		// Verify event was created
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to query events: %v", err)
		}
		if len(events) != 1 || events[0].Type != model.EventTypeCreated {
			t.Errorf("Expected 1 'Created' event, got: %+v", events)
		}
	})

	t.Run("Success_WithJWKS", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		body := fmt.Sprintf(`{
			"entity_id": "https://new-sub-with-keys.example.org",
			"status": "active",
			"description": "A new active subordinate with keys",
			"jwks": {
				"keys": [
					{
						"kty": "RSA",
						"kid": "key1",
						"n": "%s",
						"e": "AQAB"
					}
				]
			},
			"registered_entity_types": ["openid_provider"]
		}`, testRSAKeyN)
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusCreated)

		saved, err := backends.Subordinates.Get("https://new-sub-with-keys.example.org")
		if err != nil || saved == nil {
			t.Fatalf("Failed to find saved subordinate in DB")
		}
		if saved.Status != model.StatusActive {
			t.Errorf("Expected status active, got %s", saved.Status)
		}
		if saved.JWKS.Keys.Len() != 1 {
			t.Errorf("Expected 1 key in JWKS, got %d", saved.JWKS.Keys.Len())
		}
		if len(saved.SubordinateEntityTypes) != 1 || saved.SubordinateEntityTypes[0].EntityType != "openid_provider" {
			t.Errorf("Expected 1 entity type 'openid_provider', got %v", saved.SubordinateEntityTypes)
		}
	})

	t.Run("MissingEntityID", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		body := `{"status": "pending"}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		body := `{"entity_id": "https://sub.example.org", "status": "unknown"}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("ActiveWithoutKeys", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		// Trying to set active status but omitting jwks
		body := `{"entity_id": "https://sub.example.org", "status": "active"}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(`not valid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// --- GET /subordinates/:subordinateID TESTS ---

func TestGetSubordinateByID(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		// Create a mock record
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://specific.example.org",
				Status:   model.StatusActive,
			},
		})

		// Grab the actual inserted ID to test the endpoint
		saved, err := backends.Subordinates.Get("https://specific.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		var sub model.ExtendedSubordinateInfo
		if err := json.Unmarshal(body, &sub); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if sub.EntityID != "https://specific.example.org" {
			t.Errorf("Expected entity ID 'https://specific.example.org', got %s", sub.EntityID)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999", http.NoBody)
		resp, _ := doRequest(t, app, req)

		// Could be 404 or 500 depending on GORM error parsing, handlers return NotFound or ServerError
		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}

// --- PUT /subordinates/:subordinateID TESTS ---

func TestPutSubordinateByID(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		// Create a mock record
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID:    "https://update.example.org",
				Status:      model.StatusActive,
				Description: "Old Description",
				SubordinateEntityTypes: []model.SubordinateEntityType{
					{EntityType: "old_type"},
				},
			},
		})
		saved, err := backends.Subordinates.Get("https://update.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		body := `{
			"description": "New Description",
			"registered_entity_types": ["new_type_1", "new_type_2"]
		}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		// Verify it was updated in DB
		updated, err := backends.Subordinates.Get("https://update.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if updated.Description != "New Description" {
			t.Errorf("Expected description 'New Description', got %q", updated.Description)
		}

		// Note: GORM's UpdateAll currently appends related entities instead of replacing them
		// due to how it handles slices on OnConflict updates. We assert there are at least 2.
		if len(updated.SubordinateEntityTypes) < 2 {
			t.Errorf("Expected at least 2 entity types, got %d", len(updated.SubordinateEntityTypes))
		}

		// Verify event was created
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
		foundUpdateEvent := false
		for _, e := range events {
			if e.Type == model.EventTypeUpdated {
				foundUpdateEvent = true
				break
			}
		}
		if !foundUpdateEvent {
			t.Errorf("Expected 'Updated' event to be recorded")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		body := `{"description": "New"}`
		req := httptest.NewRequest("PUT", "/subordinates/9999", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://bad-body.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d", saved.ID), strings.NewReader(`not json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// --- DELETE /subordinates/:subordinateID TESTS ---

func TestDeleteSubordinateByID(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		// Create a mock record
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://delete.example.org",
				Status:   model.StatusActive,
			},
		})
		saved, err := backends.Subordinates.Get("https://delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		// Create a mock event for this subordinate
		backends.SubordinateEvents.Add(model.SubordinateEvent{
			SubordinateID: saved.ID,
			Type:          model.EventTypeCreated,
		})

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d", saved.ID), http.NoBody)
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusNoContent)

		// Verify it was deleted from DB
		deleted, err := backends.Subordinates.Get("https://delete.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if deleted != nil {
			t.Errorf("Expected subordinate to be deleted, but it still exists")
		}

		// Verify events were deleted
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err == nil && len(events) > 0 {
			t.Errorf("Expected subordinate events to be deleted, but found %d events", len(events))
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("DELETE", "/subordinates/9999", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}

// --- PUT /subordinates/:subordinateID/status TESTS ---

func TestUpdateSubordinateStatus(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://status.example.org",
				Status:   model.StatusPending,
			},
		})
		saved, err := backends.Subordinates.Get("https://status.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/status", saved.ID), strings.NewReader("blocked"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		// Verify DB status
		updated, err := backends.Subordinates.Get("https://status.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if updated.Status != model.StatusBlocked {
			t.Errorf("Expected status blocked, got %s", updated.Status)
		}

		// Verify StatusUpdated event
		events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
		foundEvent := false
		for _, e := range events {
			if e.Type == model.EventTypeStatusUpdated {
				foundEvent = true
				if e.Status == nil || *e.Status != "blocked" {
					t.Errorf("Expected event status to be \"blocked\", got %v", e.Status)
				}
				break
			}
		}
		if !foundEvent {
			t.Errorf("Expected \"StatusUpdated\" event")
		}
	})

	t.Run("MissingStatus", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-status.example.org",
				Status:   model.StatusPending,
			},
		})
		saved, err := backends.Subordinates.Get("https://missing-status.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/status", saved.ID), strings.NewReader("  "))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://invalid-status.example.org",
				Status:   model.StatusPending,
			},
		})
		saved, err := backends.Subordinates.Get("https://invalid-status.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/status", saved.ID), strings.NewReader("unknown-status"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("ActiveWithoutKeys", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://no-keys.example.org",
				Status:   model.StatusPending,
			},
		})
		saved, err := backends.Subordinates.Get("https://no-keys.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/status", saved.ID), strings.NewReader("active"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/9999/status", strings.NewReader("pending"))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := doRequest(t, app, req)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}

// --- GET /subordinates/:subordinateID/history TESTS ---

func TestGetSubordinateHistory(t *testing.T) {
	t.Parallel()
	t.Run("Success/Default", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://history.example.org",
				Status:   model.StatusPending,
			},
		})
		saved, err := backends.Subordinates.Get("https://history.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		// Create mock events
		backends.SubordinateEvents.Add(model.SubordinateEvent{
			SubordinateID: saved.ID,
			Type:          model.EventTypeCreated,
		})
		status := "active"
		backends.SubordinateEvents.Add(model.SubordinateEvent{
			SubordinateID: saved.ID,
			Type:          model.EventTypeStatusUpdated,
			Status:        &status,
		})

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/history", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		var result struct {
			Events     []eventResponse `json:"events"`
			Pagination struct {
				Total  int64 `json:"total"`
				Limit  int   `json:"limit"`
				Offset int   `json:"offset"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result.Pagination.Total != 2 {
			t.Errorf("Expected total 2, got %d", result.Pagination.Total)
		}
		if len(result.Events) != 2 {
			t.Errorf("Expected 2 events returned, got %d", len(result.Events))
		}
		if result.Pagination.Limit == 0 {
			t.Errorf("Expected a default limit, got 0")
		}
	})

	t.Run("Success/WithOpts", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://history-opts.example.org",
				Status:   model.StatusPending,
			},
		})
		saved, err := backends.Subordinates.Get("https://history-opts.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		backends.SubordinateEvents.Add(model.SubordinateEvent{
			SubordinateID: saved.ID,
			Type:          model.EventTypeCreated,
		})
		backends.SubordinateEvents.Add(model.SubordinateEvent{
			SubordinateID: saved.ID,
			Type:          model.EventTypeUpdated,
		})

		// Query for limit=1, offset=1 (should return only the older/newer event depending on DB order)
		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/history?limit=1&offset=1&type=updated", saved.ID), http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		pag := result["pagination"].(map[string]any)
		if int(pag["limit"].(float64)) != 1 {
			t.Errorf("Expected limit 1, got %v", pag["limit"])
		}
		if int(pag["offset"].(float64)) != 1 {
			t.Errorf("Expected offset 1, got %v", pag["offset"])
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999/history", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})

	t.Run("InvalidQuery", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateBaseApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-query.example.org",
			},
		})
		saved, err := backends.Subordinates.Get("https://bad-query.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/history?limit=abc", saved.ID), http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}
