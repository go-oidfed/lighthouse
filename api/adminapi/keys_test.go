package adminapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

// --- MOCKS ---

// mockBasicKMS implements kms.BasicKeyManagementSystem with a fixed ES256 default algorithm.
type mockBasicKMS struct {
	kms.BasicKeyManagementSystem
}

func (*mockBasicKMS) GetDefaultAlg() jwa.SignatureAlgorithm {
	return jwa.ES256()
}

// mockFullKMS implements kms.KeyManagementSystem for testing endpoints that require
// the full KMS interface (rotation, algorithm changes, etc.).
type mockFullKMS struct {
	kms.KeyManagementSystem
}

func (*mockFullKMS) ChangeKeyRotationConfig(_ kms.KeyRotationConfig) error { return nil }
func (*mockFullKMS) ChangeRSAKeyLength(_ int) error                        { return nil }
func (*mockFullKMS) RotateAllKeys(_ bool, _ string) error                  { return nil }
func (*mockFullKMS) ChangeAlgsAt(_ []jwa.SignatureAlgorithm, _ unixtime.Unixtime, _ time.Duration) error {
	return nil
}
func (*mockFullKMS) ChangeDefaultAlgorithmAt(_ jwa.SignatureAlgorithm, _ unixtime.Unixtime) error {
	return nil
}
func (*mockFullKMS) GetPendingChanges() (*kms.PendingAlgChange, *kms.PendingDefaultChange) {
	return nil, nil
}

// --- TEST HELPERS ---

// testRSAKeyN is a valid RSA modulus reused across all key tests to avoid duplication.
const testRSAKeyN = "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"

// newTestStorage creates a unique in-memory SQLite database for testing.
func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", url.PathEscape(t.Name()))
	store, err := storage.NewStorage(storage.Config{
		Driver: storage.DriverSQLite,
		DSN:    dsn,
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	return store
}

// newTestKeyBody returns a JSON body for adding a public key with the given kid.
func newTestKeyBody(kid string) string {
	return fmt.Sprintf(`{
		"key": {
			"kty": "RSA",
			"n": "%s",
			"e": "AQAB",
			"kid": "%s"
		}
	}`, testRSAKeyN, kid)
}

// setupPublicKeyApp creates a Fiber app configured with API-managed key storage.
// Returns the app and the KeyManagement struct for DB assertions.
func setupPublicKeyApp(t *testing.T) (*fiber.App, KeyManagement, *storage.Storage) {
	t.Helper()
	store := newTestStorage(t)
	km := KeyManagement{
		APIManagedPKs: store.DBPublicKeyStorage("api-managed"),
	}
	if err := km.APIManagedPKs.Load(); err != nil {
		t.Fatalf("Failed to create public key table: %v", err)
	}
	app := fiber.New()
	backends := model.Backends{
		KV:         store.KeyValue(),
		PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
		Transaction: func(fn model.TransactionFunc) error {
			return fn(&model.Backends{
				KV:         store.KeyValue(),
				PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			})
		},
	}
	registerKeys(app, km, store.KeyValue(), backends)
	return app, km, store
}

// injectTestKey adds a public key via the API and asserts success.
func injectTestKey(t *testing.T, app *fiber.App, kid string) {
	t.Helper()
	body := newTestKeyBody(kid)
	req := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to inject test key %q: %v", kid, err)
	}
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Inject key %q: expected 201, got %d: %s", kid, resp.StatusCode, string(b))
	}
}

// doRequest is a tiny helper to execute an HTTP request against a Fiber app and
// return the response plus the fully-read body.

// --- PUBLIC KEY MANAGEMENT TESTS ---

func TestPostPublicKey(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, km, _ := setupPublicKeyApp(t)

		body := newTestKeyBody("my-test-key-1")
		req := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 201 {
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify the response body contains the key
		var created map[string]any
		if err := json.Unmarshal(respBody, &created); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if created["kid"] != "my-test-key-1" {
			t.Errorf("Response kid: expected 'my-test-key-1', got %q", created["kid"])
		}

		// Verify it persisted to the database
		savedKey, err := km.APIManagedPKs.Get("my-test-key-1")
		if err != nil {
			t.Fatalf("Failed to fetch key from database: %v", err)
		}
		if savedKey == nil {
			t.Fatal("Expected to find saved key in database, got nil")
		}
		if savedKey.KID != "my-test-key-1" {
			t.Errorf("Expected KID 'my-test-key-1', got %q", savedKey.KID)
		}
	})

	t.Run("MissingKey", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		// Send a body with no key field
		req := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("KidMismatch", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		// The top-level "kid" doesn't match the key's embedded "kid"
		body := fmt.Sprintf(`{
			"kid": "wrong-kid",
			"key": {
				"kty": "RSA",
				"n": "%s",
				"e": "AQAB",
				"kid": "actual-kid"
			}
		}`, testRSAKeyN)
		req := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		req := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(`not valid json`))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}

func TestGetPublicKeys(t *testing.T) {
	t.Run("ReturnsInjectedKeys", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "key-1")
		injectTestKey(t, app, "key-2")

		req := httptest.NewRequest("GET", "/entity-configuration/keys/", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		var keys []map[string]any
		if err := json.Unmarshal(respBody, &keys); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(keys))
		}
	})

	t.Run("EmptyWhenNoKeys", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		req := httptest.NewRequest("GET", "/entity-configuration/keys/", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		var keys []map[string]any
		if err := json.Unmarshal(respBody, &keys); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("Expected 0 keys, got %d", len(keys))
		}
	})
}

func TestDeletePublicKey(t *testing.T) {
	t.Run("HardDelete", func(t *testing.T) {
		app, km, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "key-to-delete")

		// Sanity check key exists
		if k, _ := km.APIManagedPKs.Get("key-to-delete"); k == nil {
			t.Fatal("Setup failed: key was not inserted")
		}

		req := httptest.NewRequest("DELETE", "/entity-configuration/keys/key-to-delete", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 204 {
			t.Errorf("Expected status 204, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify key is gone from database
		deletedKey, err := km.APIManagedPKs.Get("key-to-delete")
		if err != nil {
			t.Fatalf("Error fetching key from database: %v", err)
		}
		if deletedKey != nil {
			t.Error("Expected key to be deleted, but it still exists")
		}
	})

	t.Run("RevokeInsteadOfDelete", func(t *testing.T) {
		app, km, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "key-to-revoke")

		req := httptest.NewRequest("DELETE", "/entity-configuration/keys/key-to-revoke?revoke=true&reason=compromised", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 204 {
			t.Errorf("Expected status 204, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Key should still exist but be revoked
		revokedKey, err := km.APIManagedPKs.Get("key-to-revoke")
		if err != nil {
			t.Fatalf("Error fetching key from database: %v", err)
		}
		if revokedKey == nil {
			t.Fatal("Revoked key should still exist in database")
		}
		if revokedKey.RevokedAt == nil {
			t.Error("Expected RevokedAt to be set on revoked key")
		}
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		// Deleting a non-existent key should still return 204 (idempotent)
		req := httptest.NewRequest("DELETE", "/entity-configuration/keys/does-not-exist", http.NoBody)
		resp, _ := doRequest(t, app, req)

		if resp.StatusCode != 204 {
			t.Errorf("Expected status 204 for idempotent delete, got %d", resp.StatusCode)
		}
	})
}

func TestUpdatePublicKeyExp(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, km, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "key-to-update")

		body := `{"exp": 2000000000}`
		req := httptest.NewRequest("PUT", "/entity-configuration/keys/key-to-update", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify response body
		var updated map[string]any
		if err := json.Unmarshal(respBody, &updated); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if updated["kid"] != "key-to-update" {
			t.Errorf("Response kid mismatch: got %q", updated["kid"])
		}

		// Verify in database
		updatedKey, err := km.APIManagedPKs.Get("key-to-update")
		if err != nil {
			t.Fatalf("Error fetching key: %v", err)
		}
		if updatedKey == nil {
			t.Fatal("Expected to find key in database, got nil")
		}
		if updatedKey.ExpiresAt == nil {
			t.Fatal("Expected ExpiresAt to be set, but it was nil")
		}
		if updatedKey.ExpiresAt.Unix() != 2000000000 {
			t.Errorf("Expected ExpiresAt 2000000000, got %d", updatedKey.ExpiresAt.Unix())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		body := `{"exp": 2000000000}`
		req := httptest.NewRequest("PUT", "/entity-configuration/keys/nonexistent-key", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "key-bad-update")

		req := httptest.NewRequest("PUT", "/entity-configuration/keys/key-bad-update", strings.NewReader(`not json`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}

func TestRotatePublicKey(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, km, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "old-key")

		newKeyBody := fmt.Sprintf(`{
			"key": {
				"kty": "RSA",
				"n": "%s",
				"e": "AQAB",
				"kid": "new-key"
			}
		}`, testRSAKeyN)

		req := httptest.NewRequest("POST", "/entity-configuration/keys/old-key", strings.NewReader(newKeyBody))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 201 {
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify the response body contains the new key
		var created map[string]any
		if err := json.Unmarshal(respBody, &created); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if created["kid"] != "new-key" {
			t.Errorf("Response kid: expected 'new-key', got %q", created["kid"])
		}

		// Old key should now have an expiration
		oldKey, err := km.APIManagedPKs.Get("old-key")
		if err != nil {
			t.Fatalf("Failed to fetch old key: %v", err)
		}
		if oldKey == nil {
			t.Fatal("Old key should still exist after rotation")
		}
		if oldKey.ExpiresAt == nil {
			t.Error("Old key should have ExpiresAt set after rotation")
		}

		// New key should exist
		newKey, err := km.APIManagedPKs.Get("new-key")
		if err != nil {
			t.Fatalf("Failed to fetch new key: %v", err)
		}
		if newKey == nil {
			t.Fatal("New key should exist after rotation")
		}
	})

	t.Run("OldKeyNotFound", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)

		newKeyBody := newTestKeyBody("new-key")
		req := httptest.NewRequest("POST", "/entity-configuration/keys/nonexistent-old-key", strings.NewReader(newKeyBody))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("MissingKey", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "existing-key")

		req := httptest.NewRequest("POST", "/entity-configuration/keys/existing-key", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app, _, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "existing-key-2")

		req := httptest.NewRequest("POST", "/entity-configuration/keys/existing-key-2", strings.NewReader(`not json`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("WithCustomOldKeyExp", func(t *testing.T) {
		app, km, _ := setupPublicKeyApp(t)
		injectTestKey(t, app, "rotate-custom-old")

		newKeyBody := fmt.Sprintf(`{
			"key": {
				"kty": "RSA",
				"n": "%s",
				"e": "AQAB",
				"kid": "rotate-custom-new"
			},
			"old_key_exp": 1900000000
		}`, testRSAKeyN)

		req := httptest.NewRequest("POST", "/entity-configuration/keys/rotate-custom-old", strings.NewReader(newKeyBody))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 201 {
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify old key has the custom expiration
		oldKey, err := km.APIManagedPKs.Get("rotate-custom-old")
		if err != nil {
			t.Fatalf("Failed to fetch old key: %v", err)
		}
		if oldKey == nil || oldKey.ExpiresAt == nil {
			t.Fatal("Old key should have ExpiresAt set")
		}
		if oldKey.ExpiresAt.Unix() != 1900000000 {
			t.Errorf("Old key ExpiresAt: expected 1900000000, got %d", oldKey.ExpiresAt.Unix())
		}
	})
}

// --- JWKS TESTS ---

func TestGetEntityConfigurationJWKS(t *testing.T) {
	t.Run("ReturnsValidKeys", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			APIManagedPKs: store.DBPublicKeyStorage("api-managed"),
			KMSManagedPKs: store.DBPublicKeyStorage("kms-managed"),
		}
		if err := km.APIManagedPKs.Load(); err != nil {
			t.Fatalf("Failed to load API-managed PKs: %v", err)
		}
		if err := km.KMSManagedPKs.Load(); err != nil {
			t.Fatalf("Failed to load KMS-managed PKs: %v", err)
		}

		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		injectTestKey(t, app, "jwks-key-1")

		req := httptest.NewRequest("GET", "/entity-configuration/jwks", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		var jwks struct {
			Keys []map[string]any `json:"keys"`
		}
		if err := json.Unmarshal(respBody, &jwks); err != nil {
			t.Fatalf("Failed to parse JWKS: %v", err)
		}
		if len(jwks.Keys) != 1 {
			t.Errorf("Expected 1 key in JWKS, got %d", len(jwks.Keys))
		} else if jwks.Keys[0]["kid"] != "jwks-key-1" {
			t.Errorf("Expected kid 'jwks-key-1', got %q", jwks.Keys[0]["kid"])
		}
	})

	t.Run("EmptyWhenNoKeys", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			APIManagedPKs: store.DBPublicKeyStorage("api-managed"),
			KMSManagedPKs: store.DBPublicKeyStorage("kms-managed"),
		}
		if err := km.APIManagedPKs.Load(); err != nil {
			t.Fatalf("Failed to load API-managed PKs: %v", err)
		}
		if err := km.KMSManagedPKs.Load(); err != nil {
			t.Fatalf("Failed to load KMS-managed PKs: %v", err)
		}

		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("GET", "/entity-configuration/jwks", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		var jwks struct {
			Keys []map[string]any `json:"keys"`
		}
		if err := json.Unmarshal(respBody, &jwks); err != nil {
			t.Fatalf("Failed to parse JWKS: %v", err)
		}
		if len(jwks.Keys) != 0 {
			t.Errorf("Expected 0 keys in JWKS, got %d", len(jwks.Keys))
		}
	})

	t.Run("ExcludesExpiredKeys", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			APIManagedPKs: store.DBPublicKeyStorage("api-managed"),
			KMSManagedPKs: store.DBPublicKeyStorage("kms-managed"),
		}
		if err := km.APIManagedPKs.Load(); err != nil {
			t.Fatalf("Failed to load API-managed PKs: %v", err)
		}
		if err := km.KMSManagedPKs.Load(); err != nil {
			t.Fatalf("Failed to load KMS-managed PKs: %v", err)
		}

		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		// Inject a key, then expire it
		injectTestKey(t, app, "expired-key")

		expBody := `{"exp": 1000000000}`
		expReq := httptest.NewRequest("PUT", "/entity-configuration/keys/expired-key", strings.NewReader(expBody))
		expReq.Header.Set("Content-Type", "application/json")
		doRequest(t, app, expReq)

		// JWKS should not include the expired key
		req := httptest.NewRequest("GET", "/entity-configuration/jwks", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var jwks struct {
			Keys []map[string]any `json:"keys"`
		}
		if err := json.Unmarshal(respBody, &jwks); err != nil {
			t.Fatalf("Failed to parse JWKS: %v", err)
		}
		if len(jwks.Keys) != 0 {
			t.Errorf("Expected 0 keys in JWKS (expired should be excluded), got %d", len(jwks.Keys))
		}
	})
}

// --- KMS MANAGEMENT TESTS ---

func TestGetKMSInfo(t *testing.T) {
	t.Run("ReturnsKMSDetails", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("GET", "/kms", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify the response body structure and values
		var info map[string]any
		if err := json.Unmarshal(respBody, &info); err != nil {
			t.Fatalf("Failed to parse KMS info: %v", err)
		}
		if info["kms"] != "mock-kms" {
			t.Errorf("Expected kms 'mock-kms', got %q", info["kms"])
		}
		if info["alg"] != "ES256" {
			t.Errorf("Expected alg 'ES256', got %q", info["alg"])
		}
		// Default RSA key length should be 2048
		if rsaLen, ok := info["rsa_key_len"].(float64); !ok || int(rsaLen) != 2048 {
			t.Errorf("Expected rsa_key_len 2048, got %v", info["rsa_key_len"])
		}
	})

	t.Run("IncludesPendingAlg", func(t *testing.T) {
		store := newTestStorage(t)
		// Use a custom mock that reports a pending change
		pendingMock := &mockFullKMSWithPending{}
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      pendingMock,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("GET", "/kms", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var info map[string]any
		if err := json.Unmarshal(respBody, &info); err != nil {
			t.Fatalf("Failed to parse KMS info: %v", err)
		}
		if info["pending_alg"] != "ES384" {
			t.Errorf("Expected pending_alg 'ES384', got %v", info["pending_alg"])
		}
		if info["alg_change_at"] == nil {
			t.Error("Expected alg_change_at to be set")
		}
	})
}

// mockFullKMSWithPending returns pending changes for testing the KMS info response.
type mockFullKMSWithPending struct {
	mockFullKMS
}

func (*mockFullKMSWithPending) GetPendingChanges() (*kms.PendingAlgChange, *kms.PendingDefaultChange) {
	return nil, &kms.PendingDefaultChange{
		Alg:         jwa.ES384(),
		EffectiveAt: unixtime.Unixtime{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
}

func TestPutKMSAlg(t *testing.T) {
	t.Run("NotSupportedWhenKeysNil", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      nil,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/alg", strings.NewReader(`ES512`))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidAlgorithm", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/alg", strings.NewReader(`INVALID-ALG`))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/alg", strings.NewReader(`not json`))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("Success", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/alg", strings.NewReader(`ES512`))
		req.Header.Set("Content-Type", "application/json")

		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		// Verify the response is valid KMS info
		var info map[string]any
		if err := json.Unmarshal(respBody, &info); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if info["kms"] != "mock-kms" {
			t.Errorf("Expected kms 'mock-kms', got %v", info["kms"])
		}
	})
}

func TestPutKMSRSAKeyLen(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/rsa-key-len", strings.NewReader(`4096`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		savedLen, err := storage.GetRSAKeyLen(store.KeyValue())
		if err != nil {
			t.Fatal(err)
		}
		if savedLen != 4096 {
			t.Errorf("Expected RSA key length 4096, got %d", savedLen)
		}
	})

	t.Run("NotSupportedWhenKeysNil", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      nil,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/rsa-key-len", strings.NewReader(`4096`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:       "mock-kms",
			BasicKeys: &mockBasicKMS{},
			Keys:      &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/rsa-key-len", strings.NewReader(`"not a number"`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}

// --- KMS ROTATION TESTS ---

func TestGetKMSRotation(t *testing.T) {
	t.Run("ReturnsConfig", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		initialConfig := kms.KeyRotationConfig{Enabled: true}
		if err := storage.SetKeyRotation(store.KeyValue(), initialConfig); err != nil {
			t.Fatalf("Failed to set initial rotation config: %v", err)
		}

		req := httptest.NewRequest("GET", "/kms/rotation", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		var config map[string]any
		if err := json.Unmarshal(respBody, &config); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if config["enabled"] != true {
			t.Errorf("Expected enabled true, got %v", config["enabled"])
		}
	})

	t.Run("NotSupportedWhenKeysNil", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: nil,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("GET", "/kms/rotation", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}

func TestPutKMSRotation(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		body := `{"enabled": true, "interval": 3600, "overlap": 600}`
		req := httptest.NewRequest("PUT", "/kms/rotation", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		savedConfig, err := storage.GetKeyRotation(store.KeyValue())
		if err != nil {
			t.Fatalf("Failed to get rotation config: %v", err)
		}
		if !savedConfig.Enabled {
			t.Error("Expected enabled true, got false")
		}
		if savedConfig.Interval.Duration().Seconds() != 3600 {
			t.Errorf("Expected interval 3600s, got %f", savedConfig.Interval.Duration().Seconds())
		}
		if savedConfig.Overlap.Duration().Seconds() != 600 {
			t.Errorf("Expected overlap 600s, got %f", savedConfig.Overlap.Duration().Seconds())
		}
	})

	t.Run("NotSupportedWhenKeysNil", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: nil,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		body := `{"enabled": true}`
		req := httptest.NewRequest("PUT", "/kms/rotation", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PUT", "/kms/rotation", strings.NewReader(`not json`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}

func TestPatchKMSRotation(t *testing.T) {
	t.Run("PartialUpdate", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		// Set initial config
		if err := storage.SetKeyRotation(store.KeyValue(), kms.KeyRotationConfig{Enabled: false}); err != nil {
			t.Fatalf("Failed to set initial rotation config: %v", err)
		}

		// PATCH only the 'enabled' field
		req := httptest.NewRequest("PATCH", "/kms/rotation", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		savedConfig, err := storage.GetKeyRotation(store.KeyValue())
		if err != nil {
			t.Fatal(err)
		}
		if !savedConfig.Enabled {
			t.Error("Expected enabled to be true after PATCH")
		}
	})

	t.Run("PatchInterval", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		if err := storage.SetKeyRotation(store.KeyValue(), kms.KeyRotationConfig{Enabled: true}); err != nil {
			t.Fatalf("Failed to set initial rotation config: %v", err)
		}

		// PATCH only the interval
		req := httptest.NewRequest("PATCH", "/kms/rotation", strings.NewReader(`{"interval": 7200}`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
		}

		savedConfig, err := storage.GetKeyRotation(store.KeyValue())
		if err != nil {
			t.Fatal(err)
		}
		// enabled should still be true (we didn't patch it)
		if !savedConfig.Enabled {
			t.Error("Expected enabled to remain true")
		}
		if savedConfig.Interval.Duration().Seconds() != 7200 {
			t.Errorf("Expected interval 7200s, got %f", savedConfig.Interval.Duration().Seconds())
		}
	})

	t.Run("NotSupportedWhenKeysNil", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: nil,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("PATCH", "/kms/rotation", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}

func TestPostKMSRotateAll(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: &mockFullKMS{},
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("POST", "/kms/rotate", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != fiber.StatusAccepted {
			t.Errorf("Expected status 202, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("NotSupportedWhenKeysNil", func(t *testing.T) {
		store := newTestStorage(t)
		km := KeyManagement{
			KMS:  "mock-kms",
			Keys: nil,
		}
		app := fiber.New()
		backends := model.Backends{
			KV:         store.KeyValue(),
			PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
			Transaction: func(fn model.TransactionFunc) error {
				return fn(&model.Backends{
					KV:         store.KeyValue(),
					PKStorages: func(tid string) public.PublicKeyStorage { return store.DBPublicKeyStorage(tid) },
				})
			},
		}
		registerKeys(app, km, store.KeyValue(), backends)

		req := httptest.NewRequest("POST", "/kms/rotate", http.NoBody)
		resp, respBody := doRequest(t, app, req)

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d. Body: %s", resp.StatusCode, string(respBody))
		}
	})
}
