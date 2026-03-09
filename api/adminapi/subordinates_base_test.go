package adminapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// --- TEST HELPERS ---

// newSubordinateTestStorage creates a temporary SQLite database for subordinate tests.
func newSubordinateTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lighthouse-sub-test-*")
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewStorage(storage.Config{
		Driver:  storage.DriverSQLite,
		DataDir: tempDir,
	})
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create storage: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})
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
	t.Run("Success/All", func(t *testing.T) {
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
		resp, _ := app.Test(req, -1)
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d, body: %s", resp.StatusCode, string(body))
		}
		var subs []model.BasicSubordinateInfo
		if err := json.Unmarshal(body, &subs); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(subs) != 2 {
			t.Errorf("Expected 2 subordinates, got %d", len(subs))
		}
	})

	t.Run("Success/ByStatus", func(t *testing.T) {
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
		resp, _ := app.Test(req, -1)
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d, body: %s", resp.StatusCode, string(body))
		}
		var subs []model.BasicSubordinateInfo
		json.Unmarshal(body, &subs)

		if len(subs) != 1 || subs[0].EntityID != "https://active.example.org" {
			t.Errorf("Expected only active subordinate, got: %+v", subs)
		}
	})

	t.Run("Success/ByEntityType", func(t *testing.T) {
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
		resp, _ := app.Test(req, -1)
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d, body: %s", resp.StatusCode, string(body))
		}
		var subs []model.BasicSubordinateInfo
		json.Unmarshal(body, &subs)

		if len(subs) != 1 || subs[0].EntityID != "https://rp.example.org" {
			t.Errorf("Expected only RP subordinate, got: %+v", subs)
		}
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		app, _ := setupSubordinateBaseApp(t)
		
		req := httptest.NewRequest("GET", "/subordinates?status=unknown_status", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid status, got %d", resp.StatusCode)
		}
		
		body, _ := io.ReadAll(resp.Body)
		var oidErr oidfed.Error
		json.Unmarshal(body, &oidErr)
		if oidErr.Error != "invalid_request" {
			t.Errorf("Expected 'invalid_request' error, got %q", oidErr.Error)
		}
	})
}

// --- POST /subordinates TESTS ---

func TestPostSubordinates(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, backends := setupSubordinateBaseApp(t)

		body := `{
			"entity_id": "https://new-sub.example.org",
			"status": "pending",
			"description": "A new subordinate"
		}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(b))
		}

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

	t.Run("MissingEntityID", func(t *testing.T) {
		app, _ := setupSubordinateBaseApp(t)

		body := `{"status": "pending"}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		app, _ := setupSubordinateBaseApp(t)

		body := `{"entity_id": "https://sub.example.org", "status": "unknown"}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ActiveWithoutKeys", func(t *testing.T) {
		app, _ := setupSubordinateBaseApp(t)

		// Trying to set active status but omitting jwks
		body := `{"entity_id": "https://sub.example.org", "status": "active"}`
		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, _ := setupSubordinateBaseApp(t)

		req := httptest.NewRequest("POST", "/subordinates", strings.NewReader(`not valid json`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}
