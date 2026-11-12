package adminapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/internal/version"
	"github.com/go-oidfed/lighthouse/storage"
)

// infoSigner is a TrustMarkSigner that is never asked to sign.
type infoSigner struct{}

func (infoSigner) SignTrustMark(string, string, map[string]any) (SignedTrustMark, error) {
	return SignedTrustMark{}, ErrIssuanceUnavailable
}
func (infoSigner) ServeTrustMark(string, string, SignedTrustMark, time.Duration) {}

type infoAnswer struct {
	Version  string   `json:"version"`
	EntityID *string  `json:"entity_id"`
	Features []string `json:"features"`
}

// getInfo registers the admin API with opts and the entity, and calls GET /info.
func getInfo(t *testing.T, opts *Options, entity *mockFederationEntity, authorization string) infoAnswer {
	t.Helper()
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	app := fiber.New()
	if entity == nil {
		err = Register(app, "https://ta.example.org", backends, nil, KeyManagement{}, nil, opts)
	} else {
		err = Register(app, "https://ta.example.org", backends, entity, KeyManagement{}, nil, opts)
	}
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, body, 200)
	var info infoAnswer
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("Failed to decode %s: %v", body, err)
	}
	return info
}

func TestInfo_VersionAndAlwaysAvailableFeatures(t *testing.T) {
	info := getInfo(t, nil, nil, "")
	if info.Version != version.VERSION || info.Version == "" {
		t.Errorf("version = %q, want %q", info.Version, version.VERSION)
	}
	want := []string{"conditional_status", "events", "trust_mark_instances"}
	if !slices.Equal(info.Features, want) {
		t.Errorf("features = %v, want %v", info.Features, want)
	}
	if info.EntityID != nil {
		t.Errorf("entity_id = %q, want it omitted without an entity", *info.EntityID)
	}
}

func TestInfo_WithoutTheInstanceStore(t *testing.T) {
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	backends.TrustMarkInstances = nil
	app := fiber.New()
	if err := Register(
		app, "https://ta.example.org", backends, nil, KeyManagement{}, nil, &Options{TrustMarkSigner: infoSigner{}},
	); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/info", nil))
	requireStatus(t, resp, body, 200)
	var info infoAnswer
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("Failed to decode %s: %v", body, err)
	}
	want := []string{"conditional_status", "events"}
	if !slices.Equal(info.Features, want) {
		t.Errorf("features = %v, want %v without the instance store", info.Features, want)
	}
}

func TestInfo_TypedNilEntityGivesNoEntityID(t *testing.T) {
	var entity *mockFederationEntity
	info := getInfo(t, nil, entity, "")
	if info.EntityID != nil {
		t.Errorf("entity_id = %q, want it omitted for a nil entity", *info.EntityID)
	}
}

func TestInfo_EntityID(t *testing.T) {
	info := getInfo(t, nil, &mockFederationEntity{entityID: "https://ta.example.org"}, "")
	if info.EntityID == nil || *info.EntityID != "https://ta.example.org" {
		t.Errorf("entity_id = %v, want https://ta.example.org", info.EntityID)
	}
}

func TestInfo_TrustMarkIssuanceNeedsASigner(t *testing.T) {
	info := getInfo(t, &Options{TrustMarkSigner: infoSigner{}}, nil, "")
	if !slices.Contains(info.Features, "trust_mark_issuance") {
		t.Errorf("features = %v, want trust_mark_issuance with a signer", info.Features)
	}
}

func TestInfo_EnrollCheckNeedsAConfiguredChecker(t *testing.T) {
	for name, tc := range map[string]struct {
		checker EnrollChecker
		want    bool
	}{
		"no checker":         {nil, false},
		"checker not set up": {&fakeEnrollChecker{}, false},
		"checker configured": {&fakeEnrollChecker{res: EnrollCheckResult{Configured: true}}, true},
	} {
		t.Run(
			name, func(t *testing.T) {
				info := getInfo(t, &Options{EnrollChecker: tc.checker}, nil, "")
				if got := slices.Contains(info.Features, "enroll_check"); got != tc.want {
					t.Errorf("enroll_check listed = %v, want %v (%v)", got, tc.want, info.Features)
				}
			},
		)
	}
}

// The checker's answer is read per request, as endpoints can be reloaded.
func TestInfo_EnrollCheckFollowsTheCheckerAtRequestTime(t *testing.T) {
	checker := &fakeEnrollChecker{}
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	app := fiber.New()
	if err := Register(
		app, "https://ta.example.org", backends, nil, KeyManagement{}, nil, &Options{EnrollChecker: checker},
	); err != nil {
		t.Fatalf("Register: %v", err)
	}
	listed := func() bool {
		resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/info", nil))
		requireStatus(t, resp, body, 200)
		var info infoAnswer
		if err := json.Unmarshal(body, &info); err != nil {
			t.Fatalf("Failed to decode %s: %v", body, err)
		}
		return slices.Contains(info.Features, "enroll_check")
	}
	if listed() {
		t.Fatal("enroll_check listed while the checker is not configured")
	}
	checker.res.Configured = true
	if !listed() {
		t.Error("enroll_check not listed once the checker is configured")
	}
}

func TestInfo_ServiceTokensAndActorDelegation(t *testing.T) {
	plain := mustServiceToken(t, "plain", testServiceSecret, false)
	acting := mustServiceToken(t, "acting", otherServiceSecret, true)

	t.Run(
		"tokens without act_for_users", func(t *testing.T) {
			info := getInfo(t, &Options{ServiceTokens: ServiceTokens{plain}}, nil, "")
			if !slices.Contains(info.Features, "service_tokens") || slices.Contains(info.Features, "actor_delegation") {
				t.Errorf("features = %v, want service_tokens only", info.Features)
			}
		},
	)
	t.Run(
		"a token acting for users, on the service listener", func(t *testing.T) {
			info := getInfo(
				t, &Options{ServiceTokens: ServiceTokens{plain, acting}, ServiceListener: true}, nil,
				bearer(testServiceSecret),
			)
			if !slices.Contains(info.Features, "service_tokens") || !slices.Contains(info.Features, "actor_delegation") {
				t.Errorf("features = %v, want service_tokens and actor_delegation", info.Features)
			}
		},
	)
}

func TestInfo_AllFeaturesInDocumentedOrder(t *testing.T) {
	acting := mustServiceToken(t, "acting", testServiceSecret, true)
	info := getInfo(
		t, &Options{
			TrustMarkSigner: infoSigner{},
			EnrollChecker:   &fakeEnrollChecker{res: EnrollCheckResult{Configured: true}},
			ServiceTokens:   ServiceTokens{acting},
		}, nil, "",
	)
	if !slices.Equal(info.Features, infoFeatures) {
		t.Errorf("features = %v, want %v", info.Features, infoFeatures)
	}
}

// Both listeners answer /info under their own authentication.
func TestInfo_UsesTheListenersAuthentication(t *testing.T) {
	tok := mustServiceToken(t, "svc", testServiceSecret, false)
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	createTestUser(t, backends, "admin", false)
	for name, listener := range map[string]bool{"admin port": false, "service listener": true} {
		app := fiber.New()
		if err := Register(
			app, "https://ta.example.org", backends, nil, KeyManagement{}, nil,
			&Options{UsersEnabled: true, ServiceTokens: ServiceTokens{tok}, ServiceListener: listener},
		); err != nil {
			t.Fatalf("Register: %v", err)
		}
		resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/info", nil))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: unauthenticated /info = %d (%s), want 401", name, resp.StatusCode, body)
		}
	}
	for name, tc := range map[string]struct {
		listener      bool
		authorization string
	}{
		"admin port accepts Basic":          {false, basicAuthHeader("admin", "secret-password")},
		"service listener accepts a bearer": {true, bearer(testServiceSecret)},
	} {
		app := fiber.New()
		if err := Register(
			app, "https://ta.example.org", backends, nil, KeyManagement{}, nil,
			&Options{UsersEnabled: true, ServiceTokens: ServiceTokens{tok}, ServiceListener: tc.listener},
		); err != nil {
			t.Fatalf("Register: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/info", nil)
		req.Header.Set("Authorization", tc.authorization)
		resp, body := doRequest(t, app, req)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: /info = %d (%s), want 200", name, resp.StatusCode, body)
		}
	}
}

func TestInfo_OpenAPIFeatureEnumMatchesTheCode(t *testing.T) {
	raw, err := assets.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	var doc struct {
		Paths map[string]struct {
			Get *struct {
				Tags      []string `yaml:"tags"`
				Responses map[string]struct {
					Content map[string]struct {
						Schema struct {
							Properties map[string]struct {
								Items struct {
									Enum []string `yaml:"enum"`
								} `yaml:"items"`
							} `yaml:"properties"`
						} `yaml:"schema"`
					} `yaml:"content"`
				} `yaml:"responses"`
			} `yaml:"get"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("failed to parse openapi.yaml: %v", err)
	}
	path, ok := doc.Paths["/api/v1/admin/info"]
	if !ok || path.Get == nil {
		t.Fatal("openapi.yaml has no GET /api/v1/admin/info")
	}
	op := path.Get
	if !slices.Contains(op.Tags, "Info") {
		t.Errorf("GET /info tags = %v, want Info", op.Tags)
	}
	got := op.Responses["200"].Content["application/json"].Schema.Properties["features"].Items.Enum
	if !slices.Equal(got, infoFeatures) {
		t.Errorf("openapi features enum = %v, want the code's %v", got, infoFeatures)
	}
}
