package adminapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/oidfedconst"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jws"
	"gorm.io/datatypes"

	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

// --- MOCKS ---

type mockFederationEntity struct {
	entityID                     string
	entityConfigurationPayloadFn func() (*oidfed.EntityStatementPayload, error)
}

func (m *mockFederationEntity) EntityID() string { return m.entityID }
func (m *mockFederationEntity) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return m.entityConfigurationPayloadFn()
}
func (*mockFederationEntity) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (*mockFederationEntity) SignEntityStatement(_ oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}

func (*mockFederationEntity) SignEntityStatementWithHeaders(_ oidfed.EntityStatementPayload, _ jws.Headers) (
	[]byte, error,
) {
	return nil, nil
}

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

func (m *mockAdditionalClaimsStore) Set(items []smodel.AddAdditionalClaim) (
	[]smodel.EntityConfigurationAdditionalClaim, error,
) {
	return m.setFn(items)
}

func (m *mockAdditionalClaimsStore) Create(item smodel.AddAdditionalClaim) (
	*smodel.EntityConfigurationAdditionalClaim, error,
) {
	return m.createFn(item)
}

func (m *mockAdditionalClaimsStore) Get(ident string) (*smodel.EntityConfigurationAdditionalClaim, error) {
	return m.getFn(ident)
}

func (m *mockAdditionalClaimsStore) Update(
	ident string, item smodel.AddAdditionalClaim,
) (*smodel.EntityConfigurationAdditionalClaim, error) {
	return m.updateFn(ident, item)
}

func (m *mockAdditionalClaimsStore) Delete(ident string) error {
	return m.deleteFn(ident)
}

type mockKeyValueStore struct {
	getFn    func(scope, key string) (datatypes.JSON, error)
	getAsFn  func(scope, key string, out any) (bool, error)
	setFn    func(scope, key string, value datatypes.JSON) error
	setAnyFn func(scope, key string, v any) error
	deleteFn func(scope, key string) error
}

func (m *mockKeyValueStore) Get(scope, key string) (datatypes.JSON, error) {
	if m.getFn == nil {
		return nil, nil
	}
	return m.getFn(scope, key)
}

func (m *mockKeyValueStore) GetAs(scope, key string, out any) (bool, error) {
	if m.getAsFn == nil {
		return false, nil
	}
	return m.getAsFn(scope, key, out)
}

func (m *mockKeyValueStore) Set(scope, key string, value datatypes.JSON) error {
	if m.setFn == nil {
		return nil
	}
	return m.setFn(scope, key, value)
}

func (m *mockKeyValueStore) SetAny(scope, key string, v any) error {
	if m.setAnyFn == nil {
		return nil
	}
	return m.setAnyFn(scope, key, v)
}

func (m *mockKeyValueStore) Delete(scope, key string) error {
	if m.deleteFn == nil {
		return nil
	}
	return m.deleteFn(scope, key)
}

// --- TEST HELPERS ---

func setupEntityConfigTestApp(
	fedEntity oidfed.FederationEntity,
	claims smodel.AdditionalClaimsStore,
	kv smodel.KeyValueStore,
) *fiber.App {
	app := fiber.New()
	registerEntityConfiguration(app, claims, kv, fedEntity)
	return app
}

// newStubFedEntity creates a mock FederationEntity that returns an empty payload.
func newStubFedEntity() *mockFederationEntity {
	return &mockFederationEntity{
		entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
			return &oidfed.EntityStatementPayload{}, nil
		},
	}
}

func setupRealEntityConfigClaimsApp(t *testing.T) (*fiber.App, smodel.AdditionalClaimsStore) {
	t.Helper()
	store := newTestStorage(t)
	claimsStore := store.AdditionalClaimsStorage()
	app := setupEntityConfigTestApp(newStubFedEntity(), claimsStore, store.KeyValue())
	return app, claimsStore
}

func requireEntityConfigClaimRecord(
	t *testing.T, store smodel.AdditionalClaimsStore, ident string,
) *smodel.EntityConfigurationAdditionalClaim {
	t.Helper()
	claim, err := store.Get(ident)
	if err != nil {
		t.Fatalf("failed to get entity configuration additional claim %q: %v", ident, err)
	}
	if claim == nil {
		t.Fatalf("expected entity configuration additional claim %q to exist", ident)
	}
	return claim
}

// --- TESTS ---

func TestIsUniqueConstraintError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"NilError",
			nil,
			false,
		},
		{
			"SQLiteUNIQUEConstraintFailed",
			errors.New("UNIQUE constraint failed: table.column"),
			true,
		},
		{
			"SQLiteConstraintFailed",
			errors.New("constraint failed"),
			true,
		},
		{
			"MySQLDuplicateEntry",
			errors.New("Duplicate entry 'val' for key 'idx'"),
			true,
		},
		{
			"MySQLError1062",
			errors.New("Error 1062: ..."),
			true,
		},
		{
			"PostgresDuplicateKeyValue",
			errors.New("duplicate key value violates unique constraint"),
			true,
		},
		{
			"PostgresViolatesUniqueConstraint",
			errors.New("violates unique constraint \"idx\""),
			true,
		},
		{
			"UnrelatedError",
			errors.New("connection refused"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				got := isUniqueConstraintError(tt.err)
				if got != tt.want {
					t.Errorf("isUniqueConstraintError(%v) = %v, want %v", tt.err, got, tt.want)
				}
			},
		)
	}
}

func TestGetEntityConfiguration(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
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

			req := httptest.NewRequest("GET", "/entity-configuration/", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got oidfed.EntityStatementPayload
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if got.Issuer != "https://example.com" {
				t.Errorf("expected issuer %q, got %q", "https://example.com", got.Issuer)
			}
			if got.Subject != "https://example.com" {
				t.Errorf("expected subject %q, got %q", "https://example.com", got.Subject)
			}
		},
	)

	t.Run(
		"FedEntityError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				&mockFederationEntity{
					entityConfigurationPayloadFn: func() (*oidfed.EntityStatementPayload, error) {
						return nil, errors.New("boom")
					},
				},
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestGetAdditionalClaims(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			claims := []smodel.EntityConfigurationAdditionalClaim{
				{
					ID:    1,
					Claim: "org_name",
					Value: "ACME",
					Crit:  false,
				},
				{
					ID:    2,
					Claim: "policy_uri",
					Value: "https://example.com/policy",
					Crit:  true,
				},
			}
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					listFn: func() ([]smodel.EntityConfigurationAdditionalClaim, error) {
						return claims, nil
					},
				},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/additional-claims", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got []smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("expected 2 claims, got %d", len(got))
			}
			if got[0].Claim != "org_name" {
				t.Errorf("expected first claim %q, got %q", "org_name", got[0].Claim)
			}
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					listFn: func() ([]smodel.EntityConfigurationAdditionalClaim, error) {
						return nil, errors.New("db down")
					},
				},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/additional-claims", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestPutAdditionalClaims(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					setFn: func(items []smodel.AddAdditionalClaim) (
						[]smodel.EntityConfigurationAdditionalClaim, error,
					) {
						return []smodel.EntityConfigurationAdditionalClaim{
							{
								ID:    1,
								Claim: items[0].Claim,
								Value: items[0].Value,
								Crit:  items[0].Crit,
							},
						}, nil
					},
				},
				&mockKeyValueStore{},
			)

			body := `[{"claim":"org_name","value":"ACME","crit":false}]`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got []smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if len(got) != 1 || got[0].Claim != "org_name" {
				t.Errorf("unexpected response: %+v", got)
			}
		},
	)

	t.Run(
		"SuccessWithObjectValues_RealStore", func(t *testing.T) {
			t.Parallel()
			app, store := setupRealEntityConfigClaimsApp(t)

			body := `[
			{"claim":"org_name","value":{"display":"ACME","labels":{"tier":"gold"}},"crit":false},
			{"claim":"policy_flags","value":{"beta":true},"crit":true}
		]`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)

			requireStatus(t, resp, respBody, http.StatusOK)

			var got []smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal PUT additional claims response: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("expected 2 claims in response, got %d", len(got))
			}

			storedOrg := requireEntityConfigClaimRecord(t, store, "org_name")
			storedOrgValue := requireJSONMap(t, storedOrg.Value, "stored org_name value")
			storedLabels := requireJSONMap(t, storedOrgValue["labels"], "stored org_name labels")
			if storedOrgValue["display"] != "ACME" || storedLabels["tier"] != "gold" {
				t.Fatalf("expected org_name object value to persist, got %+v", storedOrg.Value)
			}

			storedFlags := requireEntityConfigClaimRecord(t, store, "policy_flags")
			storedFlagsValue := requireJSONMap(t, storedFlags.Value, "stored policy_flags value")
			if storedFlagsValue["beta"] != true || !storedFlags.Crit {
				t.Fatalf("expected policy_flags object value and crit to persist, got %+v", storedFlags)
			}
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims", strings.NewReader("not-json"))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"UniqueConstraintError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					setFn: func(_ []smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error) {
						return nil, errors.New("UNIQUE constraint failed: claims.claim")
					},
				},
				&mockKeyValueStore{},
			)

			body := `[{"claim":"org_name","value":"ACME","crit":false}]`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 409)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					setFn: func(_ []smodel.AddAdditionalClaim) ([]smodel.EntityConfigurationAdditionalClaim, error) {
						return nil, errors.New("db down")
					},
				},
				&mockKeyValueStore{},
			)

			body := `[{"claim":"org_name","value":"ACME","crit":false}]`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestPostAdditionalClaim(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					createFn: func(item smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
						return &smodel.EntityConfigurationAdditionalClaim{
							ID:    1,
							Claim: item.Claim,
							Value: item.Value,
							Crit:  item.Crit,
						}, nil
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"ACME","crit":false}`
			req := httptest.NewRequest("POST", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 201)
			var got smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if got.Claim != "org_name" {
				t.Errorf("expected claim %q, got %q", "org_name", got.Claim)
			}
		},
	)

	t.Run(
		"SuccessWithObjectValue_RealStore", func(t *testing.T) {
			t.Parallel()
			app, store := setupRealEntityConfigClaimsApp(t)

			body := `{"claim":"org_profile","value":{"sector":"finance","flags":{"beta":true}},"crit":true}`
			req := httptest.NewRequest("POST", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)

			requireStatus(t, resp, respBody, http.StatusCreated)

			var created smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &created); err != nil {
				t.Fatalf("failed to unmarshal POST additional claim response: %v", err)
			}
			createdValue := requireJSONMap(t, created.Value, "created org_profile value")
			createdFlags := requireJSONMap(t, createdValue["flags"], "created org_profile flags")
			if createdValue["sector"] != "finance" || createdFlags["beta"] != true || !created.Crit {
				t.Fatalf("expected object value in create response, got %+v", created)
			}

			stored := requireEntityConfigClaimRecord(t, store, "org_profile")
			storedValue := requireJSONMap(t, stored.Value, "stored org_profile value")
			storedFlags := requireJSONMap(t, storedValue["flags"], "stored org_profile flags")
			if storedValue["sector"] != "finance" || storedFlags["beta"] != true || !stored.Crit {
				t.Fatalf("expected object value to persist, got %+v", stored)
			}
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("POST", "/entity-configuration/additional-claims", strings.NewReader("bad"))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"AlreadyExists", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					createFn: func(_ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
						return nil, smodel.AlreadyExistsError("claim already exists")
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"ACME","crit":false}`
			req := httptest.NewRequest("POST", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 409)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					createFn: func(_ smodel.AddAdditionalClaim) (*smodel.EntityConfigurationAdditionalClaim, error) {
						return nil, errors.New("db down")
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"ACME","crit":false}`
			req := httptest.NewRequest("POST", "/entity-configuration/additional-claims", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestGetAdditionalClaimByID(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					getFn: func(_ string) (*smodel.EntityConfigurationAdditionalClaim, error) {
						return &smodel.EntityConfigurationAdditionalClaim{
							ID:    42,
							Claim: "org_name",
							Value: "ACME",
						}, nil
					},
				},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/additional-claims/42", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if got.ID != 42 || got.Claim != "org_name" {
				t.Errorf("unexpected response: %+v", got)
			}
		},
	)

	t.Run(
		"InvalidID", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/additional-claims/abc", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"NotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					getFn: func(_ string) (*smodel.EntityConfigurationAdditionalClaim, error) {
						return nil, errors.New("not found")
					},
				},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/additional-claims/99", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 404)
		},
	)
}

func TestPutAdditionalClaimByID(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					updateFn: func(
						_ string, item smodel.AddAdditionalClaim,
					) (*smodel.EntityConfigurationAdditionalClaim, error) {
						return &smodel.EntityConfigurationAdditionalClaim{
							ID:    5,
							Claim: item.Claim,
							Value: item.Value,
							Crit:  item.Crit,
						}, nil
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"UpdatedACME","crit":true}`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims/5", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if got.Claim != "org_name" || got.Crit != true {
				t.Errorf("unexpected response: %+v", got)
			}
		},
	)

	t.Run(
		"SuccessWithObjectValue_RealStore", func(t *testing.T) {
			t.Parallel()
			app, store := setupRealEntityConfigClaimsApp(t)

			seeded, err := store.Create(
				smodel.AddAdditionalClaim{
					Claim: "org_name",
					Value: "initial",
					Crit:  false,
				},
			)
			if err != nil {
				t.Fatalf("failed to seed additional claim: %v", err)
			}

			body := `{"claim":"org_name","value":{"display":"UpdatedACME","meta":{"region":"eu"}},"crit":true}`
			path := "/entity-configuration/additional-claims/" + strconv.FormatUint(uint64(seeded.ID), 10)
			req := httptest.NewRequest("PUT", path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)

			requireStatus(t, resp, respBody, http.StatusOK)

			var updated smodel.EntityConfigurationAdditionalClaim
			if err := json.Unmarshal(respBody, &updated); err != nil {
				t.Fatalf("failed to unmarshal PUT additional claim response: %v", err)
			}
			updatedValue := requireJSONMap(t, updated.Value, "updated org_name value")
			updatedMeta := requireJSONMap(t, updatedValue["meta"], "updated org_name meta")
			if updatedValue["display"] != "UpdatedACME" || updatedMeta["region"] != "eu" || !updated.Crit {
				t.Fatalf("expected object value in update response, got %+v", updated)
			}

			stored := requireEntityConfigClaimRecord(t, store, strconv.FormatUint(uint64(seeded.ID), 10))
			storedValue := requireJSONMap(t, stored.Value, "stored updated org_name value")
			storedMeta := requireJSONMap(t, storedValue["meta"], "stored updated org_name meta")
			if storedValue["display"] != "UpdatedACME" || storedMeta["region"] != "eu" || !stored.Crit {
				t.Fatalf("expected object value to persist after update, got %+v", stored)
			}
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims/5", strings.NewReader("bad"))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"NotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					updateFn: func(_ string, _ smodel.AddAdditionalClaim) (
						*smodel.EntityConfigurationAdditionalClaim, error,
					) {
						return nil, smodel.NotFoundError("not found")
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"X","crit":false}`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims/999", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 404)
		},
	)

	t.Run(
		"AlreadyExists", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					updateFn: func(_ string, _ smodel.AddAdditionalClaim) (
						*smodel.EntityConfigurationAdditionalClaim, error,
					) {
						return nil, smodel.AlreadyExistsError("duplicate claim")
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"X","crit":false}`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims/5", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 409)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					updateFn: func(_ string, _ smodel.AddAdditionalClaim) (
						*smodel.EntityConfigurationAdditionalClaim, error,
					) {
						return nil, errors.New("db down")
					},
				},
				&mockKeyValueStore{},
			)

			body := `{"claim":"org_name","value":"X","crit":false}`
			req := httptest.NewRequest("PUT", "/entity-configuration/additional-claims/5", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestDeleteAdditionalClaimByID(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					deleteFn: func(_ string) error {
						return nil
					},
				},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/additional-claims/42", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"InvalidID", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/additional-claims/abc", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"NotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{
					deleteFn: func(_ string) error {
						return errors.New("not found error from db")
					},
				},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/additional-claims/99", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 404)
		},
	)
}

func TestGetLifetime(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
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

			req := httptest.NewRequest("GET", "/entity-configuration/lifetime", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "3600" {
				t.Errorf("expected 3600, got %q", string(respBody))
			}
		},
	)

	t.Run(
		"NotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getAsFn: func(_, _ string, _ any) (bool, error) {
						return false, nil // Not found
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/lifetime", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "86400" {
				t.Errorf("expected 86400, got %q", string(respBody))
			}
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getAsFn: func(_, _ string, _ any) (bool, error) {
						return false, errors.New("kv db down")
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/lifetime", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"ZeroValueReturnsDefault", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getAsFn: func(_, _ string, out any) (bool, error) {
						ptr := out.(*int)
						*ptr = 0
						return true, nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/lifetime", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "86400" {
				t.Errorf("expected default 86400 for zero stored value, got %q", string(respBody))
			}
		},
	)
}

func TestPutLifetime(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
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

			req := httptest.NewRequest("PUT", "/entity-configuration/lifetime", strings.NewReader("7200"))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "7200" {
				t.Errorf("expected 7200 in response, got %q", string(respBody))
			}
		},
	)

	t.Run(
		"EmptyBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/lifetime", strings.NewReader(""))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/lifetime", strings.NewReader(`"not-int"`))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"NegativeValue", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/lifetime", strings.NewReader("-3600"))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					setAnyFn: func(_, _ string, _ any) error {
						return errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/lifetime", strings.NewReader("3600"))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"ZeroValueSuccess", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					setAnyFn: func(_, _ string, v any) error {
						if v.(int) != 0 {
							t.Errorf("expected 0 to be saved, got %v", v)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/lifetime", strings.NewReader("0"))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "0" {
				t.Errorf("expected 0 in response, got %q", string(respBody))
			}
		},
	)
}

func TestGetMetadata(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(scope, key string) (datatypes.JSON, error) {
						if scope == smodel.KeyValueScopeEntityConfiguration && key == smodel.KeyValueKeyMetadata {
							return datatypes.JSON(
								fmt.Sprintf(
									`{"%s":{"issuer":"https://example.com"}}`, oidfedconst.EntityTypeOpenIDProvider,
								),
							), nil
						}
						return nil, nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got oidfed.Metadata
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to parse metadata: %v", err)
			}
			if got.OpenIDProvider == nil || got.OpenIDProvider.Issuer != "https://example.com" {
				t.Errorf("expected op.issuer=https://example.com, got %+v", got.OpenIDProvider)
			}
		},
	)

	t.Run(
		"NoMetadataStored", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "{}" {
				t.Errorf("expected {}, got %q", string(respBody))
			}
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestPutMetadata(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
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

			body := fmt.Sprintf(`{"%s":{"issuer":"https://example.com"}}`, oidfedconst.EntityTypeOpenIDProvider)
			req := httptest.NewRequest("PUT", "/entity-configuration/metadata", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)

			var got oidfed.Metadata
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if got.OpenIDProvider == nil || got.OpenIDProvider.Issuer != "https://example.com" {
				t.Errorf("Expected OpenIDProvider.Issuer 'https://example.com', got %+v", got.OpenIDProvider)
			}
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest("PUT", "/entity-configuration/metadata", strings.NewReader(`"not-an-object"`))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					setFn: func(_, _ string, _ datatypes.JSON) error {
						return errors.New("db down")
					},
				},
			)

			body := fmt.Sprintf(`{"%s":{"issuer":"https://example.com"}}`, oidfedconst.EntityTypeOpenIDProvider)
			req := httptest.NewRequest("PUT", "/entity-configuration/metadata", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestGetMetadataClaim(t *testing.T) {
	t.Parallel()
	metaJSON := fmt.Sprintf(
		`{"%s":{"issuer":"https://example.com","some_claim":123}}`, oidfedconst.EntityTypeOpenIDProvider,
	)

	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(metaJSON), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider/issuer", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != `"https://example.com"` {
				t.Errorf("expected string JSON, got %s", respBody)
			}
		},
	)

	t.Run(
		"NoMetadataStored", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider/issuer", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 404)
		},
	)

	t.Run(
		"EntityTypeNotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(metaJSON), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/oauth_client/issuer", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 404)
		},
	)

	t.Run(
		"ClaimNotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(metaJSON), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider/missing", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 404)
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider/issuer", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider/issuer", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestPutMetadataClaim(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success_ExistingMeta", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"old":123}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						s := string(value)
						if !strings.Contains(s, `"old":123`) || !strings.Contains(s, `"new":456`) {
							t.Errorf("expected merged json, got %s", s)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, 200)

			if string(body) != "456" {
				t.Errorf("Expected response body to echo back '456', got %q", string(body))
			}
		},
	)

	t.Run(
		"Success_NoExistingMeta", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						if !strings.Contains(string(value), `"new":456`) {
							t.Errorf("expected json, got %s", value)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, 200)

			if string(body) != "456" {
				t.Errorf("Expected response body to echo back '456', got %q", string(body))
			}
		},
	)

	t.Run(
		"EmptyBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider/new", strings.NewReader(""),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"StoreGetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db down during get")
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreSetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
					setFn: func(_, _ string, _ datatypes.JSON) error {
						return errors.New("db down during set")
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider/new", strings.NewReader(`456`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestDeleteMetadataClaim(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"target":123,"other":456}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
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

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider/target", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"LastClaimRemovesEntityType", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"only_claim":1}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						s := string(value)
						if strings.Contains(s, fmt.Sprintf("\"%s\"", oidfedconst.EntityTypeOpenIDProvider)) {
							t.Errorf("entity type should be removed when its last claim is deleted: %s", s)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest(
				"DELETE", "/entity-configuration/metadata/openid_provider/only_claim", http.NoBody,
			)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"NoMetadataStored", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider/target", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"EntityTypeNotInMeta", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{"other_type":{"target":123}}`), nil
					},
					setFn: func(_, _ string, _ datatypes.JSON) error {
						t.Errorf("set should not be called")
						return nil
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider/target", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider/target", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreGetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider/target", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreSetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"target":123}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, _ datatypes.JSON) error {
						return errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider/target", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestGetMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"issuer":"https://example.com"}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			var got map[string]json.RawMessage
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if string(got["issuer"]) != `"https://example.com"` {
				t.Errorf("expected url, got %s", got["issuer"])
			}
		},
	)

	t.Run(
		"NoMetadataStored", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "{}" {
				t.Errorf("expected {}, got %s", respBody)
			}
		},
	)

	t.Run(
		"EntityTypeNotFound", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(fmt.Sprintf(`{"%s":{"x":1}}`, oidfedconst.EntityTypeOAuthClient)), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, respBody := doRequest(t, app, req)
			requireStatus(t, resp, respBody, 200)
			if string(respBody) != "{}" {
				t.Errorf("expected {}, got %s", respBody)
			}
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("GET", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestPutMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"old":1},"other":{}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						s := string(value)
						if strings.Contains(s, `"old":1`) {
							t.Errorf("should have replaced openid_provider entirely: %s", s)
						}
						if !strings.Contains(s, `"new":2`) {
							t.Errorf("should contain new value: %s", s)
						}
						if !strings.Contains(s, `"other"`) {
							t.Errorf("should retain other types: %s", s)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, 200)

			var got map[string]json.RawMessage
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if _, ok := got["new"]; !ok {
				t.Errorf("Expected response to contain 'new' claim, got keys %v", got)
			}
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider", strings.NewReader(`not-json`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"StoreGetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreSetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
					setFn: func(_, _ string, _ datatypes.JSON) error {
						return errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest(
				"PUT", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestPostMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success_MergesInto", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"old":1}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						s := string(value)
						if !strings.Contains(s, `"old":1`) || !strings.Contains(s, `"new":2`) {
							t.Errorf("should merge existing and new: %s", s)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest(
				"POST", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, 200)

			var got map[string]json.RawMessage
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if _, ok := got["new"]; !ok {
				t.Errorf("Expected response to contain 'new' claim, got keys %v", got)
			}
		},
	)

	t.Run(
		"Success_CreatesNew", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						s := string(value)
						if !strings.Contains(s, fmt.Sprintf(`"%s":{"new":2}`, oidfedconst.EntityTypeOpenIDProvider)) {
							t.Errorf("should create new: %s", s)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest(
				"POST", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, 200)

			var got map[string]json.RawMessage
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if _, ok := got["new"]; !ok {
				t.Errorf("Expected response to contain 'new' claim, got keys %v", got)
			}
		},
	)

	t.Run(
		"InvalidBody", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{},
			)

			req := httptest.NewRequest(
				"POST", "/entity-configuration/metadata/openid_provider", strings.NewReader(`not-json`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 400)
		},
	)

	t.Run(
		"StoreGetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest(
				"POST", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreSetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
					setFn: func(_, _ string, _ datatypes.JSON) error {
						return errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest(
				"POST", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest(
				"POST", "/entity-configuration/metadata/openid_provider", strings.NewReader(`{"new":2}`),
			)
			req.Header.Set("Content-Type", "application/json")
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}

func TestDeleteMetadataEntityType(t *testing.T) {
	t.Parallel()
	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"target":123},"other":{}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, value datatypes.JSON) error {
						s := string(value)
						if strings.Contains(s, fmt.Sprintf("\"%s\"", oidfedconst.EntityTypeOpenIDProvider)) {
							t.Errorf("entity type was not deleted: %s", s)
						}
						if !strings.Contains(s, `"other"`) {
							t.Errorf("wrong entity type deleted: %s", s)
						}
						return nil
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"NoMetadataStored", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, nil
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 204)
		},
	)

	t.Run(
		"CorruptStoredMetadata", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(`{bad-json`), nil
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreGetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return nil, errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)

	t.Run(
		"StoreSetError", func(t *testing.T) {
			t.Parallel()
			app := setupEntityConfigTestApp(
				newStubFedEntity(),
				&mockAdditionalClaimsStore{},
				&mockKeyValueStore{
					getFn: func(_, _ string) (datatypes.JSON, error) {
						return datatypes.JSON(
							fmt.Sprintf(
								`{"%s":{"target":123}}`, oidfedconst.EntityTypeOpenIDProvider,
							),
						), nil
					},
					setFn: func(_, _ string, _ datatypes.JSON) error {
						return errors.New("db error")
					},
				},
			)

			req := httptest.NewRequest("DELETE", "/entity-configuration/metadata/openid_provider", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, 500)
		},
	)
}
