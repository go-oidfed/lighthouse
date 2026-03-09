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
	t.Run("Success/WithMetadata", func(t *testing.T) {
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

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		json.Unmarshal(body, &result)

		if rp, ok := result["openid_relying_party"].(map[string]any); !ok || rp["client_name"] != "My App" {
			t.Errorf("Failed to retrieve correctly unmarshaled metadata: %+v", result)
		}
	})

	t.Run("NoMetadata", func(t *testing.T) {
		app, backends := setupSubordinateMetadataApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://no-meta.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://no-meta.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/metadata", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 when metadata is missing, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateMetadataApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999/metadata", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}

func TestPutSubordinateMetadata(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
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

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(b))
		}

		// Verify DB update
		updated, _ := backends.Subordinates.Get("https://meta-put.example.org")
		if updated.Metadata == nil {
			t.Fatalf("Expected Metadata to be saved in DB, got nil")
		}
		
		rpMeta := (*updated.Metadata).RelyingParty
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

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _ := setupSubordinateMetadataApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/9999/metadata", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", resp.StatusCode)
		}
	})
}
