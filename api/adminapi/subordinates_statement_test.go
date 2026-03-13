package adminapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jws"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// mockFedEntity implements oidfed.FederationEntity just enough for the statement generation to not panic
type mockFedEntity struct{}

func (m mockFedEntity) EntityID() string {
	return "https://lighthouse.example.org"
}

func (m mockFedEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return nil, nil
}
func (m mockFedEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (m mockFedEntity) SignEntityStatement(payload oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (m mockFedEntity) SignEntityStatementWithHeaders(payload oidfed.EntityStatementPayload, headers jws.Headers) ([]byte, error) {
	return nil, nil
}

func setupSubordinateStatementApp(t *testing.T) (*fiber.App, model.Backends) {
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
	
	// registerSubordinateStatement takes the router, subordinate backend, KV, and the FederationEntity.
	g := app.Group("/subordinates/:subordinateID/statement")
	g.Get("/", handleGetSubordinateStatement(backends.Subordinates, backends.KV, mockFedEntity{}))
	
	return app, backends
}

// --- GET /subordinates/:subordinateID/statement TESTS ---

func TestSubordinateStatement(t *testing.T) {
	t.Parallel()
	t.Run("GET Success", func(t *testing.T) {
			t.Parallel()
		app, backends := setupSubordinateStatementApp(t)

		backends.Subordinates.Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: "https://statement.example.org",
			},
		})
		saved, _ := backends.Subordinates.Get("https://statement.example.org")

		req := httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/statement", saved.ID), http.NoBody)
		resp, _ := app.Test(req, -1)

		requireStatus(t, resp, http.StatusOK)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		json.Unmarshal(body, &result)

		if result["iss"] != "https://lighthouse.example.org" {
			t.Errorf("Expected issuer to be lighthouse, got %v", result["iss"])
		}
		if result["sub"] != "https://statement.example.org" {
			t.Errorf("Expected subject to be subordinate entity ID, got %v", result["sub"])
		}
	})

	t.Run("GET NotFound", func(t *testing.T) {
			t.Parallel()
		app, _ := setupSubordinateStatementApp(t)

		req := httptest.NewRequest("GET", "/subordinates/9999/statement", http.NoBody)
		resp, _ := app.Test(req, -1)

		assertStatusOneOf(t, resp, http.StatusNotFound, http.StatusInternalServerError)
	})
}
