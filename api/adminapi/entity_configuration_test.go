package adminapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jws"

	smodel "github.com/go-oidfed/lighthouse/storage/model"
	"gorm.io/datatypes"
)

// ---------------------------------------------------------------------------
// Mock: FederationEntity
// ---------------------------------------------------------------------------

type mockFederationEntity struct {
	entityID                     string
	entityConfigurationPayloadFn func() (*oidfed.EntityStatementPayload, error)
}

func (m *mockFederationEntity) EntityID() string { return m.entityID }
func (m *mockFederationEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return m.entityConfigurationPayloadFn()
}
func (m *mockFederationEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (m *mockFederationEntity) SignEntityStatement(_ oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (m *mockFederationEntity) SignEntityStatementWithHeaders(_ oidfed.EntityStatementPayload, _ jws.Headers) ([]byte, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: AdditionalClaimsStore
// ---------------------------------------------------------------------------

type mockAdditionalClaimsStore struct {
	listFn   func() ([]smodel.EntityConfigurationAdditionalClaim, error)
	setFn    func([]smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error)
	createFn func(smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error)
	getFn    func(string) (*smodel.EntityConfigurationAdditionalClaim, error)
	updateFn func(string, smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error)
	deleteFn func(string) error
}

func (m *mockAdditionalClaimsStore) List() ([]smodel.EntityConfigurationAdditionalClaim, error) {
	return m.listFn()
}
func (m *mockAdditionalClaimsStore) Set(items []smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error) {
	return m.setFn(items)
}
func (m *mockAdditionalClaimsStore) Create(item smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
	return m.createFn(item)
}
func (m *mockAdditionalClaimsStore) Get(ident string) (*smodel.EntityConfigurationAdditionalClaim, error) {
	return m.getFn(ident)
}
func (m *mockAdditionalClaimsStore) Update(ident string, item smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
	return m.updateFn(ident, item)
}
func (m *mockAdditionalClaimsStore) Delete(ident string) error {
	return m.deleteFn(ident)
}

// ---------------------------------------------------------------------------
// Mock: KeyValueStore
// ---------------------------------------------------------------------------

type mockKeyValueStore struct {
	getFn    func(scope, key string) (datatypes.JSON, error)
	getAsFn  func(scope, key string, out any) (bool, error)
	setFn    func(scope, key string, value datatypes.JSON) error
	setAnyFn func(scope, key string, v any) error
	deleteFn func(scope, key string) error
}

func (m *mockKeyValueStore) Get(scope, key string) (datatypes.JSON, error) {
	return m.getFn(scope, key)
}
func (m *mockKeyValueStore) GetAs(scope, key string, out any) (bool, error) {
	return m.getAsFn(scope, key, out)
}
func (m *mockKeyValueStore) Set(scope, key string, value datatypes.JSON) error {
	return m.setFn(scope, key, value)
}
func (m *mockKeyValueStore) SetAny(scope, key string, v any) error {
	return m.setAnyFn(scope, key, v)
}
func (m *mockKeyValueStore) Delete(scope, key string) error {
	return m.deleteFn(scope, key)
}

// ---------------------------------------------------------------------------
// Test helper: create a Fiber app with the entity configuration routes
// ---------------------------------------------------------------------------

func setupEntityConfigTestApp(
	fedEntity oidfed.FederationEntity,
	claims smodel.AdditionalClaimsStore,
	kv smodel.KeyValueStore,
) *fiber.App {
	app := fiber.New()
	registerEntityConfiguration(app, claims, kv, fedEntity)
	return app
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestIsUniqueConstraintError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"NilError", nil, false},
		{"SQLiteUNIQUEConstraintFailed", errors.New("UNIQUE constraint failed: table.column"), true},
		{"SQLiteConstraintFailed", errors.New("constraint failed"), true},
		{"MySQLDuplicateEntry", errors.New("Duplicate entry 'val' for key 'idx'"), true},
		{"MySQLError1062", errors.New("Error 1062: ..."), true},
		{"PostgresDuplicateKeyValue", errors.New("duplicate key value violates unique constraint"), true},
		{"PostgresViolatesUniqueConstraint", errors.New("violates unique constraint \"idx\""), true},
		{"UnrelatedError", errors.New("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUniqueConstraintError(tt.err)
			if got != tt.want {
				t.Errorf("isUniqueConstraintError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestGetEntityConfiguration(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		payload := &oidfed.EntityStatementPayload{
			Issuer:  "https://example.com",
			Subject: "https://example.com",
		}
		app := setupEntityConfigTestApp(
			&mockFederationEntity{
				entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
					return payload, nil
				},
			},
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		var got oidfed.EntityStatementPayload
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.Issuer != "https://example.com" {
			t.Errorf("expected issuer %q, got %q", "https://example.com", got.Issuer)
		}
		if got.Subject != "https://example.com" {
			t.Errorf("expected subject %q, got %q", "https://example.com", got.Subject)
		}
	})

	t.Run("FedEntityError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			&mockFederationEntity{
				entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
					return nil, errors.New("boom")
				},
			},
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}

func TestGetAdditionalClaims(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		claims := []smodel.EntityConfigurationAdditionalClaim{
			{ID: 1, Claim: "org_name", Value: "ACME", Crit: false},
			{ID: 2, Claim: "policy_uri", Value: "https://example.com/policy", Crit: true},
		}
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				listFn: func() ([]smodel.EntityConfigurationAdditionalClaim, error) {
					return claims, nil
				},
			},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/additional-claims", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		var got []smodel.EntityConfigurationAdditionalClaim
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 claims, got %d", len(got))
		}
		if got[0].Claim != "org_name" {
			t.Errorf("expected first claim %q, got %q", "org_name", got[0].Claim)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				listFn: func() ([]smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, errors.New("db down")
				},
			},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/additional-claims", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}

func TestPutAdditionalClaims(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				setFn: func(items []smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error) {
					return []smodel.EntityConfigurationAdditionalClaim{
						{ID: 1, Claim: items[0].Claim, Value: items[0].Value, Crit: items[0].Crit},
					}, nil
				},
			},
			&mockKeyValueStore{},
		)

		body := `[{"claim":"org_name","value":"ACME","crit":false}]`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		respBody, _ := io.ReadAll(resp.Body)
		var got []smodel.EntityConfigurationAdditionalClaim
		if err := json.Unmarshal(respBody, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(got) != 1 || got[0].Claim != "org_name" {
			t.Errorf("unexpected response: %+v", got)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims", strings.NewReader("not-json"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("UniqueConstraintError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				setFn: func(_ []smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, errors.New("UNIQUE constraint failed: claims.claim")
				},
			},
			&mockKeyValueStore{},
		)

		body := `[{"claim":"org_name","value":"ACME","crit":false}]`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				setFn: func(_ []smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, errors.New("db down")
				},
			},
			&mockKeyValueStore{},
		)

		body := `[{"claim":"org_name","value":"ACME","crit":false}]`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}
