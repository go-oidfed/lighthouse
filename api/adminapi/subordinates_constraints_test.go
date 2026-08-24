package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/oidfedconst"
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
			return fn(
				&model.Backends{
					Subordinates:      store.SubordinateStorage(),
					SubordinateEvents: store.SubordinateEventsStorage(),
					KV:                store.KeyValue(),
				},
			)
		},
	}

	app := fiber.New()
	registerSubordinateConstraints(app, backends)
	return app, backends
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/constraints TESTS ---

func TestSubordinateConstraintsAll(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			constraints := &oidfed.ConstraintSpecification{
				MaxPathLength: new(5),
			}

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://constraints-get.example.org",
					},
					Constraints: constraints,
				},
			)
			saved, err := backends.Subordinates.Get("https://constraints-get.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result oidfed.ConstraintSpecification
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if result.MaxPathLength == nil || *result.MaxPathLength != 5 {
				t.Errorf("Failed to retrieve correctly unmarshaled constraints: %+v", result)
			}
		},
	)

	t.Run(
		"GET NoConstraints", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://no-constraints.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://no-constraints.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			if string(body) != "{}" {
				t.Errorf("Expected empty json object for nil constraints, got %s", string(body))
			}
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://constraints-put.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://constraints-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			body := `{
			"max_path_length": 3
		}`

			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			// Verify DB
			updated, err := backends.Subordinates.Get("https://constraints-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints == nil || updated.Constraints.MaxPathLength == nil || *updated.Constraints.MaxPathLength != 3 {
				t.Errorf("Expected constraints to be updated in DB")
			}

			// Verify Event
			events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
			if err != nil {
				t.Fatalf("Failed to get events: %v", err)
			}
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
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://constraints-delete.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						MaxPathLength: new(5),
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://constraints-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNoContent)

			updated, err := backends.Subordinates.Get("https://constraints-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints != nil {
				t.Errorf("Expected Constraints to be nil after deletion")
			}

			// Verify Event
			events, _, err := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
			if err != nil {
				t.Fatalf("Failed to get events: %v", err)
			}
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
		},
	)

	t.Run(
		"NotFound", func(t *testing.T) {
			t.Parallel()
			app, _ := setupSubordinateConstraintsApp(t)
			req := httptest.NewRequest("GET", "/subordinates/9999/constraints", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			assertStatus(t, resp, bodyBytes, http.StatusNotFound)
		},
	)
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/constraints/max-path-length TESTS ---

func TestSubordinateConstraintsMaxPathLength(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://maxpath-get.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						MaxPathLength: new(5),
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://maxpath-get.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"GET", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), http.NoBody,
			)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result int
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if result != 5 {
				t.Errorf("Failed to retrieve max path length: %d", result)
			}
		},
	)

	t.Run(
		"GET NotFound", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://maxpath-missing.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{},
				},
			)
			saved, err := backends.Subordinates.Get("https://maxpath-missing.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"GET", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNotFound)
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://maxpath-put.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{"keep_me"},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://maxpath-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), strings.NewReader(`3`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			updated, err := backends.Subordinates.Get("https://maxpath-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints == nil || updated.Constraints.MaxPathLength == nil || *updated.Constraints.MaxPathLength != 3 {
				t.Errorf("Expected max_path_length to be set to 3")
			}
			if len(updated.Constraints.AllowedEntityTypes) == 0 || updated.Constraints.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected sibling constraints to be untouched")
			}
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://maxpath-delete.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						MaxPathLength:      new(5),
						AllowedEntityTypes: []string{"keep_me"},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://maxpath-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"DELETE", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID), http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNoContent)

			updated, err := backends.Subordinates.Get("https://maxpath-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints.MaxPathLength != nil {
				t.Errorf("Expected max_path_length to be nil after deletion")
			}
			if updated.Constraints.AllowedEntityTypes == nil {
				t.Fatal("Expected AllowedEntityTypes to be retained")
			}
			if len(updated.Constraints.AllowedEntityTypes) != 1 || updated.Constraints.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected AllowedEntityTypes [keep_me], got %v", updated.Constraints.AllowedEntityTypes)
			}
		},
	)
}

// --- GET, PUT, DELETE /subordinates/:subordinateID/constraints/naming-constraints TESTS ---

func TestSubordinateConstraintsNamingConstraints(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://naming-get.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						NamingConstraints: &oidfed.NamingConstraints{
							Permitted: []string{"example.com"},
						},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://naming-get.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"GET", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), http.NoBody,
			)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result oidfed.NamingConstraints
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(result.Permitted) == 0 || result.Permitted[0] != "example.com" {
				t.Errorf("Failed to retrieve naming constraints: %+v", result)
			}
		},
	)

	t.Run(
		"GET NotFound", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://naming-missing.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{},
				},
			)
			saved, err := backends.Subordinates.Get("https://naming-missing.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"GET", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNotFound)
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://naming-put.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{"keep_me"},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://naming-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			body := `{"permitted": ["new.example.com"], "excluded": ["bad.example.com"]}`
			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID),
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			updated, err := backends.Subordinates.Get("https://naming-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints == nil || updated.Constraints.NamingConstraints == nil || len(updated.Constraints.NamingConstraints.Permitted) == 0 {
				t.Errorf("Expected naming constraints to be set")
			}
			if updated.Constraints.AllowedEntityTypes == nil {
				t.Fatal("Expected sibling constraints to be untouched")
			}
			if len(updated.Constraints.AllowedEntityTypes) != 1 || updated.Constraints.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected AllowedEntityTypes [keep_me], got %v", updated.Constraints.AllowedEntityTypes)
			}
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://naming-delete.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						NamingConstraints: &oidfed.NamingConstraints{
							Permitted: []string{"example.com"},
						},
						AllowedEntityTypes: []string{"keep_me"},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://naming-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"DELETE", fmt.Sprintf("/subordinates/%d/constraints/naming-constraints", saved.ID), http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNoContent)

			updated, err := backends.Subordinates.Get("https://naming-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints.NamingConstraints != nil {
				t.Errorf("Expected naming constraints to be nil after deletion")
			}
			if updated.Constraints.AllowedEntityTypes == nil {
				t.Fatal("Expected AllowedEntityTypes to be retained")
			}
			if len(updated.Constraints.AllowedEntityTypes) != 1 || updated.Constraints.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected AllowedEntityTypes [keep_me], got %v", updated.Constraints.AllowedEntityTypes)
			}
		},
	)
}

// --- GET, PUT, POST, DELETE /subordinates/:subordinateID/constraints/allowed-entity-types TESTS ---

func TestSubordinateConstraintsAllowedEntityTypes(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://allowed-get.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{oidfedconst.EntityTypeOpenIDRelyingParty},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://allowed-get.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"GET", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID), http.NoBody,
			)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result []string
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(result) == 0 || result[0] != oidfedconst.EntityTypeOpenIDRelyingParty {
				t.Errorf("Failed to retrieve allowed entity types: %+v", result)
			}
		},
	)

	t.Run(
		"GET NotFound", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://allowed-missing.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{},
				},
			)
			saved, err := backends.Subordinates.Get("https://allowed-missing.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"GET", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID), http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNotFound)
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://allowed-put.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{"old_type"},
						MaxPathLength:      new(5),
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://allowed-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			body := `["new_type"]`
			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID),
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			updated, err := backends.Subordinates.Get("https://allowed-put.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			if updated.Constraints == nil || len(updated.Constraints.AllowedEntityTypes) == 0 || updated.Constraints.AllowedEntityTypes[0] != "new_type" {
				t.Errorf("Expected allowed entity types to be replaced")
			}
			if updated.Constraints.MaxPathLength == nil {
				t.Fatal("Expected sibling constraints to be untouched")
			}
			if *updated.Constraints.MaxPathLength != 5 {
				t.Errorf("Expected MaxPathLength to be 5, got %d", *updated.Constraints.MaxPathLength)
			}
		},
	)

	t.Run(
		"POST Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://allowed-post.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{"old_type"},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://allowed-post.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			body := `merged_type`
			req := httptest.NewRequest(
				"POST", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID),
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "text/plain")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusCreated)

			updated, err := backends.Subordinates.Get("https://allowed-post.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			// POST should merge the new type with the old type
			types := updated.Constraints.AllowedEntityTypes
			if len(types) != 2 {
				t.Errorf("Expected 2 allowed entity types after merge, got %d: %+v", len(types), types)
			}
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://allowed-delete.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{
							"delete_me",
							"keep_me",
						},
						MaxPathLength: new(5),
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://allowed-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"DELETE", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types/delete_me", saved.ID),
				http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			updated, err := backends.Subordinates.Get("https://allowed-delete.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}
			types := updated.Constraints.AllowedEntityTypes

			if len(types) != 1 || types[0] != "keep_me" {
				t.Errorf("Expected delete_me to be removed, leaving keep_me. Got: %+v", types)
			}
			if updated.Constraints.MaxPathLength == nil {
				t.Fatal("Expected MaxPathLength to be retained")
			}
			if *updated.Constraints.MaxPathLength != 5 {
				t.Errorf("Expected MaxPathLength to be 5, got %d", *updated.Constraints.MaxPathLength)
			}
		},
	)
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
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)

			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					MaxPathLength: new(5),
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("GET", "/subordinates/constraints", http.NoBody)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result oidfed.ConstraintSpecification
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if result.MaxPathLength == nil || *result.MaxPathLength != 5 {
				t.Errorf("Failed to retrieve constraints: %+v", result)
			}
		},
	)

	t.Run(
		"GET NoConstraints", func(t *testing.T) {
			t.Parallel()
			app, _ := setupGeneralConstraintsApp(t)
			req := httptest.NewRequest("GET", "/subordinates/constraints", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)

			assertStatus(t, resp, bodyBytes, http.StatusNotFound)
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)

			body := `{"max_path_length": 3}`
			req := httptest.NewRequest("PUT", "/subordinates/constraints", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if updated.MaxPathLength == nil || *updated.MaxPathLength != 3 {
				t.Errorf("Expected max_path_length to be 3")
			}
		},
	)
}

// --- GET, PUT, DELETE /subordinates/constraints/max-path-length TESTS ---

func TestGeneralConstraintsMaxPathLength(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					MaxPathLength: new(5),
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("GET", "/subordinates/constraints/max-path-length", http.NoBody)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result int
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}
			if result != 5 {
				t.Errorf("Failed to retrieve max path length: %d", result)
			}
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					AllowedEntityTypes: []string{"keep_me"},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("PUT", "/subordinates/constraints/max-path-length", strings.NewReader(`3`))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if updated.MaxPathLength == nil || *updated.MaxPathLength != 3 {
				t.Errorf("Expected max_path_length to be 3")
			}
			if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected sibling constraints to be untouched")
			}
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					MaxPathLength:      new(5),
					AllowedEntityTypes: []string{"keep_me"},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("DELETE", "/subordinates/constraints/max-path-length", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNoContent)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if updated.MaxPathLength != nil {
				t.Errorf("Expected max_path_length to be nil")
			}
			if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected AllowedEntityTypes to be safely retained")
			}
		},
	)
}

// --- GET, PUT, DELETE /subordinates/constraints/naming-constraints TESTS ---

func TestGeneralConstraintsNamingConstraints(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					NamingConstraints: &oidfed.NamingConstraints{
						Permitted: []string{"example.com"},
					},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("GET", "/subordinates/constraints/naming-constraints", http.NoBody)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result oidfed.NamingConstraints
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(result.Permitted) == 0 || result.Permitted[0] != "example.com" {
				t.Errorf("Failed to retrieve naming constraints: %+v", result)
			}
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					AllowedEntityTypes: []string{"keep_me"},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			body := `{"permitted": ["new.example.com"], "excluded": ["bad.example.com"]}`
			req := httptest.NewRequest("PUT", "/subordinates/constraints/naming-constraints", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if updated.NamingConstraints == nil || len(updated.NamingConstraints.Permitted) == 0 || updated.NamingConstraints.Permitted[0] != "new.example.com" {
				t.Errorf("Expected naming constraints to be set")
			}
			if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected sibling constraints to be untouched")
			}
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					NamingConstraints: &oidfed.NamingConstraints{
						Permitted: []string{"example.com"},
					},
					AllowedEntityTypes: []string{"keep_me"},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("DELETE", "/subordinates/constraints/naming-constraints", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusNoContent)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if updated.NamingConstraints != nil {
				t.Errorf("Expected naming constraints to be nil")
			}
			if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected AllowedEntityTypes to be retained safely")
			}
		},
	)
}

// --- GET, PUT, POST, DELETE /subordinates/constraints/allowed-entity-types TESTS ---

func TestGeneralConstraintsAllowedEntityTypes(t *testing.T) {
	t.Parallel()
	t.Run(
		"GET Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					AllowedEntityTypes: []string{oidfedconst.EntityTypeOpenIDRelyingParty},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest("GET", "/subordinates/constraints/allowed-entity-types", http.NoBody)
			resp, body := doRequest(t, app, req)

			requireStatus(t, resp, body, http.StatusOK)

			var result []string
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(result) == 0 || result[0] != oidfedconst.EntityTypeOpenIDRelyingParty {
				t.Errorf("Failed to retrieve allowed entity types: %+v", result)
			}
		},
	)

	t.Run(
		"PUT Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					AllowedEntityTypes: []string{"old_type"},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest(
				"PUT", "/subordinates/constraints/allowed-entity-types", strings.NewReader(`["new_type"]`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if len(updated.AllowedEntityTypes) == 0 || updated.AllowedEntityTypes[0] != "new_type" {
				t.Errorf("Expected allowed entity types to be replaced")
			}
		},
	)

	t.Run(
		"POST Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					AllowedEntityTypes: []string{"old_type"},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest(
				"POST", "/subordinates/constraints/allowed-entity-types", strings.NewReader(`merged_type`),
			)
			req.Header.Set("Content-Type", "text/plain")
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusCreated)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if len(updated.AllowedEntityTypes) != 2 {
				t.Errorf("Expected 2 allowed entity types after merge")
			}
		},
	)

	t.Run(
		"DELETE Success", func(t *testing.T) {
			t.Parallel()
			app, backends := setupGeneralConstraintsApp(t)
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					AllowedEntityTypes: []string{
						"delete_me",
						"keep_me",
					},
				},
			); err != nil {
				t.Fatalf("Failed to set KV value: %v", err)
			}

			req := httptest.NewRequest(
				"DELETE", "/subordinates/constraints/allowed-entity-types/delete_me", http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)

			requireStatus(t, resp, bodyBytes, http.StatusOK)

			var updated oidfed.ConstraintSpecification
			if _, err := backends.KV.GetAs(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &updated,
			); err != nil {
				t.Fatalf("Failed to get KV value: %v", err)
			}
			if len(updated.AllowedEntityTypes) != 1 || updated.AllowedEntityTypes[0] != "keep_me" {
				t.Errorf("Expected delete_me to be removed")
			}
		},
	)
}

// --- POST /subordinates/:subordinateID/constraints TESTS (Copy from General) ---

func TestSubordinateConstraintsPostAll(t *testing.T) {
	t.Parallel()

	t.Run(
		"Success/CopiesFromGeneral", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			// Seed general constraints in KV
			if err := backends.KV.SetAny(
				model.KeyValueScopeSubordinateStatement, model.KeyValueKeyConstraints, &oidfed.ConstraintSpecification{
					MaxPathLength:      new(3),
					AllowedEntityTypes: []string{oidfedconst.EntityTypeOpenIDProvider},
				},
			); err != nil {
				t.Fatalf("Failed to seed general constraints: %v", err)
			}

			// Seed subordinate with no constraints
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://post-all.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://post-all.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusCreated)

			var result oidfed.ConstraintSpecification
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if result.MaxPathLength == nil || *result.MaxPathLength != 3 {
				t.Errorf("Expected MaxPathLength 3, got %v", result.MaxPathLength)
			}
			if len(result.AllowedEntityTypes) != 1 || result.AllowedEntityTypes[0] != oidfedconst.EntityTypeOpenIDProvider {
				t.Errorf("Expected AllowedEntityTypes [openid_provider], got %v", result.AllowedEntityTypes)
			}
		},
	)

	t.Run(
		"Success/NoGeneral", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			// No general constraints seeded — should copy empty spec
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://post-all-empty.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://post-all-empty.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), http.NoBody)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusCreated)

			var result oidfed.ConstraintSpecification
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if result.MaxPathLength != nil {
				t.Errorf("Expected nil MaxPathLength for empty general, got %v", result.MaxPathLength)
			}
		},
	)

	t.Run(
		"NotFound", func(t *testing.T) {
			t.Parallel()
			app, _ := setupSubordinateConstraintsApp(t)

			req := httptest.NewRequest("POST", "/subordinates/9999/constraints", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			assertStatus(t, resp, bodyBytes, http.StatusNotFound)
		},
	)
}

// --- EDGE CASE TESTS ---

func TestSubordinateConstraintsEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run(
		"MaxPathLength/ZeroBoundary", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://mpl-zero.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://mpl-zero.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			// max_path_length = 0 should succeed (0 is valid, means "no further subordinates")
			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID),
				strings.NewReader(`0`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var result int
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}
			if result != 0 {
				t.Errorf("Expected 0, got %d", result)
			}
		},
	)

	t.Run(
		"MaxPathLength/NegativeRejected", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://mpl-neg.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://mpl-neg.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints/max-path-length", saved.ID),
				strings.NewReader(`-1`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
		},
	)

	t.Run(
		"PostAllowedEntityType/Duplicate", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://dup-type.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{oidfedconst.EntityTypeOpenIDProvider},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://dup-type.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			// POST duplicate — should return existing list without adding
			req := httptest.NewRequest(
				"POST", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types", saved.ID),
				strings.NewReader(`openid_provider`),
			)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusCreated)

			var result []string
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}
			// Should still have exactly 1, not 2
			if len(result) != 1 {
				t.Errorf("Expected 1 entity type (idempotent), got %d: %v", len(result), result)
			}
		},
	)

	t.Run(
		"DeleteAllowedEntityType/NonExistent", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://del-noexist.example.org",
					},
					Constraints: &oidfed.ConstraintSpecification{
						AllowedEntityTypes: []string{oidfedconst.EntityTypeOpenIDProvider},
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://del-noexist.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			// DELETE a type that doesn't exist — returns unchanged list
			req := httptest.NewRequest(
				"DELETE", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types/nonexistent_type", saved.ID),
				http.NoBody,
			)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var result []string
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}
			if len(result) != 1 || result[0] != oidfedconst.EntityTypeOpenIDProvider {
				t.Errorf("Expected unchanged [openid_provider], got %v", result)
			}
		},
	)

	t.Run(
		"DeleteAllowedEntityType/NilConstraints", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			// Subordinate with nil constraints
			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://del-nilcons.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://del-nilcons.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			req := httptest.NewRequest(
				"DELETE", fmt.Sprintf("/subordinates/%d/constraints/allowed-entity-types/any", saved.ID), http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusNoContent)
		},
	)

	t.Run(
		"PutAll/NegativeMaxPathLength", func(t *testing.T) {
			t.Parallel()
			app, backends := setupSubordinateConstraintsApp(t)

			backends.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://putall-neg.example.org",
					},
				},
			)
			saved, err := backends.Subordinates.Get("https://putall-neg.example.org")
			if err != nil {
				t.Fatalf("Failed to get subordinate: %v", err)
			}

			body := fmt.Sprintf(
				`{"max_path_length": -1, "allowed_entity_types": ["%s"]}`, oidfedconst.EntityTypeOpenIDProvider,
			)
			req := httptest.NewRequest(
				"PUT", fmt.Sprintf("/subordinates/%d/constraints", saved.ID), strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			assertErrorResponse(t, resp, respBody, http.StatusBadRequest, "invalid_request")
		},
	)
}

// --- REGRESSION: constraints writes must not freeze the general metadata policy ---

// TestSubordinateConstraintsWriteDoesNotFreezeMetadataPolicy verifies that a
// constraints write on a subordinate without its own metadata policy does not
// materialize the general metadata policy as a frozen snapshot on the
// subordinate (see bug: "PUT /subordinates/{id}/constraints silently freezes
// that subordinate's metadata policy"). The subordinate must keep live-following
// the general policy after constraints writes.
func TestSubordinateConstraintsWriteDoesNotFreezeMetadataPolicy(t *testing.T) {
	t.Parallel()

	generalPolicy := func(value string) *oidfed.MetadataPolicies {
		return &oidfed.MetadataPolicies{
			FederationEntity: oidfed.MetadataPolicy{
				"organization_name": oidfed.MetadataPolicyEntry{"value": value},
			},
		}
	}

	setGeneralPolicy := func(t *testing.T, backends model.Backends, mp *oidfed.MetadataPolicies) {
		t.Helper()
		if err := backends.KV.SetAny(
			model.KeyValueScopeSubordinateStatement, model.KeyValueKeyMetadataPolicy, mp,
		); err != nil {
			t.Fatalf("Failed to set general metadata policy: %v", err)
		}
	}

	// The value a subordinate's statement resolves to (general fallback applied).
	resolvedOrganizationName := func(t *testing.T, backends model.Backends, entityID string) any {
		t.Helper()
		info, err := backends.Subordinates.Get(entityID)
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		if info.MetadataPolicy == nil {
			t.Fatalf("Expected resolved metadata policy, got nil")
		}
		entry := info.MetadataPolicy.FederationEntity["organization_name"]
		if entry == nil {
			t.Fatalf("Expected organization_name policy entry, got %+v", info.MetadataPolicy)
		}
		return entry["value"]
	}

	setup := func(t *testing.T) (*fiber.App, model.Backends, string) {
		t.Helper()
		app, backends := setupSubordinateConstraintsApp(t)
		backends.Subordinates.Add(
			model.ExtendedSubordinateInfo{
				BasicSubordinateInfo: model.BasicSubordinateInfo{
					EntityID: "https://constraints-no-freeze.example.org",
				},
			},
		)
		saved, err := backends.Subordinates.Get("https://constraints-no-freeze.example.org")
		if err != nil {
			t.Fatalf("Failed to get subordinate: %v", err)
		}
		return app, backends, fmt.Sprintf("%d", saved.ID)
	}

	t.Run(
		"PutAll", func(t *testing.T) {
			t.Parallel()
			app, backends, id := setup(t)
			const entityID = "https://constraints-no-freeze.example.org"

			setGeneralPolicy(t, backends, generalPolicy("v1"))
			if got := resolvedOrganizationName(t, backends, entityID); got != "v1" {
				t.Fatalf("Expected subordinate to live-follow general policy 'v1', got %v", got)
			}

			body := fmt.Sprintf(
				`{"allowed_entity_types": ["%s"]}`, oidfedconst.EntityTypeOpenIDProvider,
			)
			req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%s/constraints", id), strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, http.StatusOK)

			// The constraints write must not have persisted a metadata policy snapshot.
			raw, err := backends.Subordinates.GetByDBIDRaw(id)
			if err != nil {
				t.Fatalf("Failed to get raw subordinate: %v", err)
			}
			if raw.MetadataPolicy != nil {
				t.Fatalf("Constraints write materialized the general metadata policy onto the subordinate")
			}

			setGeneralPolicy(t, backends, generalPolicy("v2"))
			if got := resolvedOrganizationName(t, backends, entityID); got != "v2" {
				t.Fatalf("Expected subordinate to keep live-following general policy 'v2', got %v", got)
			}
		},
	)

	t.Run(
		"DeleteAll", func(t *testing.T) {
			t.Parallel()
			app, backends, id := setup(t)
			const entityID = "https://constraints-no-freeze.example.org"

			setGeneralPolicy(t, backends, generalPolicy("v1"))

			req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%s/constraints", id), http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, http.StatusNoContent)

			raw, err := backends.Subordinates.GetByDBIDRaw(id)
			if err != nil {
				t.Fatalf("Failed to get raw subordinate: %v", err)
			}
			if raw.MetadataPolicy != nil {
				t.Fatalf("Constraints delete materialized the general metadata policy onto the subordinate")
			}

			setGeneralPolicy(t, backends, generalPolicy("v2"))
			if got := resolvedOrganizationName(t, backends, entityID); got != "v2" {
				t.Fatalf("Expected subordinate to keep live-following general policy 'v2', got %v", got)
			}
		},
	)
}
