package adminapi

import (
	"encoding/json"
	"errors"
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
		t.Parallel()
		app, _ := setupSubordinateLifetimeApp(t)

		req := httptest.NewRequest("GET", "/subordinates/lifetime", http.NoBody)
		resp, body := doRequest(t, app, req)

		requireStatus(t, resp, http.StatusOK)

		var lifetime int
		if err := json.Unmarshal(body, &lifetime); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if lifetime == 0 {
			t.Errorf("Expected a non-zero default lifetime")
		}
	})

	t.Run("PUT and GET Success", func(t *testing.T) {
		t.Parallel()
		app, backends := setupSubordinateLifetimeApp(t)

		// PUT new lifetime
		body := `7200`
		putReq := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader(body))
		putReq.Header.Set("Content-Type", "application/json")
		putResp, _ := doRequest(t, app, putReq)

		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", putResp.StatusCode)
		}

		// GET to verify update
		getReq := httptest.NewRequest("GET", "/subordinates/lifetime", http.NoBody)
		getResp, b := doRequest(t, app, getReq)

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected GET status 200, got %d", getResp.StatusCode)
		}

		var lifetime int
		if err := json.Unmarshal(b, &lifetime); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if lifetime != 7200 {
			t.Errorf("Expected lifetime to be 7200, got %d", lifetime)
		}

		// Verify KV DB Update directly
		var updated int
		found, err := backends.KV.GetAs(model.KeyValueScopeSubordinateStatement, model.KeyValueKeyLifetime, &updated)
		if err != nil {
			t.Fatalf("Failed to get KV value: %v", err)
		}
		if !found || updated != 7200 {
			t.Errorf("Expected lifetime in KV to be 7200")
		}
	})

	t.Run("PUT InvalidBody", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateLifetimeApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader("bad json"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("PUT EmptyBody", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateLifetimeApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("PUT NegativeValue", func(t *testing.T) {
		t.Parallel()
		app, _ := setupSubordinateLifetimeApp(t)

		req := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader("-100"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("GET StorageError", func(t *testing.T) {
		t.Parallel()
		kv := &mockKeyValueStore{
			getAsFn: func(_, _ string, _ any) (bool, error) {
				return false, errors.New("db read error")
			},
		}
		app := fiber.New()
		registerGeneralSubordinateLifetime(app, kv)

		req := httptest.NewRequest("GET", "/subordinates/lifetime", http.NoBody)
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})

	t.Run("PUT StorageError", func(t *testing.T) {
		t.Parallel()
		kv := &mockKeyValueStore{
			setAnyFn: func(_, _ string, _ any) error {
				return errors.New("db write error")
			},
		}
		app := fiber.New()
		registerGeneralSubordinateLifetime(app, kv)

		req := httptest.NewRequest("PUT", "/subordinates/lifetime", strings.NewReader("3600"))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := doRequest(t, app, req)

		assertStatus(t, resp, http.StatusInternalServerError)
	})
}
