package storage

import (
	"os"
	"testing"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TestSQLiteConnection tests connecting to a SQLite database
func TestSQLiteConnection(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create a temporary directory for the SQLite database
	tempDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a SQLite configuration
	config := Config{
		Driver:  DriverSQLite,
		DataDir: tempDir,
	}

	// Connect to the database
	db, err := Connect(config)
	if err != nil {
		t.Fatalf("Failed to connect to SQLite database: %v", err)
	}

	// Check if the connection is valid
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get SQL DB: %v", err)
	}

	if err = sqlDB.Ping(); err != nil {
		t.Fatalf("Failed to ping SQLite database: %v", err)
	}
}

// TestMySQLConnection tests connecting to a MySQL database
func TestMySQLConnection(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	// Skip if MySQL DSN is not provided
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("Skipping MySQL test. Set MYSQL_DSN environment variable")
	}

	// Create a MySQL configuration
	config := Config{
		Driver: DriverMySQL,
		DSN:    dsn,
	}

	// Connect to the database
	db, err := Connect(config)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL database: %v", err)
	}

	// Check if the connection is valid
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get SQL DB: %v", err)
	}

	if err = sqlDB.Ping(); err != nil {
		t.Fatalf("Failed to ping MySQL database: %v", err)
	}
}

// TestPostgresConnection tests connecting to a PostgreSQL database
func TestPostgresConnection(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	// Skip if PostgreSQL DSN is not provided
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL test. Set POSTGRES_DSN environment variable")
	}

	// Create a PostgreSQL configuration
	config := Config{
		Driver: DriverPostgres,
		DSN:    dsn,
	}

	// Connect to the database
	db, err := Connect(config)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL database: %v", err)
	}

	// Check if the connection is valid
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get SQL DB: %v", err)
	}

	if err = sqlDB.Ping(); err != nil {
		t.Fatalf("Failed to ping PostgreSQL database: %v", err)
	}
}

// TestStorageCreation tests creating a storage with different database types
func TestStorageCreation(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create a temporary directory for the SQLite database
	tempDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test SQLite storage creation
	sqliteConfig := Config{
		Driver:  DriverSQLite,
		DataDir: tempDir,
	}

	sqliteStorage, err := NewStorage(sqliteConfig)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}

	// Test basic operations
	subStorage := sqliteStorage.SubordinateStorage()
	if subStorage == nil {
		t.Fatal("SubordinateStorage() returned nil")
	}

	trustStorage := sqliteStorage.TrustMarkedEntitiesStorage()
	if trustStorage == nil {
		t.Fatal("TrustMarkedEntitiesStorage() returned nil")
	}
}

// TestAuthorityHintJWKSSyncRoundTrip verifies the jwks_sync JSON column
// persists and round-trips through the AuthorityHintsStorage.
func TestAuthorityHintJWKSSyncRoundTrip(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	tempDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := NewStorage(Config{Driver: DriverSQLite, DataDir: tempDir})
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}
	store := st.AuthorityHintsStorage()

	created, err := store.Create(model.AddAuthorityHint{
		EntityID: "https://superior.example",
		JWKSSync: &model.JWKSSyncConf{
			Mode:        model.JWKSSyncPush,
			JWTLifetime: 600,
			Timeout:     20,
			Headers:     map[string]string{"X-Custom": "yes"},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err := store.Get("https://superior.example")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.JWKSSync == nil || got.JWKSSync.Mode != model.JWKSSyncPush {
		t.Fatalf("expected push jwks_sync, got %+v", got.JWKSSync)
	}
	if got.JWKSSync.Timeout != 20 {
		t.Fatalf("timeout = %d, want 20", got.JWKSSync.Timeout)
	}
	if got.JWKSSync.JWTLifetime != 600 {
		t.Fatalf("jwt_lifetime = %d, want 600", got.JWKSSync.JWTLifetime)
	}
	if got.JWKSSync.Headers["X-Custom"] != "yes" {
		t.Fatalf("headers = %v", got.JWKSSync.Headers)
	}

	// Update to trigger mode and confirm it replaces the stored config.
	if _, err := store.Update(created.EntityID, model.AddAuthorityHint{
		EntityID: created.EntityID,
		JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncTrigger},
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	got2, err := store.Get("https://superior.example")
	if err != nil {
		t.Fatalf("get after update failed: %v", err)
	}
	if got2.JWKSSync == nil || got2.JWKSSync.Mode != model.JWKSSyncTrigger {
		t.Fatalf("expected trigger after update, got %+v", got2.JWKSSync)
	}

	// Update keeps the identity (entity_id is required and always written;
	// the handler enforces presence, storage trusts the caller).
	if _, err := store.Update(created.EntityID, model.AddAuthorityHint{
		EntityID: created.EntityID,
		JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncNone},
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	got3, err := store.Get("https://superior.example")
	if err != nil {
		t.Fatalf("get after update failed: %v", err)
	}
	if got3.EntityID != "https://superior.example" {
		t.Fatalf("entity_id must be preserved when resupplied, got %q", got3.EntityID)
	}
	if got3.JWKSSync == nil || got3.JWKSSync.Mode != model.JWKSSyncNone {
		t.Fatalf("expected jwks_sync none after update, got %+v", got3.JWKSSync)
	}

	// Renaming via update moves the row to the new unique entity_id.
	if _, err := store.Update(created.EntityID, model.AddAuthorityHint{
		EntityID: "https://superior-renamed.example",
	}); err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if _, err := store.Get("https://superior.example"); err == nil {
		t.Fatalf("old entity_id must no longer resolve after rename")
	}
	renamed, err := store.Get("https://superior-renamed.example")
	if err != nil {
		t.Fatalf("get after rename failed: %v", err)
	}
	if renamed.JWKSSync != nil {
		t.Fatalf("replacement PUT without jwks_sync must reset syncing, got %+v", renamed.JWKSSync)
	}

	// Renaming onto another hint's entity_id must surface a conflict (409 at
	// the handler level).
	if _, err := store.Create(model.AddAuthorityHint{
		EntityID: "https://taken.example",
	}); err != nil {
		t.Fatalf("create second hint failed: %v", err)
	}
	if _, err := store.Update("https://superior-renamed.example", model.AddAuthorityHint{
		EntityID: "https://taken.example",
	}); err != nil {
		if _, ok := err.(model.AlreadyExistsError); !ok {
			t.Fatalf("expected AlreadyExistsError on colliding rename, got %v", err)
		}
	} else {
		t.Fatalf("expected colliding rename to be rejected")
	}
}

// TestAuthorityHintPatch verifies the partial-update (PATCH) storage path:
// only provided fields change, null clears, and invalid input is rejected.
func TestAuthorityHintPatch(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	tempDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := NewStorage(Config{Driver: DriverSQLite, DataDir: tempDir})
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}
	store := st.AuthorityHintsStorage()

	if _, err := store.Create(model.AddAuthorityHint{
		EntityID:    "https://patch.example",
		Description: "original",
		JWKSSync: &model.JWKSSyncConf{
			Mode: model.JWKSSyncPush,
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Changing only jwks_sync keeps description and identity.
	patched, err := store.Patch("https://patch.example", map[string]any{
		"jwks_sync": map[string]any{"mode": "trigger"},
	})
	if err != nil {
		t.Fatalf("patch jwks_sync failed: %v", err)
	}
	if patched.Description != "original" {
		t.Fatalf("description must be untouched by jwks_sync patch, got %q", patched.Description)
	}
	if patched.JWKSSync == nil || patched.JWKSSync.Mode != model.JWKSSyncTrigger {
		t.Fatalf("expected trigger after patch, got %+v", patched.JWKSSync)
	}

	// Changing only description keeps jwks_sync and identity.
	patched, err = store.Patch("https://patch.example", map[string]any{"description": "updated"})
	if err != nil {
		t.Fatalf("patch description failed: %v", err)
	}
	if patched.Description != "updated" {
		t.Fatalf("expected description updated, got %q", patched.Description)
	}
	if patched.JWKSSync == nil || patched.JWKSSync.Mode != model.JWKSSyncTrigger {
		t.Fatalf("jwks_sync must be untouched by description patch, got %+v", patched.JWKSSync)
	}

	// null jwks_sync clears syncing.
	patched, err = store.Patch("https://patch.example", map[string]any{"jwks_sync": nil})
	if err != nil {
		t.Fatalf("patch clear jwks_sync failed: %v", err)
	}
	if patched.JWKSSync != nil {
		t.Fatalf("expected jwks_sync cleared, got %+v", patched.JWKSSync)
	}

	// Empty patch is a no-op.
	patched, err = store.Patch("https://patch.example", map[string]any{})
	if err != nil {
		t.Fatalf("patch no-op failed: %v", err)
	}
	if patched.Description != "updated" {
		t.Fatalf("expected description untouched, got %q", patched.Description)
	}

	// Invalid input is rejected.
	if _, err := store.Patch("https://patch.example", map[string]any{"jwks_sync": map[string]any{"mode": "bogus"}}); err == nil {
		t.Fatalf("expected error for unknown jwks_sync mode")
	}
	if _, err := store.Patch("https://patch.example", map[string]any{"description": 42}); err == nil {
		t.Fatalf("expected error for non-string description")
	}
	if _, err := store.Patch("missing", map[string]any{"description": "x"}); err == nil {
		t.Fatalf("expected not-found error for unknown hint")
	}
}
