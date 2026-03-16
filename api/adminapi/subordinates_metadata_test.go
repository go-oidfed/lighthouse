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

func setupSubordinateMetadataApp(t *testing.T) (*fiber.App, model.Backends) {
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
	registerSubordinateMetadata(app, backends)
	return app, backends
}

// --- GET & PUT /subordinates/:subordinateID/metadata TESTS ---

func TestGetSubordinateMetadata(t *testing.T) {
	t.Parallel()
	t.Run("Success/WithMetadata", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		meta := &oidfed.Metadata{
			RelyingParty: &oidfed.OpenIDRelyingPartyMetadata{
				ClientName: "My App",
			},
		}

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-get.example.org",
			},
			Metadata: meta,
		})
		saved, _ := backends.Subordinates.Get("https://meta-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		json.Unmarshal(body, &result)

		if rp, ok := result["openid_relying_party"].(map[string]any); !ok || rp["client_name"] != "My App" {
			t.Errorf("Failed to retrieve correctly unmarshaled metadata: %+v", result)
		}
	})

	t.Run("NoMetadata", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://no-meta.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://no-meta.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateMetadataApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999/metadata", http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}

func TestPutSubordinateMetadata(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-put.example.org")

		body := `{
			"openid_relying_party": {
				"client_name": "New App Name"
			}
		}`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://meta-put.example.org")
		if updated.Metadata == nil {
			t.Fatalf("Expected Metadata to be saved in DB, got nil")
		}

		rpMeta := updated.Metadata.RelyingParty
		if rpMeta.ClientName != "New App Name" {
			t.Errorf("Expected 'New App Name', got: %+v", rpMeta.ClientName)
		}

		// Verify Event logging
		events, _, _ := backends.SubordinateEvents.GetBySubordinateID(saved.ID, model.EventQueryOpts{})
		found := false
		for _, e := range events {
			if e.Type == model.EventTypeMetadataUpdated {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected MetadataUpdated event to be logged")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateMetadataApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/9999/metadata", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}

// --- GET /subordinates/:subordinateID/metadata/:entityType TESTS ---

func TestGetSubordinateMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		meta := &oidfed.Metadata{
			Extra: map[string]any{
				"custom_entity_type": map[string]any{
					"custom_claim": "hello",
				},
			},
		}

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-type-get.example.org",
			},
			Metadata: meta,
		})
		saved, _ := backends.Subordinates.Get("https://meta-type-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata/custom_entity_type", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		json.Unmarshal(body, &result)

		if result["custom_claim"] != "hello" {
			t.Errorf("Failed to retrieve entity type metadata: %+v", result)
		}
	})

	t.Run("NotFound/Subordinate", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateMetadataApp(t)
		req := httptest.NewRequest("GET", "/subordinates/9999/metadata/custom", http.NoBody)
		resp, _ := app.Test(req, -1)
		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})

	t.Run("NotFound/EntityType", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-meta-type.example.org",
			},
			Metadata: &oidfed.Metadata{},
		})
		saved, _ := backends.Subordinates.Get("https://missing-meta-type.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata/missing_type", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

// --- PUT /subordinates/:subordinateID/metadata/:entityType TESTS ---

func TestPutSubordinateMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-type-put.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"old_type":    map[string]any{"claim": "keep_me"},
					"target_type": map[string]any{"claim": "delete_me"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-type-put.example.org")

		body := `{"new_claim": "new_value"}`

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata/target_type", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://meta-type-put.example.org")
		extra := updated.Metadata.Extra

		if extra["old_type"] == nil {
			t.Errorf("Expected non-target entity types to be untouched")
		}

		target := extra["target_type"].(map[string]any)
		if target["claim"] != nil {
			t.Errorf("Expected old claim to be wiped by PUT replacement")
		}
		if target["new_claim"] != "new_value" {
			t.Errorf("Expected new claim to be set")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-meta-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-meta-put.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata/target_type", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// --- POST & DELETE /subordinates/:subordinateID/metadata/:entityType TESTS ---

func TestPostSubordinateMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run("Success/Merge", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-type-post.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"target_type": map[string]any{"existing_claim": "kept"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-type-post.example.org")

		body := `{"new_claim": "merged"}`

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/metadata/target_type", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		updated, _ := backends.Subordinates.Get("https://meta-type-post.example.org")
		target := updated.Metadata.Extra["target_type"].(map[string]any)

		if target["existing_claim"] != "kept" {
			t.Errorf("Expected existing claim to be kept during merge")
		}
		if target["new_claim"] != "merged" {
			t.Errorf("Expected new claim to be merged in")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-meta-post.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-meta-post.example.org")

		req := httptest.NewRequest("POST", fmt.Sprintf("/subordinates/%d/metadata/target_type", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestDeleteSubordinateMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-type-delete.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"delete_me": map[string]any{"claim": "bye"},
					"keep_me":   map[string]any{"claim": "stay"},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-type-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata/delete_me", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNoContent)

		updated, _ := backends.Subordinates.Get("https://meta-type-delete.example.org")
		extra := updated.Metadata.Extra

		if extra["delete_me"] != nil {
			t.Errorf("Expected delete_me entity type to be entirely removed")
		}
		if extra["keep_me"] == nil {
			t.Errorf("Expected keep_me entity type to be safely retained")
		}
	})

	t.Run("NotFound/Subordinate", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateMetadataApp(t)
		req := httptest.NewRequest("DELETE", "/subordinates/9999/metadata/delete_me", http.NoBody)
		resp, _ := app.Test(req, -1)
		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})

	t.Run("NotFound/EntityType", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-meta-delete-type.example.org",
			},
			Metadata: &oidfed.Metadata{},
		})
		saved, _ := backends.Subordinates.Get("https://missing-meta-delete-type.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata/missing_type", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

// --- /subordinates/:subordinateID/metadata/:entityType/:claim TESTS ---

func TestGetSubordinateMetadataClaim(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-claim-get.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"target_type": map[string]any{
						"target_claim": "found_it",
						"other_claim":  "ignore_me",
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-claim-get.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata/target_type/target_claim", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result string
		json.Unmarshal(body, &result)

		if result != "found_it" {
			t.Errorf("Failed to retrieve claim metadata: got %s", result)
		}
	})

	t.Run("NotFound/Claim", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-meta-claim.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"target_type": map[string]any{},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://missing-meta-claim.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata/target_type/missing", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusNotFound)
	})
}

func TestPutSubordinateMetadataClaim(t *testing.T) {
	t.Parallel()
	t.Run("Success/Replace", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-claim-put.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"target_type": map[string]any{
						"target_claim": "old_value",
						"safe_claim":   "untouched",
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-claim-put.example.org")

		body := `"new_value"` // Notice we send just a JSON string here since it is a single claim value

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata/target_type/target_claim", saved.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		updated, _ := backends.Subordinates.Get("https://meta-claim-put.example.org")
		target := updated.Metadata.Extra["target_type"].(map[string]any)

		if target["safe_claim"] != "untouched" {
			t.Errorf("Expected sibling claim to remain untouched")
		}
		if target["target_claim"] != "new_value" {
			t.Errorf("Expected target claim to be fully replaced")
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://bad-body-meta-claim-put.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://bad-body-meta-claim-put.example.org")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/subordinates/%d/metadata/target_type/target_claim", saved.ID), strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusBadRequest)
	})
}

func TestDeleteSubordinateMetadataClaim(t *testing.T) {
	t.Parallel()
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://meta-claim-delete.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"target_type": map[string]any{
						"delete_me": "gone",
						"keep_me":   "staying",
					},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://meta-claim-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata/target_type/delete_me", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusNoContent)

		updated, _ := backends.Subordinates.Get("https://meta-claim-delete.example.org")
		target := updated.Metadata.Extra["target_type"].(map[string]any)

		if _, ok := target["delete_me"]; ok {
			t.Errorf("Expected claim delete_me to be deleted")
		}
		if _, ok := target["keep_me"]; !ok {
			t.Errorf("Expected claim keep_me to be safely retained")
		}
	})

	t.Run("NotFound/Claim", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateMetadataApp(t)
		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://missing-meta-claim-delete.example.org",
			},
			Metadata: &oidfed.Metadata{
				Extra: map[string]any{
					"target_type": map[string]any{},
				},
			},
		})
		saved, _ := backends.Subordinates.Get("https://missing-meta-claim-delete.example.org")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d/metadata/target_type/not_here", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatus(t, resp, http.StatusNotFound)
	})
}
