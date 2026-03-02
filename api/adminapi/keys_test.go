package adminapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

// --- MOCKS ---

// mockBasicKMS pretends to be a KMS system.
// We only implement the method we need: GetDefaultAlg.
type mockBasicKMS struct {
	// We embed the interface to satisfy the compiler if there are other methods we don't implement yet.
	// NOTE: If the test panics, it means we need to implement more methods.
	kms.BasicKeyManagementSystem
}

func (m *mockBasicKMS) GetDefaultAlg() jwa.SignatureAlgorithm {
	return jwa.ES256()
}

// mockFullKMS embeds the full KeyManagementSystem interface.
// We use this when we need to test methods like ChangeKeyRotationConfig.
type mockFullKMS struct {
    kms.KeyManagementSystem // This lets us satisfy the interface without implementing every single method
}

// Mock the specific method we are testing
func (m *mockFullKMS) ChangeKeyRotationConfig(cfg kms.KeyRotationConfig) error {
    return nil // Pretend the KMS successfully updated its schedule
}

// --- TESTS ---

func TestGetKMSInfo(t *testing.T) {
	// 1. ARRANGE: Setup the world
	// ---------------------------

	// A. Create a temporary database
	tempDir, err := os.MkdirTemp("", "lighthouse-test-kms-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir) // Clean up when done

	// B. Connect to the DB (using SQLite)
	config := storage.Config{
		Driver:  storage.DriverSQLite,
		DataDir: tempDir,
	}
	store, err := storage.NewStorage(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// C. Setup the KeyManagement struct with our Mock
	km := KeyManagement{
		KMS:       "mock-kms",
		BasicKeys: &mockBasicKMS{},
		// We leave 'Keys', 'APIManagedPKs', etc. nil because GET /kms doesn't use them (hopefully)
	}

	// D. Setup Fiber App and Register Routes
	app := fiber.New()
	// We pass the KeyValueStorage from our real DB
	registerKeys(app, km, store.KeyValue())

	// 2. ACT: Perform the request
	// ---------------------------
	req := httptest.NewRequest("GET", "/kms", nil)
	resp, err := app.Test(req, -1) // -1 means no timeout
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// 3. ASSERT: Verify the result
	// ----------------------------
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Optional: You could read the body here and verify "alg" is "ES256"
}

// ... (keep your existing TestGetKMSInfo function) ...

func TestPutKMSAlg_NotSupported(t *testing.T) {
	// 1. ARRANGE
	tempDir, err := os.MkdirTemp("", "lighthouse-test-kms-alg-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	config := storage.Config{
		Driver:  storage.DriverSQLite,
		DataDir: tempDir,
	}
	store, err := storage.NewStorage(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	km := KeyManagement{
		KMS:       "mock-kms",
		BasicKeys: &mockBasicKMS{},
		Keys:      nil, // Explicitly nil to trigger the error
	}

	app := fiber.New()
	registerKeys(app, km, store.KeyValue())

	// 2. ACT
	req := httptest.NewRequest("PUT", "/kms/alg", strings.NewReader(`"ES512"`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// 3. ASSERT
	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}


func TestUpdateKMSRotation(t *testing.T) {
    // 1. ARRANGE
    tempDir, err := os.MkdirTemp("", "lighthouse-test-rotation-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(tempDir)

    store, err := storage.NewStorage(storage.Config{
        Driver:  storage.DriverSQLite,
        DataDir: tempDir,
    })
    if err != nil {
        t.Fatalf("Failed to create storage: %v", err)
    }

    // Use our new "Full" mock here
    km := KeyManagement{
        KMS:  "mock-kms",
        Keys: &mockFullKMS{}, 
    }

    app := fiber.New()
    registerKeys(app, km, store.KeyValue())

    // 2. ACT
    // We send a JSON body to update the rotation settings
    // JSON: enabled=true, interval=3600s (1 hour), overlap=600s (10 mins)
    body := `{"enabled": true, "interval": 3600, "overlap": 600}`
    req := httptest.NewRequest("PUT", "/kms/rotation", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := app.Test(req, -1)
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // 3. ASSERT (HTTP Layer)
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }

    // 4. ASSERT (Database Layer)
    // We verify that the data actually persisted to the storage
    savedConfig, err := storage.GetKeyRotation(store.KeyValue())
    if err != nil {
        t.Fatalf("Failed to get rotation config from storage: %v", err)
    }

    if savedConfig.Enabled != true {
        t.Errorf("Expected enabled true, got %v", savedConfig.Enabled)
    }
    // Note: interval is stored as time.Duration (nanoseconds) usually, 
    // so 3600 seconds = 3600 * 1,000,000,000
    if savedConfig.Interval.Duration().Seconds() != 3600 {
        t.Errorf("Expected interval 3600s, got %f", savedConfig.Interval.Duration().Seconds())
    }
	if savedConfig.Overlap.Duration().Seconds() != 600 {
		t.Errorf("Expected overlap 600s, got %f", savedConfig.Overlap.Duration().Seconds())
	}
}

func TestPostPublicKey(t *testing.T) {
	// 1. ARRANGE
	tempDir, err := os.MkdirTemp("", "lighthouse-test-post-keys-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.NewStorage(storage.Config{
		Driver:  storage.DriverSQLite,
		DataDir: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set up APIManagedPKs using the real database!
	km := KeyManagement{
		APIManagedPKs: store.DBPublicKeyStorage("api-managed"),
	}

	if err := km.APIManagedPKs.Load(); err != nil {
		t.Fatalf("Failed to create public key table: %v", err)
	}

	app := fiber.New()
	registerKeys(app, km, store.KeyValue())

	// 2. ACT
	// We send a JSON body containing a standard RSA Public Key in JWK format
	body := `{
		"key": {
			"kty": "RSA",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB",
			"kid": "my-test-key-1"
		}
	}`
	req := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}

	// 3. ASSERT
	
	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 201, got %d. Response body: %s", resp.StatusCode, string(respBody))
	}

	// Using the storage method `Get`, we ask the database for "my-test-key-1"
	savedKey, err := km.APIManagedPKs.Get("my-test-key-1")
	if err != nil {
		t.Fatalf("Failed to fetch key from database: %v", err)
	}
	
	if savedKey == nil {
		t.Fatal("Expected to find saved key in database, got nil")
	}
	
	if savedKey.KID != "my-test-key-1" {
		t.Errorf("Expected KID 'my-test-key-1', got '%s'", savedKey.KID)
	}
}


func TestGetPublicKeys(t *testing.T) {
	// 1. ARRANGE
	tempDir, err := os.MkdirTemp("", "lighthouse-test-get-keys-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.NewStorage(storage.Config{
		Driver:  storage.DriverSQLite,
		DataDir: tempDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	km := KeyManagement{
		APIManagedPKs: store.DBPublicKeyStorage("api-managed"),
	}
	if err := km.APIManagedPKs.Load(); err != nil {
		t.Fatalf("Failed to create public key table: %v", err)
	}

	app := fiber.New()
	registerKeys(app, km, store.KeyValue())

	// INJECT DATA: Let's use our previous test's logic to put a key in the DB directly
	body := `{
		"kid": "my-get-test-key", 
		"key": {
			"kty": "RSA",
			"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"e": "AQAB",
			"kid": "my-get-test-key"
		}
	}`
	// We use the app.Test to POST the data first (we know this works because of our last test!)
	setupReq := httptest.NewRequest("POST", "/entity-configuration/keys", strings.NewReader(body))
	setupReq.Header.Set("Content-Type", "application/json")
	app.Test(setupReq, -1) 

	// 2. ACT
	// TODO Task A: Create a GET request to "/entity-configuration/keys/" (notice the trailing slash!)
	getReq := httptest.NewRequest("GET", "/entity-configuration/keys/", nil)
	// TODO Task B: Execute the request using app.Test
	resp, err := app.Test(getReq, -1)
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ := io.ReadAll(resp.Body)

	// 3. ASSERT
	// TODO Task C: Check that resp.StatusCode is exactly 200
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d. Response body: %s", resp.StatusCode, string(respBody))
	}
	
	// Task D: Let's read the JSON response and see if our key is in it!	
	// The API returns a list of keys, so we parse it into a slice of maps
	var returnedKeys []map[string]any
	if err := json.Unmarshal(respBody, &returnedKeys); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if len(returnedKeys) != 1 {
		t.Errorf("Expected 1 key in the response, but got %d", len(returnedKeys))
	} else if returnedKeys[0]["kid"] != "my-get-test-key" {
		t.Errorf("Expected kid 'my-get-test-key', got '%v'", returnedKeys[0]["kid"])
	}
	
}