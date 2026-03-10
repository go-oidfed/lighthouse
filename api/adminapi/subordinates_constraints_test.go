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

// setupSubordinateConstraintsApp creates a Fiber app and registers constraints endpoints.
func setupSubordinateConstraintsApp(t *testing.T) (*fiber.App, model.Backends) {
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
	registerSubordinateConstraints(app, backends)
	return app, backends
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/constraints TESTS ---

func TestSubordinateConstraintsAll(t *testing.T) {
	t.Run("GET Success", func(t *testing.T) {
		app, backends := setupSubordinateConstraintsApp(t)

		length := 5
		constraints := &oidfed.ConstraintSpecification{
			MaxPathLength: &length,
		}

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://constraints-get.example.org",
			},
			Constraints: constraints,
		})
		saved, _ := backends.Subordinates.Get("https://constraints-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.ConstraintSpecification
		json.Unmarshal(body, &result)

		if result.MaxPathLength == nil || *result.MaxPathLength != 5 {
			t.Errorf("Failed to retrieve correctly unmarshaled constraints: %+v", result)
		}
	})

	t.Run("GET NoConstraints", func(t *testing.T) {
		app, backends := setupSubordinateConstraintsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://no-constraints.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://no-constraints.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
		
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "{}" {
			t.Errorf("Expected empty json object for nil constraints, got %s", string(body))
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://constraints-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://constraints-put.example.org")

		body := `{
			"max_path_length": 3
		}`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB
		updated, _ := backends.Subordinates.Get("https://constraints-put.example.org")
		if updated.Constraints == nil || updated.Constraints.MaxPathLength == nil || *updated.Constraints.MaxPathLength != 3 {
			t.Errorf("Expected constraints to be updated in DB")
		}

		// Verify Event
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeConstraintsUpdated {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ConstraintsUpdated event to be logged")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		app, backends := setupSubordinateConstraintsApp(t)

		length := 5
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://constraints-delete.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				MaxPathLength: &length,
			},
		})
		saved, _ := backends.Subordinates.Get("https://constraints-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", resp.StatusCode)
		}

		updated, _ := backends.Subordinates.Get("https://constraints-delete.example.org")
		if updated.Constraints != nil {
			t.Errorf("Expected Constraints to be nil after deletion")
		}

		// Verify Event
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeConstraintsDeleted {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ConstraintsDeleted event to be logged")
		}
	})
	
	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateConstraintsApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/constraints", http.NoBody)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}