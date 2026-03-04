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

func TestPostAdditionalClaim(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				createFn: func(item smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return &smodel.EntityConfigurationAdditionalClaim{
						ID: 1, Claim: item.Claim, Value: item.Value, Crit: item.Crit,
					}, nil
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"ACME","crit":false}`
		req, _ := http.NewRequest(http.MethodPost, "/entity-configuration/additional-claims", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", resp.StatusCode)
		}
		respBody, _ := io.ReadAll(resp.Body)
		var got smodel.EntityConfigurationAdditionalClaim
		if err := json.Unmarshal(respBody, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.Claim != "org_name" {
			t.Errorf("expected claim %q, got %q", "org_name", got.Claim)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPost, "/entity-configuration/additional-claims", strings.NewReader("bad"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				createFn: func(_ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, smodel.AlreadyExistsError("claim already exists")
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"ACME","crit":false}`
		req, _ := http.NewRequest(http.MethodPost, "/entity-configuration/additional-claims", strings.NewReader(body))
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
				createFn: func(_ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, errors.New("db down")
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"ACME","crit":false}`
		req, _ := http.NewRequest(http.MethodPost, "/entity-configuration/additional-claims", strings.NewReader(body))
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

func TestGetAdditionalClaimByID(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				getFn: func(ident string) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return &smodel.EntityConfigurationAdditionalClaim{
						ID: 42, Claim: "org_name", Value: "ACME",
					}, nil
				},
			},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/additional-claims/42", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		respBody, _ := io.ReadAll(resp.Body)
		var got smodel.EntityConfigurationAdditionalClaim
		if err := json.Unmarshal(respBody, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.ID != 42 || got.Claim != "org_name" {
			t.Errorf("unexpected response: %+v", got)
		}
	})

	t.Run("InvalidID", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/additional-claims/abc", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				getFn: func(_ string) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, errors.New("not found")
				},
			},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/additional-claims/99", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

func TestPutAdditionalClaimByID(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				updateFn: func(ident string, item smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return &smodel.EntityConfigurationAdditionalClaim{
						ID: 5, Claim: item.Claim, Value: item.Value, Crit: item.Crit,
					}, nil
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"UpdatedACME","crit":true}`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims/5", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		respBody, _ := io.ReadAll(resp.Body)
		var got smodel.EntityConfigurationAdditionalClaim
		if err := json.Unmarshal(respBody, &got); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if got.Claim != "org_name" || got.Crit != true {
			t.Errorf("unexpected response: %+v", got)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims/5", strings.NewReader("bad"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				updateFn: func(_ string, _ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, smodel.NotFoundError("not found")
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"X","crit":false}`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims/999", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				updateFn: func(_ string, _ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, smodel.AlreadyExistsError("duplicate claim")
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"X","crit":false}`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims/5", strings.NewReader(body))
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
				updateFn: func(_ string, _ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
					return nil, errors.New("db down")
				},
			},
			&mockKeyValueStore{},
		)

		body := `{"claim":"org_name","value":"X","crit":false}`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/additional-claims/5", strings.NewReader(body))
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

func TestDeleteAdditionalClaimByID(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				deleteFn: func(ident string) error {
					return nil
				},
			},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/additional-claims/42", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidID", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/additional-claims/abc", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{
				deleteFn: func(_ string) error {
					return errors.New("not found error from db")
				},
			},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/additional-claims/99", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

func TestGetLifetime(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getAsFn: func(scope, key string, out any) (bool, error) {
					if scope == smodel.KeyValueScopeEntityConfiguration && key == smodel.KeyValueKeyLifetime {
						ptr := out.(*int)
						*ptr = 3600
						return true, nil
					}
					return false, nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/lifetime", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "3600" {
			t.Errorf("expected 3600, got %q", string(body))
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getAsFn: func(scope, key string, out any) (bool, error) {
					return false, nil // Not found
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/lifetime", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "86400" {
			t.Errorf("expected 86400, got %q", string(body))
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getAsFn: func(scope, key string, out any) (bool, error) {
					return false, errors.New("kv db down")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/lifetime", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}

func TestPutLifetime(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				setAnyFn: func(scope, key string, v any) error {
					if scope == smodel.KeyValueScopeEntityConfiguration && key == smodel.KeyValueKeyLifetime {
						val := v.(int)
						if val != 7200 {
							t.Errorf("expected 7200 to be saved, got %d", val)
						}
					}
					return nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/lifetime", strings.NewReader("7200"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "7200" {
			t.Errorf("expected 7200 in response, got %q", string(body))
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/lifetime", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/lifetime", strings.NewReader(`"not-int"`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400 for string body, got %d", resp.StatusCode)
		}
	})

	t.Run("NegativeValue", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/lifetime", strings.NewReader("-3600"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400 for negative, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				setAnyFn: func(scope, key string, v any) error {
					return errors.New("db error")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/lifetime", strings.NewReader("3600"))
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

func TestGetMetadata(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					if scope == smodel.KeyValueScopeEntityConfiguration && key == smodel.KeyValueKeyMetadata {
						return datatypes.JSON(`{"openid_provider":{"issuer":"https://example.com"}}`), nil
					}
					return nil, nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		var got oidfed.Metadata
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("failed to parse metadata: %v", err)
		}
		if got.OpenIDProvider == nil || got.OpenIDProvider.Issuer != "https://example.com" {
			t.Errorf("expected op.issuer=https://example.com, got %+v", got.OpenIDProvider)
		}
	})

	t.Run("NoMetadataStored", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "{}" {
			t.Errorf("expected {}, got %q", string(body))
		}
	})

	t.Run("CorruptStoredMetadata", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{bad-json`), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, errors.New("db error")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}

func TestPutMetadata(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				setFn: func(scope, key string, value datatypes.JSON) error {
					if scope == smodel.KeyValueScopeEntityConfiguration && key == smodel.KeyValueKeyMetadata {
						if !strings.Contains(string(value), "https://example.com") {
							t.Errorf("expected string to contain issuer, got %s", value)
						}
					}
					return nil
				},
			},
		)

		body := `{"openid_provider":{"issuer":"https://example.com"}}`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata", strings.NewReader(`"not-an-object"`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				setFn: func(scope, key string, value datatypes.JSON) error {
					return errors.New("db down")
				},
			},
		)

		body := `{"openid_provider":{"issuer":"https://example.com"}}`
		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata", strings.NewReader(body))
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

func TestGetMetadataClaim(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	metaJSON := `{"openid_provider":{"issuer":"https://example.com","some_claim":123}}`

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(metaJSON), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata/openid_provider/issuer", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != `"https://example.com"` {
			t.Errorf("expected string JSON, got %s", body)
		}
	})

	t.Run("NoMetadataStored", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata/openid_provider/issuer", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("EntityTypeNotFound", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(metaJSON), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata/oauth_client/issuer", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("ClaimNotFound", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(metaJSON), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata/openid_provider/missing", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("CorruptStoredMetadata", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{bad-json`), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata/openid_provider/issuer", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, errors.New("db error")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodGet, "/entity-configuration/metadata/openid_provider/issuer", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}

func TestPutMetadataClaim(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success_ExistingMeta", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{"openid_provider":{"old":123}}`), nil
				},
				setFn: func(scope, key string, value datatypes.JSON) error {
					s := string(value)
					if !strings.Contains(s, `"old":123`) || !strings.Contains(s, `"new":456`) {
						t.Errorf("expected merged json, got %s", s)
					}
					return nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Success_NoExistingMeta", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, nil
				},
				setFn: func(scope, key string, value datatypes.JSON) error {
					if !strings.Contains(string(value), `"new":456`) {
						t.Errorf("expected json, got %s", value)
					}
					return nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata/openid_provider/new", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status 400 for empty body, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreGetError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, errors.New("db down during get")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreSetError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, nil
				},
				setFn: func(scope, key string, value datatypes.JSON) error {
					return errors.New("db down during set")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("CorruptStoredMetadata", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{bad-json`), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodPut, "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`))
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

func TestDeleteMetadataClaim(t *testing.T) {
	stubFedEntity := &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}

	t.Run("Success", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{"openid_provider":{"target":123,"other":456}}`), nil
				},
				setFn: func(scope, key string, value datatypes.JSON) error {
					s := string(value)
					if strings.Contains(s, `"target"`) {
						t.Errorf("claim was not deleted: %s", s)
					}
					if !strings.Contains(s, `"other"`) {
						t.Errorf("wrong claim deleted: %s", s)
					}
					return nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/metadata/openid_provider/target", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", resp.StatusCode)
		}
	})

	t.Run("NoMetadataStored", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/metadata/openid_provider/target", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", resp.StatusCode)
		}
	})

	t.Run("EntityTypeNotInMeta", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{"other_type":{"target":123}}`), nil
				},
				setFn: func(scope, key string, value datatypes.JSON) error {
					t.Errorf("set should not be called")
					return nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/metadata/openid_provider/target", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", resp.StatusCode)
		}
	})

	t.Run("CorruptStoredMetadata", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{bad-json`), nil
				},
			},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/metadata/openid_provider/target", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreGetError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return nil, errors.New("db error")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/metadata/openid_provider/target", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("StoreSetError", func(t *testing.T) {
		app := setupEntityConfigTestApp(
			stubFedEntity,
			&mockAdditionalClaimsStore{},
			&mockKeyValueStore{
				getFn: func(scope, key string) (datatypes.JSON, error) {
					return datatypes.JSON(`{"openid_provider":{"target":123}}`), nil
				},
				setFn: func(scope, key string, value datatypes.JSON) error {
					return errors.New("db error")
				},
			},
		)

		req, _ := http.NewRequest(http.MethodDelete, "/entity-configuration/metadata/openid_provider/target", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}
