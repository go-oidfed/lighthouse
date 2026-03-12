package storage

import (
	"os"
	"testing"
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

	if err := sqlDB.Ping(); err != nil {
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

	if err := sqlDB.Ping(); err != nil {
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

	if err := sqlDB.Ping(); err != nil {
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
