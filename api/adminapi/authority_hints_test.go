package adminapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/internal"
	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

type mockAuthorityHintsStore struct {
	listFn   func() ([]smodel.AuthorityHint, error)
	createFn func(smodel.AddAuthorityHint) (*smodel.AuthorityHint, error)
	getFn    func(string) (*smodel.AuthorityHint, error)
	updateFn func(string, smodel.AddAuthorityHint) (*smodel.AuthorityHint, error)
	patchFn  func(string, map[string]any) (*smodel.AuthorityHint, error)
	deleteFn func(string) error
}

func (m *mockAuthorityHintsStore) List() ([]smodel.AuthorityHint, error) {
	return m.listFn()
}

func (m *mockAuthorityHintsStore) Create(item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
	return m.createFn(item)
}

func (m *mockAuthorityHintsStore) Get(ident string) (*smodel.AuthorityHint, error) {
	return m.getFn(ident)
}

func (m *mockAuthorityHintsStore) Update(ident string, item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
	return m.updateFn(ident, item)
}

func (m *mockAuthorityHintsStore) Patch(ident string, patch map[string]any) (*smodel.AuthorityHint, error) {
	return m.patchFn(ident, patch)
}

func (m *mockAuthorityHintsStore) Delete(ident string) error {
	return m.deleteFn(ident)
}

func setupAuthorityHintsTestApp(t *testing.T, store smodel.AuthorityHintsStore, applyHook ...func() error) *fiber.App {
	t.Helper()
	app := fiber.New()
	var hook func() error
	if len(applyHook) > 0 {
		hook = applyHook[0]
	}
	registerAuthorityHints(app, store, hook)
	return app
}

func setEntityConfigurationCache(t *testing.T, value []byte) {
	t.Helper()
	_ = cache.Delete(internal.CacheKeyEntityConfiguration)
	if err := cache.Set(internal.CacheKeyEntityConfiguration, value, time.Minute); err != nil {
		t.Fatalf("failed to seed entity configuration cache: %v", err)
	}
	t.Cleanup(func() {
		_ = cache.Delete(internal.CacheKeyEntityConfiguration)
	})
}

func requireEntityConfigurationCache(t *testing.T, wantSet bool, wantValue []byte) {
	t.Helper()
	var got []byte
	set, err := cache.Get(internal.CacheKeyEntityConfiguration, &got)
	if err != nil {
		t.Fatalf("failed to read entity configuration cache: %v", err)
	}
	if set != wantSet {
		t.Fatalf("expected cache present=%v, got %v", wantSet, set)
	}
	if wantSet && !bytes.Equal(got, wantValue) {
		t.Fatalf("expected cached value %q, got %q", string(wantValue), string(got))
	}
	if !wantSet && len(got) != 0 {
		t.Fatalf("expected empty cached value after invalidation, got %q", string(got))
	}
}

func TestAuthorityHintsList(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			listFn: func() ([]smodel.AuthorityHint, error) {
				return []smodel.AuthorityHint{{ID: 1, EntityID: "https://ta.example", Description: "Trust anchor"}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/entity-configuration/authority-hints/", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var got []smodel.AuthorityHint
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 authority hint, got %d", len(got))
		}
		if got[0].EntityID != "https://ta.example" {
			t.Fatalf("expected entity_id %q, got %q", "https://ta.example", got[0].EntityID)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()

		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			listFn: func() ([]smodel.AuthorityHint, error) {
				return nil, errors.New("db down")
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/entity-configuration/authority-hints/", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusInternalServerError)
	})
}

// TestAuthorityHintsCreate must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache,
// which is shared process-wide state.
func TestAuthorityHintsCreate(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run("SuccessInvalidatesCache", func(t *testing.T) {
		var gotInput smodel.AddAuthorityHint
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				gotInput = item
				return &smodel.AuthorityHint{ID: 10, EntityID: item.EntityID, Description: item.Description}, nil
			},
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":"https://ta.example","description":"Trust anchor"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusCreated)

		var got smodel.AuthorityHint
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if gotInput.EntityID != "https://ta.example" || gotInput.Description != "Trust anchor" {
			t.Fatalf("unexpected create input: %+v", gotInput)
		}
		if got.ID != 10 || got.EntityID != gotInput.EntityID {
			t.Fatalf("unexpected response payload: %+v", got)
		}
		requireEntityConfigurationCache(t, false, nil)
	})

	t.Run("InvalidBodyKeepsCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(_ smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				t.Fatalf("create should not be called for invalid body")
				return nil, nil
			},
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusBadRequest)
		requireEntityConfigurationCache(t, true, cacheValue)
	})

	t.Run("ConflictKeepsCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(_ smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				return nil, smodel.AlreadyExistsError("duplicate authority hint")
			},
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":"https://ta.example"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusConflict)
		requireEntityConfigurationCache(t, true, cacheValue)
	})
}

func TestAuthorityHintsGet(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			getFn: func(ident string) (*smodel.AuthorityHint, error) {
				if ident != "42" {
					t.Fatalf("unexpected authority hint id %q", ident)
				}
				return &smodel.AuthorityHint{ID: 42, EntityID: "https://ta.example", Description: "Trust anchor"}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/entity-configuration/authority-hints/42", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var got smodel.AuthorityHint
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.ID != 42 || got.EntityID != "https://ta.example" {
			t.Fatalf("unexpected response payload: %+v", got)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()

		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			getFn: func(string) (*smodel.AuthorityHint, error) {
				return nil, smodel.NotFoundError("missing")
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/entity-configuration/authority-hints/missing", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNotFound)
	})
}

// TestAuthorityHintsUpdate must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestAuthorityHintsUpdate(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run("SuccessInvalidatesCache", func(t *testing.T) {
		var gotID string
		var gotInput smodel.AddAuthorityHint
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			updateFn: func(ident string, item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				gotID = ident
				gotInput = item
				return &smodel.AuthorityHint{ID: 7, EntityID: item.EntityID, Description: item.Description}, nil
			},
		})

		req := httptest.NewRequest(
			http.MethodPut,
			"/entity-configuration/authority-hints/7",
			strings.NewReader(`{"entity_id":"https://updated.example","description":"Updated"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		var got smodel.AuthorityHint
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if gotID != "7" {
			t.Fatalf("expected authority hint id %q, got %q", "7", gotID)
		}
		if gotInput.EntityID != "https://updated.example" || gotInput.Description != "Updated" {
			t.Fatalf("unexpected update input: %+v", gotInput)
		}
		if got.EntityID != gotInput.EntityID {
			t.Fatalf("unexpected response payload: %+v", got)
		}
		requireEntityConfigurationCache(t, false, nil)
	})

	t.Run("NotFoundKeepsCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			updateFn: func(string, smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				return nil, smodel.NotFoundError("missing")
			},
		})

		req := httptest.NewRequest(
			http.MethodPut,
			"/entity-configuration/authority-hints/missing",
			strings.NewReader(`{"entity_id":"https://updated.example"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNotFound)
		requireEntityConfigurationCache(t, true, cacheValue)
	})

	t.Run("ConflictKeepsCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			updateFn: func(string, smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				return nil, smodel.AlreadyExistsError("duplicate authority hint")
			},
		})

		req := httptest.NewRequest(
			http.MethodPut,
			"/entity-configuration/authority-hints/7",
			strings.NewReader(`{"entity_id":"https://updated.example"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusConflict)
		requireEntityConfigurationCache(t, true, cacheValue)
	})
}

// TestAuthorityHintsDelete must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestAuthorityHintsDelete(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run("SuccessInvalidatesCache", func(t *testing.T) {
		var gotID string
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			deleteFn: func(ident string) error {
				gotID = ident
				return nil
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/entity-configuration/authority-hints/11", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNoContent)
		if gotID != "11" {
			t.Fatalf("expected authority hint id %q, got %q", "11", gotID)
		}
		requireEntityConfigurationCache(t, false, nil)
	})

	t.Run("NotFoundKeepsCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			deleteFn: func(string) error {
				return smodel.NotFoundError("missing")
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/entity-configuration/authority-hints/missing", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNotFound)
		requireEntityConfigurationCache(t, true, cacheValue)
	})
}

// TestAuthorityHintsCreateJWKSSync must NOT use t.Parallel() (uses the global EC cache).
func TestAuthorityHintsCreateJWKSSync(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run("PersistsSyncAndReappliesHook", func(t *testing.T) {
		var gotInput smodel.AddAuthorityHint
		applierCalls := 0
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				gotInput = item
				return &smodel.AuthorityHint{ID: 12, EntityID: item.EntityID, JWKSSync: item.JWKSSync}, nil
			},
		}, func() error {
			applierCalls++
			return nil
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":"https://superior.example","jwks_sync":{"mode":"push","timeout":20,"jwt_lifetime":600}}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusCreated)

		if gotInput.JWKSSync == nil || gotInput.JWKSSync.Mode != smodel.JWKSSyncPush {
			t.Fatalf("expected push jwks_sync in create input, got %+v", gotInput.JWKSSync)
		}
		if applierCalls != 1 {
			t.Fatalf("expected applier to be invoked once, got %d", applierCalls)
		}
		var got smodel.AuthorityHint
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.JWKSSync == nil || got.JWKSSync.Mode != smodel.JWKSSyncPush {
			t.Fatalf("expected jwks_sync in response, got %+v", got.JWKSSync)
		}
		requireEntityConfigurationCache(t, false, nil)
	})

	t.Run("TriggerModeValid", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				return &smodel.AuthorityHint{ID: 13, EntityID: item.EntityID, JWKSSync: item.JWKSSync}, nil
			},
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":"https://superior.example","jwks_sync":{"mode":"trigger"}}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusCreated)
	})

	t.Run("InvalidModeRejected", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		applierCalls := 0
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(_ smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				t.Fatalf("create should not be called for invalid jwks_sync mode")
				return nil, nil
			},
		}, func() error {
			applierCalls++
			return nil
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":"https://superior.example","jwks_sync":{"mode":"bogus"}}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusBadRequest)
		if applierCalls != 0 {
			t.Fatalf("applier must not run on rejected input, got %d calls", applierCalls)
		}
		requireEntityConfigurationCache(t, true, cacheValue)
	})

	t.Run("ApplierErrorStillSucceeds", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			createFn: func(item smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
				return &smodel.AuthorityHint{ID: 14, EntityID: item.EntityID, JWKSSync: item.JWKSSync}, nil
			},
		}, func() error {
			return errors.New("re-attach failed")
		})

		req := httptest.NewRequest(
			http.MethodPost,
			"/entity-configuration/authority-hints/",
			strings.NewReader(`{"entity_id":"https://superior.example","jwks_sync":{"mode":"trigger"}}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusCreated)
	})
}

// TestAuthorityHintsDeleteReappliesHook must NOT use t.Parallel().
func TestAuthorityHintsDeleteReappliesHook(t *testing.T) {
	cacheValue := []byte("cached-entity-config")
	applierCalls := 0
	setEntityConfigurationCache(t, cacheValue)
	app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
		deleteFn: func(string) error { return nil },
	}, func() error {
		applierCalls++
		return nil
	})

	req := httptest.NewRequest(http.MethodDelete, "/entity-configuration/authority-hints/11", http.NoBody)
	resp, bodyBytes := doRequest(t, app, req)
	requireStatus(t, resp, bodyBytes, http.StatusNoContent)
	if applierCalls != 1 {
		t.Fatalf("expected applier to be invoked once, got %d", applierCalls)
	}
}

// TestAuthorityHintsUpdateRequiresEntityID must NOT use t.Parallel().
// PUT is a full replacement: entity_id is a required identity field and is
// never derived from the path or DB id.
func TestAuthorityHintsUpdateRequiresEntityID(t *testing.T) {
	cacheValue := []byte("cached-entity-config")
	setEntityConfigurationCache(t, cacheValue)
	app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
		updateFn: func(string, smodel.AddAuthorityHint) (*smodel.AuthorityHint, error) {
			t.Fatalf("update must not be called when entity_id is missing")
			return nil, nil
		},
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/entity-configuration/authority-hints/7",
		strings.NewReader(`{"jwks_sync":{"mode":"push"}}`),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusBadRequest)
	requireEntityConfigurationCache(t, true, cacheValue)
}

// TestAuthorityHintsPatch must NOT use t.Parallel().
// PATCH partially updates a hint: only provided fields change, entity_id is
// not accepted.
func TestAuthorityHintsPatch(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run("JWKSSyncOnly", func(t *testing.T) {
		var gotID string
		var gotPatch map[string]any
		applierCalls := 0
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			patchFn: func(ident string, patch map[string]any) (*smodel.AuthorityHint, error) {
				gotID = ident
				gotPatch = patch
				return &smodel.AuthorityHint{ID: 7, EntityID: "https://existing.example", JWKSSync: &smodel.JWKSSyncConf{Mode: smodel.JWKSSyncPush}}, nil
			},
		}, func() error {
			applierCalls++
			return nil
		})

		req := httptest.NewRequest(
			http.MethodPatch,
			"/entity-configuration/authority-hints/7",
			strings.NewReader(`{"jwks_sync":{"mode":"push","timeout":25}}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		if gotID != "7" {
			t.Fatalf("expected authority hint id %q, got %q", "7", gotID)
		}
		if _, present := gotPatch["description"]; present {
			t.Fatalf("description must be absent from the patch, got %v", gotPatch)
		}
		sync, ok := gotPatch["jwks_sync"].(map[string]any)
		if !ok || sync["mode"] != "push" || sync["timeout"] != float64(25) {
			t.Fatalf("unexpected jwks_sync in patch: %v", gotPatch["jwks_sync"])
		}
		if applierCalls != 1 {
			t.Fatalf("expected applier to be invoked once, got %d", applierCalls)
		}
		var got smodel.AuthorityHint
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.JWKSSync == nil || got.JWKSSync.Mode != smodel.JWKSSyncPush {
			t.Fatalf("expected jwks_sync in response, got %+v", got.JWKSSync)
		}
		requireEntityConfigurationCache(t, false, nil)
	})

	t.Run("DescriptionOnly", func(t *testing.T) {
		var gotPatch map[string]any
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			patchFn: func(_ string, patch map[string]any) (*smodel.AuthorityHint, error) {
				gotPatch = patch
				return &smodel.AuthorityHint{ID: 7, EntityID: "https://existing.example", Description: "new"}, nil
			},
		})

		req := httptest.NewRequest(
			http.MethodPatch,
			"/entity-configuration/authority-hints/7",
			strings.NewReader(`{"description":"new"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if _, present := gotPatch["jwks_sync"]; present {
			t.Fatalf("jwks_sync must be absent from the patch, got %v", gotPatch)
		}
		if gotPatch["description"] != "new" {
			t.Fatalf("unexpected description in patch: %v", gotPatch["description"])
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			patchFn: func(string, map[string]any) (*smodel.AuthorityHint, error) {
				return nil, smodel.NotFoundError("missing")
			},
		})

		req := httptest.NewRequest(
			http.MethodPatch,
			"/entity-configuration/authority-hints/missing",
			strings.NewReader(`{"description":"new"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusNotFound)
	})

	t.Run("InvalidSyncModeRejected", func(t *testing.T) {
		applierCalls := 0
		setEntityConfigurationCache(t, cacheValue)
		app := setupAuthorityHintsTestApp(t, &mockAuthorityHintsStore{
			patchFn: func(string, map[string]any) (*smodel.AuthorityHint, error) {
				return nil, smodel.ValidationError("jwks_sync: unknown mode (supported: none, push, trigger)")
			},
		}, func() error {
			applierCalls++
			return nil
		})

		req := httptest.NewRequest(
			http.MethodPatch,
			"/entity-configuration/authority-hints/7",
			strings.NewReader(`{"jwks_sync":{"mode":"bogus"}}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusBadRequest)
		if applierCalls != 0 {
			t.Fatalf("applier must not run on rejected input, got %d calls", applierCalls)
		}
	})
}
