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
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
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

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.ConstraintSpecification
		json.Unmarshal(body, &result)

		if result.MaxPathLength == nil || *result.MaxPathLength != 5 {
			t.Errorf("Failed to retrieve correctly unmarshaled constraints: %+v", result)
		}
	})

	t.Run("GET NoConstraints", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://no-constraints.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://no-constraints.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "{}" {
			t.Errorf("Expected empty json object for nil constraints, got %s", string(body))
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
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

		requireStatus(t, resp, http.StatusOK)

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
		t.Parallel()
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

		requireStatus(t, resp, http.StatusNoContent)

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
		t.Parallel()
		app, _ := setupSubordinateConstraintsApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/constraints", http.NoBody)
		resp, _ := app.Test(req, -1)
		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/constraints/max-path-length TESTS ---

func TestSubordinateConstraintsMaxPathLength(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		length := 5
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://maxpath-get.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				MaxPathLength: &length,
			},
		})
		saved, _ := backends.Subordinates.Get("https://maxpath-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result int
		json.Unmarshal(body, &result)

		if result != 5 {
			t.Errorf("Failed to retrieve max path length: %d", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://maxpath-missing.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{},
		})
		saved, _ := backends.Subordinates.Get("https://maxpath-missing.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://maxpath-put.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				AllowedEntityTypes: []string{"keep_me"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://maxpath-put.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), strings.NewReader(`3`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		updated, _ := backends.Subordinates.Get("https://maxpath-put.example.org")
		if updated.Constraints == nil || updated.Constraints.MaxPathLength == nil || *updated.Constraints.MaxPathLength != 3 {
			t.Errorf("Expected max_path_length to be set to 3")
		}
		if len(updated.Constraints.AllowedEntityTypes) == 0 || updated.Constraints.AllowedEntityTypes[0] != "keep_me" {
			t.Errorf("Expected sibling constraints to be untouched")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		length := 5
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://maxpath-delete.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				MaxPathLength:      &length,
				AllowedEntityTypes: []string{"keep_me"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://maxpath-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNoContent)

		updated, _ := backends.Subordinates.Get("https://maxpath-delete.example.org")
		if updated.Constraints.MaxPathLength != nil {
			t.Errorf("Expected max_path_length to be nil after deletion")
		}
		if updated.Constraints.AllowedEntityTypes == nil {
			t.Errorf("Expected AllowedEntityTypes to be retained")
		}
	})
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/constraints/naming-constraints TESTS ---

func TestSubordinateConstraintsNamingConstraints(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://naming-get.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				NamingConstraints: &oidfed.NamingConstraints{
					Permitted: []string{"example.com"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://naming-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.NamingConstraints
		json.Unmarshal(body, &result)

		if len(result.Permitted) == 0 || result.Permitted[0] != "example.com" {
			t.Errorf("Failed to retrieve naming constraints: %+v", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://naming-missing.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{},
		})
		saved, _ := backends.Subordinates.Get("https://naming-missing.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://naming-put.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				AllowedEntityTypes: []string{"keep_me"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://naming-put.example.org")

		body := `{"permitted": ["new.example.com"], "excluded": ["bad.example.com"]}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		updated, _ := backends.Subordinates.Get("https://naming-put.example.org")
		if updated.Constraints == nil || updated.Constraints.NamingConstraints == nil || len(updated.Constraints.NamingConstraints.Permitted) == 0 {
			t.Errorf("Expected naming constraints to be set")
		}
		if updated.Constraints.AllowedEntityTypes == nil {
			t.Errorf("Expected sibling constraints to be untouched")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://naming-delete.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				NamingConstraints: &oidfed.NamingConstraints{
					Permitted: []string{"example.com"},
				},
				AllowedEntityTypes: []string{"keep_me"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://naming-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNoContent)

		updated, _ := backends.Subordinates.Get("https://naming-delete.example.org")
		if updated.Constraints.NamingConstraints != nil {
			t.Errorf("Expected naming constraints to be nil after deletion")
		}
		if updated.Constraints.AllowedEntityTypes == nil {
			t.Errorf("Expected AllowedEntityTypes to be retained")
		}
	})
}

// --- GET, PUT, POST, DELETE /subordinates/:subordinateID/constraints/allowed-entity-types TESTS ---

func TestSubordinateConstraintsAllowedEntityTypes(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://allowed-get.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				AllowedEntityTypes: []string{"openid_relying_party"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://allowed-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result []string
		json.Unmarshal(body, &result)

		if len(result) == 0 || result[0] != "openid_relying_party" {
			t.Errorf("Failed to retrieve allowed entity types: %+v", result)
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://allowed-missing.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{},
		})
		saved, _ := backends.Subordinates.Get("https://allowed-missing.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		length := 5
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://allowed-put.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				AllowedEntityTypes: []string{"old_type"},
				MaxPathLength:      &length,
			},
		})
		saved, _ := backends.Subordinates.Get("https://allowed-put.example.org")

		body := `["new_type"]`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		updated, _ := backends.Subordinates.Get("https://allowed-put.example.org")
		if updated.Constraints == nil || len(updated.Constraints.AllowedEntityTypes) == 0 || updated.Constraints.AllowedEntityTypes[0] != "new_type" {
			t.Errorf("Expected allowed entity types to be replaced")
		}
		if updated.Constraints.MaxPathLength == nil {
			t.Errorf("Expected sibling constraints to be untouched")
		}
	})

	t.Run("POST Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://allowed-post.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				AllowedEntityTypes: []string{"old_type"},
			},
		})
		saved, _ := backends.Subordinates.Get("https://allowed-post.example.org")

		body := `merged_type`
		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusCreated)

		updated, _ := backends.Subordinates.Get("https://allowed-post.example.org")

		// POST should merge the new type with the old type
		types := updated.Constraints.AllowedEntityTypes
		if len(types) != 2 {
			t.Errorf("Expected 2 allowed entity types after merge, got %d: %+v", len(types), types)
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateConstraintsApp(t)

		length := 5
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://allowed-delete.example.org",
			},
			Constraints: &oidfed.ConstraintSpecification{
				AllowedEntityTypes: []string{"delete_me", "keep_me"},
				MaxPathLength:      &length,
			},
		})
		saved, _ := backends.Subordinates.Get("https://allowed-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types/delete_me", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		updated, _ := backends.Subordinates.Get("https://allowed-delete.example.org")
		types := updated.Constraints.AllowedEntityTypes

		if len(types) != 1 || types[0] != "keep_me" {
			t.Errorf("Expected delete_me to be removed, leaving keep_me. Got: %+v", types)
		}
		if updated.Constraints.MaxPathLength == nil {
			t.Errorf("Expected MaxPathLength to be retained")
		}
	})
}

// ============================================================================
// GENERAL CONSTRAINTS TESTS
// ============================================================================

func setupGeneralConstraintsApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	backends := model.Backends{
		KV: store.KeyValue(),
	}

	app := fiber.New()
	registerGeneralConstraints(app, backends.KV)
	return app, backends
}

// --- GET, PUT /subordinates/constraints TESTS ---

func TestGeneralConstraintsAll(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)

		length := 5
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			MaxPathLength: &length,
		})

		req := httptest.NewRequest("GET", "/subordinates/constraints", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.ConstraintSpecification
		json.Unmarshal(body, &result)

		if result.MaxPathLength == nil || *result.MaxPathLength != 5 {
			t.Errorf("Failed to retrieve constraints: %+v", result)
		}
	})

	t.Run("GET NoConstraints", func(t *testing.T) {
		t.Parallel()
		app, _ := setupGeneralConstraintsApp(t)
		req := httptest.NewRequest("GET", "/subordinates/constraints", http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)

		body := `{"max_path_length": 3}`
		req := httptest.NewRequest("PUT", "/subordinates/constraints", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if updated.MaxPathLength == nil || *updated.MaxPathLength != 3 {
			t.Errorf("Expected max_path_length to be 3")
		}
	})
}

// --- GET, PUT, DELETE /subordinates/constraints/max-path-length TESTS ---

func TestGeneralConstraintsMaxPathLength(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		length := 5
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			MaxPathLength: &length,
		})

		req := httptest.NewRequest("GET", "/subordinates/constraints/max-path-length", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result int
		json.Unmarshal(body, &result)
		if result != 5 {
			t.Errorf("Failed to retrieve max path length: %d", result)
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			AllowedEntityTypes: []string{"keep_me"},
		})

		req := httptest.NewRequest("PUT", "/subordinates/constraints/max-path-length", strings.NewReader(`3`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if updated.MaxPathLength == nil || *updated.MaxPathLength != 3 {
			t.Errorf("Expected max_path_length to be 3")
		}
		if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
			t.Errorf("Expected sibling constraints to be untouched")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		length := 5
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			MaxPathLength:      &length,
			AllowedEntityTypes: []string{"keep_me"},
		})

		req := httptest.NewRequest("DELETE", "/subordinates/constraints/max-path-length", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNoContent)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if updated.MaxPathLength != nil {
			t.Errorf("Expected max_path_length to be nil")
		}
		if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
			t.Errorf("Expected AllowedEntityTypes to be safely retained")
		}
	})
}

// --- GET, PUT, DELETE /subordinates/constraints/naming-constraints TESTS ---

func TestGeneralConstraintsNamingConstraints(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			NamingConstraints: &oidfed.NamingConstraints{
				Permitted: []string{"example.com"},
			},
		})

		req := httptest.NewRequest("GET", "/subordinates/constraints/naming-constraints", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result oidfed.NamingConstraints
		json.Unmarshal(body, &result)

		if len(result.Permitted) == 0 || result.Permitted[0] != "example.com" {
			t.Errorf("Failed to retrieve naming constraints: %+v", result)
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			AllowedEntityTypes: []string{"keep_me"},
		})

		body := `{"permitted": ["new.example.com"], "excluded": ["bad.example.com"]}`
		req := httptest.NewRequest("PUT", "/subordinates/constraints/naming-constraints", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if updated.NamingConstraints == nil || len(updated.NamingConstraints.Permitted) == 0 || updated.NamingConstraints.Permitted[0] != "new.example.com" {
			t.Errorf("Expected naming constraints to be set")
		}
		if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
			t.Errorf("Expected sibling constraints to be untouched")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			NamingConstraints: &oidfed.NamingConstraints{
				Permitted: []string{"example.com"},
			},
			AllowedEntityTypes: []string{"keep_me"},
		})

		req := httptest.NewRequest("DELETE", "/subordinates/constraints/naming-constraints", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNoContent)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if updated.NamingConstraints != nil {
			t.Errorf("Expected naming constraints to be nil")
		}
		if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
			t.Errorf("Expected AllowedEntityTypes to be retained safely")
		}
	})
}

// --- GET, PUT, POST, DELETE /subordinates/constraints/allowed-entity-types TESTS ---

func TestGeneralConstraintsAllowedEntityTypes(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			AllowedEntityTypes: []string{"openid_relying_party"},
		})

		req := httptest.NewRequest("GET", "/subordinates/constraints/allowed-entity-types", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result []string
		json.Unmarshal(body, &result)

		if len(result) == 0 || result[0] != "openid_relying_party" {
			t.Errorf("Failed to retrieve allowed entity types: %+v", result)
		}
	})

	t.Run("PUT Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			AllowedEntityTypes: []string{"old_type"},
		})

		req := httptest.NewRequest("PUT", "/subordinates/constraints/allowed-entity-types", strings.NewReader(`["new_type"]`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "new_type" {
			t.Errorf("Expected allowed entity types to be replaced")
		}
	})

	t.Run("POST Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			AllowedEntityTypes: []string{"old_type"},
		})

		req := httptest.NewRequest("POST", "/subordinates/constraints/allowed-entity-types", strings.NewReader(`merged_type`))
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusCreated)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if len(updated.AllowedEntityTypes) != 2 {
			t.Errorf("Expected 2 allowed entity types after merge")
		}
	})

	t.Run("DELETE Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupGeneralConstraintsApp(t)
		backends.KV.SetAny(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
			AllowedEntityTypes: []string{"delete_me", "keep_me"},
		})

		req := httptest.NewRequest("DELETE", "/subordinates/constraints/allowed-entity-types/delete_me", http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		var updated oidfed.ConstraintSpecification
		backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated)
		if len(updated.AllowedEntityTypes) != 1 || updated.AllowedEntityTypes[0] != "keep_me" {
			t.Errorf("Expected delete_me to be removed")
		}
	})
}
