package adminapi

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-oidfed/lib/cache"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/internal"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func setCacheEntry(t *testing.T, key string, value []byte) {
	t.Helper()
	_ = cache.Delete(key)
	if err := cache.Set(key, value, time.Minute); err != nil {
		t.Fatalf("failed to seed cache for %q: %v", key, err)
	}
	t.Cleanup(func() {
		_ = cache.Delete(key)
	})
}

func requireCacheEntry(t *testing.T, key string, wantSet bool, wantValue []byte) {
	t.Helper()
	var got []byte
	set, err := cache.Get(key, &got)
	if err != nil {
		t.Fatalf("failed to read cache key %q: %v", key, err)
	}
	if set != wantSet {
		t.Fatalf("expected cache present=%v for %q, got %v", wantSet, key, set)
	}
	if wantSet && !bytes.Equal(got, wantValue) {
		t.Fatalf("expected cached value %q for %q, got %q", string(wantValue), key, string(got))
	}
	if !wantSet && len(got) != 0 {
		t.Fatalf("expected empty cached value for cleared key %q, got %q", key, string(got))
	}
}

// TestEntityConfigurationCacheInvalidationMiddleware must NOT use t.Parallel().
// It operates on the global process-wide cache (cache.Set/Get/Delete), which is
// shared mutable state. Parallelizing these subtests would cause race conditions
// on the entity configuration cache key.
func TestEntityConfigurationCacheInvalidationMiddleware(t *testing.T) {
	cacheValue := []byte("entity-config-jwt")

	t.Run("SuccessDeletesCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := fiber.New()
		app.Post("/entity-config", entityConfigurationCacheInvalidationMiddleware, func(c *fiber.Ctx) error {
			return c.SendStatus(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodPost, "/entity-config", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNoContent)
		requireEntityConfigurationCache(t, false, nil)
	})

	t.Run("FailureKeepsCache", func(t *testing.T) {
		setEntityConfigurationCache(t, cacheValue)
		app := fiber.New()
		app.Post("/entity-config", entityConfigurationCacheInvalidationMiddleware, func(c *fiber.Ctx) error {
			return c.SendStatus(http.StatusBadRequest)
		})

		req := httptest.NewRequest(http.MethodPost, "/entity-config", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusBadRequest)
		requireEntityConfigurationCache(t, true, cacheValue)
	})

	// TODO: Currently there are no redirects. If we add any in the future, we should verify that they do not clear the cache.
	// t.Run("RedirectDoesNotClearCache", func(t *testing.T) {
	// 	setEntityConfigurationCache(t, cacheValue)
	//
	// 	app := fiber.New()
	// 	app.Post("/entity-config", entityConfigurationCacheInvalidationMiddleware, func(c *fiber.Ctx) error {
	// 		return c.Redirect("/other", fiber.StatusMovedPermanently)
	// 	})
	//
	// 	req := httptest.NewRequest(http.MethodPost, "/entity-config", http.NoBody)
	// 	resp, bodyBytes := doRequest(t, app, req)
	//
	// 	requireStatus(t, resp, bodyBytes, fiber.StatusMovedPermanently)
	// 	requireEntityConfigurationCache(t, true, cacheValue)
	// })
}

// TestSubordinateStatementsCacheInvalidationMiddleware must NOT use t.Parallel().
// It operates on the global process-wide cache (cache.Set/Get/Delete), which is
// shared mutable state. Parallelizing these subtests would cause race conditions
// on the subordinate statement cache keys.
//
// The fetch endpoint caches statements keyed by entity ID, but admin routes
// identify subordinates by DB primary key. The middleware closure resolves the
// DB ID to the entity ID before deleting the cache entry.
func TestSubordinateStatementsCacheInvalidationMiddleware(t *testing.T) {
	entity1 := "https://sub1.example"
	entity2 := "https://sub2.example"
	key1 := internal.SubordinateStatementCacheKey(entity1)
	key2 := internal.SubordinateStatementCacheKey(entity2)
	value1 := []byte("statement-1")
	value2 := []byte("statement-2")

	// Helper to build a storage backend with two subordinates whose DB IDs
	// are known, so tests can send requests with :subordinateID = DB ID.
	setupStorage := func(t *testing.T) (model.SubordinateStorageBackend, string, string) {
		t.Helper()
		store := newSubordinateTestStorage(t)
		require.NoError(t, store.SubordinateStorage().Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: entity1,
				Status:   model.StatusActive,
			},
		}))
		require.NoError(t, store.SubordinateStorage().Add(model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: entity2,
				Status:   model.StatusActive,
			},
		}))
		sub1, err := store.SubordinateStorage().Get(entity1)
		require.NoError(t, err)
		sub2, err := store.SubordinateStorage().Get(entity2)
		require.NoError(t, err)
		return store.SubordinateStorage(), fmt.Sprintf("%d", sub1.ID), fmt.Sprintf("%d", sub2.ID)
	}

	t.Run("SpecificSubordinateDeletesOnlyTarget", func(t *testing.T) {
		store, dbID1, _ := setupStorage(t)
		setCacheEntry(t, key1, value1)
		setCacheEntry(t, key2, value2)

		app := fiber.New()
		app.Delete(
			"/subordinates/:subordinateID",
			subordinateStatementsCacheInvalidationMiddleware(store),
			func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) },
		)

		req := httptest.NewRequest(http.MethodDelete, "/subordinates/"+dbID1, http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNoContent)
		requireCacheEntry(t, key1, false, nil)
		requireCacheEntry(t, key2, true, value2)
	})

	t.Run("CollectionSuccessClearsAll", func(t *testing.T) {
		setCacheEntry(t, key1, value1)
		setCacheEntry(t, key2, value2)

		app := fiber.New()
		app.Post(
			"/subordinates",
			subordinateStatementsCacheInvalidationMiddleware(nil),
			func(c *fiber.Ctx) error { return c.SendStatus(http.StatusCreated) },
		)

		req := httptest.NewRequest(http.MethodPost, "/subordinates", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusCreated)
		requireCacheEntry(t, key1, false, nil)
		requireCacheEntry(t, key2, false, nil)
	})

	t.Run("LookupFailureClearsAll", func(t *testing.T) {
		store := newSubordinateTestStorage(t).SubordinateStorage()
		setCacheEntry(t, key1, value1)
		setCacheEntry(t, key2, value2)

		app := fiber.New()
		app.Delete(
			"/subordinates/:subordinateID",
			subordinateStatementsCacheInvalidationMiddleware(store),
			func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) },
		)

		// Use a nonexistent DB ID — lookup fails, middleware falls back to Clear all.
		req := httptest.NewRequest(http.MethodDelete, "/subordinates/9999", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusNoContent)
		requireCacheEntry(t, key1, false, nil)
		requireCacheEntry(t, key2, false, nil)
	})

	t.Run("FailureKeepsAll", func(t *testing.T) {
		store, dbID1, _ := setupStorage(t)
		setCacheEntry(t, key1, value1)
		setCacheEntry(t, key2, value2)

		app := fiber.New()
		app.Delete(
			"/subordinates/:subordinateID",
			subordinateStatementsCacheInvalidationMiddleware(store),
			func(c *fiber.Ctx) error { return c.SendStatus(http.StatusInternalServerError) },
		)

		req := httptest.NewRequest(http.MethodDelete, "/subordinates/"+dbID1, http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusInternalServerError)
		requireCacheEntry(t, key1, true, value1)
		requireCacheEntry(t, key2, true, value2)
	})
}
