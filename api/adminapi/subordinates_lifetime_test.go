package adminapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

func setupSubordinateLifetimeApp(t *testing.T) (*fiber.App, model.Backends) {
	t.Helper()
	store := newSubordinateTestStorage(t)

	backends := model.Backends{
		KV: store.KeyValue(),
	}

	app := fiber.New()
	registerGeneralSubordinateLifetime(app, backends.KV)
	return app, backends
}

func TestSubordinateLifetime(t *testing.T) {
	t.Parallel()
	t.Run("GET Success/Default", func(t *testing.T) {
		app, _ := setupSubordinateLifetimeApp(t)

		req := httptest.NewRequest("GET", "/subordinates/lifetime", http.NoBody)
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var lifetime int
		json.Unmarshal(body, &lifetime)

		if lifetime == 0 {
			t.Errorf("Expected a non-zero default lifetime")
		}
	})

	t.Run("PUT and GET Success", func(t *testing.T) {
		app, backends := setupSubordinateLifetimeApp(t)

		// PUT new lifetime
		body := `7200`
		putReq := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader(body))
		putReq.Header.Set("Content-Type", "application/json")
		putResp, _ := app.Test(putReq, -1)

		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", putResp.StatusCode)
		}

		// GET to verify update
		getReq := httptest.NewRequest("GET", "/subordinates/lifetime", http.NoBody)
		getResp, _ := app.Test(getReq, -1)

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected GET status 200, got %d", getResp.StatusCode)
		}

		b, _ := io.ReadAll(getResp.Body)
		var lifetime int
		json.Unmarshal(b, &lifetime)

		if lifetime != 7200 {
			t.Errorf("Expected lifetime to be 7200, got %d", lifetime)
		}

		// Verify KV DB Update directly
		var updated int
		found, _ := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyLifetime, &updated)
		if !found || updated != 7200 {
			t.Errorf("Expected lifetime in KV to be 7200")
		}
	})

	t.Run("PUT InvalidBody", func(t *testing.T) {
		app, _ := setupSubordinateLifetimeApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}
