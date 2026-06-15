package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage"
	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

type mockPublishedTrustMarksStore struct {
	listFn   func() ([]smodel.PublishedTrustMark, error)
	createFn func(smodel.AddTrustMark) (*smodel.PublishedTrustMark, error)
	getFn    func(string) (*smodel.PublishedTrustMark, error)
	updateFn func(string, smodel.AddTrustMark) (*smodel.PublishedTrustMark, error)
	patchFn  func(string, smodel.UpdateTrustMark) (*smodel.PublishedTrustMark, error)
	deleteFn func(string) error
}

func (m *mockPublishedTrustMarksStore) List() ([]smodel.PublishedTrustMark, error) {
	return m.listFn()
}

func (m *mockPublishedTrustMarksStore) Create(item smodel.AddTrustMark) (*smodel.PublishedTrustMark, error) {
	return m.createFn(item)
}

func (m *mockPublishedTrustMarksStore) Get(ident string) (*smodel.PublishedTrustMark, error) {
	return m.getFn(ident)
}

func (m *mockPublishedTrustMarksStore) Update(ident string, item smodel.AddTrustMark) (
	*smodel.PublishedTrustMark, error,
) {
	return m.updateFn(ident, item)
}

func (m *mockPublishedTrustMarksStore) Patch(ident string, item smodel.UpdateTrustMark) (
	*smodel.PublishedTrustMark, error,
) {
	return m.patchFn(ident, item)
}

func (m *mockPublishedTrustMarksStore) Delete(ident string) error {
	return m.deleteFn(ident)
}

type mockTrustMarkConfigInvalidator struct {
	calls int
}

func (m *mockTrustMarkConfigInvalidator) Invalidate() {
	m.calls++
}

func setupEntityTrustMarksTestApp(
	store smodel.PublishedTrustMarksStore,
	configInvalidator TrustMarkConfigInvalidator,
) *fiber.App {
	app := fiber.New()
	registerEntityTrustMarks(app, store, configInvalidator)
	return app
}

func setupRealEntityTrustMarksTestApp(t *testing.T) (
	*fiber.App, smodel.PublishedTrustMarksStore, *mockTrustMarkConfigInvalidator,
) {
	t.Helper()
	store := newTestStorage(t)
	backends, err := store.Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatal(err.Error())
	}
	invalidator := &mockTrustMarkConfigInvalidator{}
	app := setupEntityTrustMarksTestApp(backends.PublishedTrustMarks, invalidator)
	return app, backends.PublishedTrustMarks, invalidator
}

func requirePublishedTrustMarkRecord(
	t *testing.T, store smodel.PublishedTrustMarksStore, ident string,
) *smodel.PublishedTrustMark {
	t.Helper()
	item, err := store.Get(ident)
	if err != nil {
		t.Fatalf("failed to get published trust mark %q: %v", ident, err)
	}
	if item == nil {
		t.Fatalf("expected published trust mark %q to exist", ident)
	}
	return item
}

func requireSelfIssuanceSpec(
	t *testing.T, spec *smodel.SelfIssuedTrustMarkSpec, name string,
) *smodel.SelfIssuedTrustMarkSpec {
	t.Helper()
	if spec == nil {
		t.Fatalf("expected %s to be present", name)
	}
	return spec
}

// TestEntityTrustMarksRealStoragePersistence must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestEntityTrustMarksRealStoragePersistence(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run(
		"CreatePersistsSelfIssuanceSpec", func(t *testing.T) {
			app, store, invalidator := setupRealEntityTrustMarksTestApp(t)
			setEntityConfigurationCache(t, cacheValue)

			body := `{
			"trust_mark_type":"https://tm.example/self-issued",
			"refresh":true,
			"self_issuance_spec":{
				"lifetime":3600,
				"ref":"https://tm.example/specs/self-issued",
				"logo_uri":"https://tm.example/logo.svg",
				"additional_claims":{
					"labels":{"tier":"gold"},
					"display":"ACME"
				},
				"include_extra_claims_in_info":true
			}
		}`
			req := httptest.NewRequest(http.MethodPost, "/entity-configuration/trust-marks/", strings.NewReader(body))
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, respBody := doRequest(t, app, req)

			requireStatus(t, resp, respBody, http.StatusCreated)

			var created smodel.PublishedTrustMark
			if err := json.Unmarshal(respBody, &created); err != nil {
				t.Fatalf("failed to unmarshal create trust mark response: %v", err)
			}
			createdSpec := requireSelfIssuanceSpec(t, created.SelfIssuanceSpec, "created self issuance spec")
			createdClaims := requireJSONMap(t, createdSpec.AdditionalClaims, "created self issuance claims")
			createdLabels := requireJSONMap(t, createdClaims["labels"], "created self issuance labels")
			if createdSpec.Lifetime != 3600 || createdSpec.Ref != "https://tm.example/specs/self-issued" || createdLabels["tier"] != "gold" {
				t.Fatalf("expected create response to include structured self issuance spec, got %+v", created)
			}

			stored := requirePublishedTrustMarkRecord(t, store, strconv.FormatUint(uint64(created.ID), 10))
			storedSpec := requireSelfIssuanceSpec(t, stored.SelfIssuanceSpec, "stored self issuance spec")
			storedClaims := requireJSONMap(t, storedSpec.AdditionalClaims, "stored self issuance claims")
			storedLabels := requireJSONMap(t, storedClaims["labels"], "stored self issuance labels")
			if storedSpec.Lifetime != 3600 || storedSpec.LogoURI != "https://tm.example/logo.svg" || storedClaims["display"] != "ACME" || storedLabels["tier"] != "gold" {
				t.Fatalf("expected self issuance spec to persist, got %+v", stored)
			}
			if !stored.Refresh {
				t.Fatalf("expected refresh flag to persist, got %+v", stored)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)

	t.Run(
		"UpdatePersistsSelfIssuanceSpec", func(t *testing.T) {
			app, store, invalidator := setupRealEntityTrustMarksTestApp(t)
			setEntityConfigurationCache(t, cacheValue)

			seeded, err := store.Create(
				smodel.AddTrustMark{
					TrustMarkType: "https://tm.example/self-issued",
					SelfIssuanceSpec: &smodel.SelfIssuedTrustMarkSpec{
						Lifetime: 900,
						AdditionalClaims: map[string]any{
							"labels": map[string]any{"tier": "silver"},
						},
					},
				},
			)
			if err != nil {
				t.Fatalf("failed to seed published trust mark: %v", err)
			}

			body := `{
			"trust_mark_type":"https://tm.example/self-issued",
			"refresh":true,
			"min_lifetime":120,
			"self_issuance_spec":{
				"lifetime":2400,
				"ref":"https://tm.example/specs/updated",
				"additional_claims":{
					"labels":{"tier":"platinum"},
					"display":"UpdatedACME"
				}
			}
		}`
			path := "/entity-configuration/trust-marks/" + strconv.FormatUint(uint64(seeded.ID), 10)
			req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, respBody := doRequest(t, app, req)

			requireStatus(t, resp, respBody, http.StatusOK)

			var updated smodel.PublishedTrustMark
			if err := json.Unmarshal(respBody, &updated); err != nil {
				t.Fatalf("failed to unmarshal update trust mark response: %v", err)
			}
			updatedSpec := requireSelfIssuanceSpec(t, updated.SelfIssuanceSpec, "updated self issuance spec")
			updatedClaims := requireJSONMap(t, updatedSpec.AdditionalClaims, "updated self issuance claims")
			updatedLabels := requireJSONMap(t, updatedClaims["labels"], "updated self issuance labels")
			if updatedSpec.Lifetime != 2400 || updatedClaims["display"] != "UpdatedACME" || updatedLabels["tier"] != "platinum" {
				t.Fatalf("expected update response to include structured self issuance spec, got %+v", updated)
			}

			stored := requirePublishedTrustMarkRecord(t, store, strconv.FormatUint(uint64(seeded.ID), 10))
			storedSpec := requireSelfIssuanceSpec(t, stored.SelfIssuanceSpec, "stored updated self issuance spec")
			storedClaims := requireJSONMap(t, storedSpec.AdditionalClaims, "stored updated self issuance claims")
			storedLabels := requireJSONMap(t, storedClaims["labels"], "stored updated self issuance labels")
			if stored.MinLifetime != 120 || !stored.Refresh || storedSpec.Ref != "https://tm.example/specs/updated" || storedClaims["display"] != "UpdatedACME" || storedLabels["tier"] != "platinum" {
				t.Fatalf("expected updated self issuance spec to persist, got %+v", stored)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)

	t.Run(
		"PatchPersistsSelfIssuanceSpec", func(t *testing.T) {
			app, store, invalidator := setupRealEntityTrustMarksTestApp(t)
			setEntityConfigurationCache(t, cacheValue)

			seeded, err := store.Create(
				smodel.AddTrustMark{
					TrustMarkType: "https://tm.example/self-issued",
					SelfIssuanceSpec: &smodel.SelfIssuedTrustMarkSpec{
						Lifetime: 1200,
						AdditionalClaims: map[string]any{
							"labels": map[string]any{"tier": "bronze"},
						},
					},
				},
			)
			if err != nil {
				t.Fatalf("failed to seed published trust mark for patch: %v", err)
			}

			body := `{
			"refresh":true,
			"self_issuance_spec":{
				"lifetime":4800,
				"logo_uri":"https://tm.example/logo-updated.svg",
				"additional_claims":{
					"labels":{"tier":"diamond"},
					"meta":{"region":"eu"}
				},
				"include_extra_claims_in_info":true
			}
		}`
			path := "/entity-configuration/trust-marks/" + strconv.FormatUint(uint64(seeded.ID), 10)
			req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, respBody := doRequest(t, app, req)

			requireStatus(t, resp, respBody, http.StatusOK)

			var patched smodel.PublishedTrustMark
			if err := json.Unmarshal(respBody, &patched); err != nil {
				t.Fatalf("failed to unmarshal patch trust mark response: %v", err)
			}
			patchedSpec := requireSelfIssuanceSpec(t, patched.SelfIssuanceSpec, "patched self issuance spec")
			patchedClaims := requireJSONMap(t, patchedSpec.AdditionalClaims, "patched self issuance claims")
			patchedLabels := requireJSONMap(t, patchedClaims["labels"], "patched self issuance labels")
			patchedMeta := requireJSONMap(t, patchedClaims["meta"], "patched self issuance meta")
			if patchedSpec.Lifetime != 4800 || patchedSpec.LogoURI != "https://tm.example/logo-updated.svg" || patchedLabels["tier"] != "diamond" || patchedMeta["region"] != "eu" {
				t.Fatalf("expected patch response to include structured self issuance spec, got %+v", patched)
			}

			stored := requirePublishedTrustMarkRecord(t, store, strconv.FormatUint(uint64(seeded.ID), 10))
			storedSpec := requireSelfIssuanceSpec(t, stored.SelfIssuanceSpec, "stored patched self issuance spec")
			storedClaims := requireJSONMap(t, storedSpec.AdditionalClaims, "stored patched self issuance claims")
			storedLabels := requireJSONMap(t, storedClaims["labels"], "stored patched self issuance labels")
			storedMeta := requireJSONMap(t, storedClaims["meta"], "stored patched self issuance meta")
			if !stored.Refresh || !storedSpec.IncludeExtraClaimsInInfo || storedLabels["tier"] != "diamond" || storedMeta["region"] != "eu" {
				t.Fatalf("expected patched self issuance spec to persist, got %+v", stored)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)
}

func TestEntityTrustMarksList(t *testing.T) {
	t.Parallel()

	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()

			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					listFn: func() ([]smodel.PublishedTrustMark, error) {
						return []smodel.PublishedTrustMark{
							{
								ID:            1,
								TrustMarkType: "https://tm.example/type",
							},
						}, nil
					},
				}, nil,
			)

			req := httptest.NewRequest(http.MethodGet, "/entity-configuration/trust-marks/", http.NoBody)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var got []smodel.PublishedTrustMark
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if len(got) != 1 || got[0].TrustMarkType != "https://tm.example/type" {
				t.Fatalf("unexpected trust mark list: %+v", got)
			}
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()

			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					listFn: func() ([]smodel.PublishedTrustMark, error) {
						return nil, errors.New("db down")
					},
				}, nil,
			)

			req := httptest.NewRequest(http.MethodGet, "/entity-configuration/trust-marks/", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusInternalServerError)
		},
	)
}

// TestEntityTrustMarksCreate must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestEntityTrustMarksCreate(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run(
		"SuccessInvalidatesCacheAndConfig", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			var gotInput smodel.AddTrustMark
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					createFn: func(item smodel.AddTrustMark) (*smodel.PublishedTrustMark, error) {
						gotInput = item
						return &smodel.PublishedTrustMark{
							ID:              3,
							TrustMarkType:   item.TrustMarkType,
							TrustMarkIssuer: item.TrustMarkIssuer,
							Refresh:         item.Refresh,
						}, nil
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/entity-configuration/trust-marks/",
				strings.NewReader(`{"trust_mark_type":"https://tm.example/type","trust_mark_issuer":"https://issuer.example","refresh":true}`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusCreated)

			var got smodel.PublishedTrustMark
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if gotInput.TrustMarkType != "https://tm.example/type" || gotInput.TrustMarkIssuer != "https://issuer.example" || !gotInput.Refresh {
				t.Fatalf("unexpected create input: %+v", gotInput)
			}
			if got.ID != 3 || got.TrustMarkType != gotInput.TrustMarkType {
				t.Fatalf("unexpected response payload: %+v", got)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)

	t.Run(
		"InvalidBodyKeepsCacheAndSkipsInvalidator", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					createFn: func(_ smodel.AddTrustMark) (*smodel.PublishedTrustMark, error) {
						t.Fatalf("create should not be called for invalid body")
						return nil, nil
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/entity-configuration/trust-marks/",
				strings.NewReader(`{"trust_mark_type":`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusBadRequest)
			if invalidator.calls != 0 {
				t.Fatalf("expected invalidator to stay at 0 calls, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, true, cacheValue)
		},
	)

	t.Run(
		"ConflictKeepsCacheAndSkipsInvalidator", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					createFn: func(_ smodel.AddTrustMark) (*smodel.PublishedTrustMark, error) {
						return nil, smodel.AlreadyExistsError("trust mark already exists")
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/entity-configuration/trust-marks/",
				strings.NewReader(`{"trust_mark_type":"https://tm.example/type"}`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusConflict)
			if invalidator.calls != 0 {
				t.Fatalf("expected invalidator to stay at 0 calls, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, true, cacheValue)
		},
	)
}

func TestEntityTrustMarksGet(t *testing.T) {
	t.Parallel()

	t.Run(
		"Success", func(t *testing.T) {
			t.Parallel()

			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					getFn: func(ident string) (*smodel.PublishedTrustMark, error) {
						if ident != "3" {
							t.Fatalf("unexpected trust mark id %q", ident)
						}
						return &smodel.PublishedTrustMark{
							ID:            3,
							TrustMarkType: "https://tm.example/type",
						}, nil
					},
				}, nil,
			)

			req := httptest.NewRequest(http.MethodGet, "/entity-configuration/trust-marks/3", http.NoBody)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var got smodel.PublishedTrustMark
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if got.ID != 3 || got.TrustMarkType != "https://tm.example/type" {
				t.Fatalf("unexpected response payload: %+v", got)
			}
		},
	)

	t.Run(
		"StoreError", func(t *testing.T) {
			t.Parallel()

			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					getFn: func(string) (*smodel.PublishedTrustMark, error) {
						return nil, errors.New("db down")
					},
				}, nil,
			)

			req := httptest.NewRequest(http.MethodGet, "/entity-configuration/trust-marks/3", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusInternalServerError)
		},
	)
}

// TestEntityTrustMarksUpdate must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestEntityTrustMarksUpdate(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run(
		"SuccessInvalidatesCacheAndConfig", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			var gotID string
			var gotInput smodel.AddTrustMark
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					updateFn: func(ident string, item smodel.AddTrustMark) (*smodel.PublishedTrustMark, error) {
						gotID = ident
						gotInput = item
						return &smodel.PublishedTrustMark{
							ID:              7,
							TrustMarkType:   item.TrustMarkType,
							TrustMarkIssuer: item.TrustMarkIssuer,
						}, nil
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPut,
				"/entity-configuration/trust-marks/7",
				strings.NewReader(`{"trust_mark_type":"https://tm.example/updated","trust_mark_issuer":"https://issuer.example"}`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var got smodel.PublishedTrustMark
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if gotID != "7" {
				t.Fatalf("expected trust mark id %q, got %q", "7", gotID)
			}
			if gotInput.TrustMarkType != "https://tm.example/updated" || gotInput.TrustMarkIssuer != "https://issuer.example" {
				t.Fatalf("unexpected update input: %+v", gotInput)
			}
			if got.TrustMarkType != gotInput.TrustMarkType {
				t.Fatalf("unexpected response payload: %+v", got)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)

	t.Run(
		"NotFoundKeepsCacheAndSkipsInvalidator", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					updateFn: func(string, smodel.AddTrustMark) (*smodel.PublishedTrustMark, error) {
						return nil, smodel.NotFoundError("missing")
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPut,
				"/entity-configuration/trust-marks/missing",
				strings.NewReader(`{"trust_mark_type":"https://tm.example/updated"}`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusNotFound)
			if invalidator.calls != 0 {
				t.Fatalf("expected invalidator to stay at 0 calls, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, true, cacheValue)
		},
	)
}

// TestEntityTrustMarksPatch must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestEntityTrustMarksPatch(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run(
		"SuccessInvalidatesCacheAndConfig", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			var gotID string
			var gotInput smodel.UpdateTrustMark
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					patchFn: func(ident string, item smodel.UpdateTrustMark) (*smodel.PublishedTrustMark, error) {
						gotID = ident
						gotInput = item
						return &smodel.PublishedTrustMark{
							ID:              9,
							TrustMarkIssuer: *item.TrustMarkIssuer,
							Refresh:         *item.Refresh,
						}, nil
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPatch,
				"/entity-configuration/trust-marks/9",
				strings.NewReader(`{"trust_mark_issuer":"https://issuer.example","refresh":true}`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, body := doRequest(t, app, req)
			requireStatus(t, resp, body, http.StatusOK)

			var got smodel.PublishedTrustMark
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if gotID != "9" {
				t.Fatalf("expected trust mark id %q, got %q", "9", gotID)
			}
			if gotInput.TrustMarkIssuer == nil || *gotInput.TrustMarkIssuer != "https://issuer.example" {
				t.Fatalf("unexpected patch issuer input: %+v", gotInput)
			}
			if gotInput.Refresh == nil || !*gotInput.Refresh {
				t.Fatalf("unexpected patch refresh input: %+v", gotInput)
			}
			if got.TrustMarkIssuer != *gotInput.TrustMarkIssuer || !got.Refresh {
				t.Fatalf("unexpected response payload: %+v", got)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)

	t.Run(
		"ValidationErrorKeepsCacheAndSkipsInvalidator", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					patchFn: func(string, smodel.UpdateTrustMark) (*smodel.PublishedTrustMark, error) {
						return nil, smodel.ValidationError("invalid trust mark")
					},
				}, invalidator,
			)

			req := httptest.NewRequest(
				http.MethodPatch,
				"/entity-configuration/trust-marks/9",
				strings.NewReader(`{"refresh":true}`),
			)
			req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusBadRequest)
			if invalidator.calls != 0 {
				t.Fatalf("expected invalidator to stay at 0 calls, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, true, cacheValue)
		},
	)
}

// TestEntityTrustMarksDelete must NOT use t.Parallel().
// It uses the global entity configuration cache via setEntityConfigurationCache.
func TestEntityTrustMarksDelete(t *testing.T) {
	cacheValue := []byte("cached-entity-config")

	t.Run(
		"SuccessInvalidatesCacheAndConfig", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			var gotID string
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					deleteFn: func(ident string) error {
						gotID = ident
						return nil
					},
				}, invalidator,
			)

			req := httptest.NewRequest(http.MethodDelete, "/entity-configuration/trust-marks/14", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusNoContent)
			if gotID != "14" {
				t.Fatalf("expected trust mark id %q, got %q", "14", gotID)
			}
			if invalidator.calls != 1 {
				t.Fatalf("expected invalidator to be called once, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, false, nil)
		},
	)

	t.Run(
		"NotFoundKeepsCacheAndSkipsInvalidator", func(t *testing.T) {
			invalidator := &mockTrustMarkConfigInvalidator{}
			setEntityConfigurationCache(t, cacheValue)
			app := setupEntityTrustMarksTestApp(
				&mockPublishedTrustMarksStore{
					deleteFn: func(string) error {
						return smodel.NotFoundError("missing")
					},
				}, invalidator,
			)

			req := httptest.NewRequest(http.MethodDelete, "/entity-configuration/trust-marks/missing", http.NoBody)
			resp, bodyBytes := doRequest(t, app, req)
			requireStatus(t, resp, bodyBytes, http.StatusNotFound)
			if invalidator.calls != 0 {
				t.Fatalf("expected invalidator to stay at 0 calls, got %d", invalidator.calls)
			}
			requireEntityConfigurationCache(t, true, cacheValue)
		},
	)
}
