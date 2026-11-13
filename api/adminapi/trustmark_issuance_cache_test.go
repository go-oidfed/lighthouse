package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// recordingCacheInvalidator records the trust mark cache invalidations it receives.
type recordingCacheInvalidator struct {
	mu    sync.Mutex
	marks []string
	types []string
}

func (r *recordingCacheInvalidator) InvalidateTrustMark(trustMarkType, entityID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.marks = append(r.marks, trustMarkType+"|"+entityID)
}

func (r *recordingCacheInvalidator) InvalidateTrustMarkType(trustMarkType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types = append(r.types, trustMarkType)
}

func (r *recordingCacheInvalidator) assert(t *testing.T, wantMarks, wantTypes []string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	marks := slices.Sorted(slices.Values(r.marks))
	types := slices.Sorted(slices.Values(r.types))
	marks = slices.Compact(marks)
	types = slices.Compact(types)
	if !slices.Equal(marks, slices.Sorted(slices.Values(wantMarks))) {
		t.Errorf("invalidated trust marks = %v, want %v", r.marks, wantMarks)
	}
	if !slices.Equal(types, slices.Sorted(slices.Values(wantTypes))) {
		t.Errorf("invalidated trust mark types = %v, want %v", r.types, wantTypes)
	}
}

// setupCacheInvalidationApp registers the issuance routes on real storage with
// a spec "type-a" holding the active subject "https://a.example.org".
func setupCacheInvalidationApp(t *testing.T) (*fiber.App, *recordingCacheInvalidator, uint, uint) {
	t.Helper()
	store := newTestStorage(t).TrustMarkSpecStorage()
	spec, err := store.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-a"})
	if err != nil {
		t.Fatalf("create spec: %v", err)
	}
	subject, err := store.CreateSubject(
		"type-a", &model.AddTrustMarkSubject{
			EntityID: "https://a.example.org",
			Status:   model.StatusActive,
		},
	)
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	inv := &recordingCacheInvalidator{}
	app := fiber.New()
	registerTrustMarkIssuance(app, store, inv)
	return app, inv, spec.ID, subject.ID
}

func TestTrustMarkIssuance_CacheInvalidation(t *testing.T) {
	t.Parallel()

	type request struct {
		method, path, body string
		contentType        string
	}
	specPath := func(specID uint) string {
		return fmt.Sprintf("/trust-marks/issuance-spec/%d", specID)
	}
	subjectPath := func(specID, subjectID uint) string {
		return fmt.Sprintf("/trust-marks/issuance-spec/%d/subjects/%d", specID, subjectID)
	}
	tests := []struct {
		name       string
		request    func(specID, subjectID uint) request
		wantStatus int
		wantMarks  []string
		wantTypes  []string
	}{
		{
			name: "CreateSubject",
			request: func(specID, _ uint) request {
				return request{
					"POST", specPath(specID) + "/subjects", `{"entity_id":"https://b.example.org"}`,
					fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusCreated,
			wantMarks:  []string{"type-a|https://b.example.org"},
		},
		{
			name: "UpdateSubject",
			request: func(specID, subjectID uint) request {
				return request{
					"PUT", subjectPath(specID, subjectID), `{"entity_id":"https://a.example.org","status":"active"}`,
					fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusOK,
			wantMarks:  []string{"type-a|https://a.example.org"},
		},
		{
			name: "UpdateSubjectEntityID",
			request: func(specID, subjectID uint) request {
				return request{
					"PUT", subjectPath(specID, subjectID), `{"entity_id":"https://c.example.org","status":"active"}`,
					fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusOK,
			wantMarks:  []string{"type-a|https://a.example.org", "type-a|https://c.example.org"},
		},
		{
			name: "ChangeSubjectStatus",
			request: func(specID, subjectID uint) request {
				return request{"PUT", subjectPath(specID, subjectID) + "/status", "blocked", fiber.MIMETextPlain}
			},
			wantStatus: http.StatusOK,
			wantMarks:  []string{"type-a|https://a.example.org"},
		},
		{
			name: "DeleteSubject",
			request: func(specID, subjectID uint) request {
				return request{"DELETE", subjectPath(specID, subjectID), "", ""}
			},
			wantStatus: http.StatusNoContent,
			wantMarks:  []string{"type-a|https://a.example.org"},
		},
		{
			name: "PutAdditionalClaims",
			request: func(specID, subjectID uint) request {
				return request{
					"PUT", subjectPath(specID, subjectID) + "/additional-claims", `{"level":"gold"}`,
					fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusOK,
			wantMarks:  []string{"type-a|https://a.example.org"},
		},
		{
			name: "CopyAdditionalClaims",
			request: func(specID, subjectID uint) request {
				return request{"POST", subjectPath(specID, subjectID) + "/additional-claims", "", ""}
			},
			wantStatus: http.StatusOK,
			wantMarks:  []string{"type-a|https://a.example.org"},
		},
		{
			name: "UpdateSpec",
			request: func(specID, _ uint) request {
				return request{"PUT", specPath(specID), `{"trust_mark_type":"type-a","cache_ttl":60}`, fiber.MIMEApplicationJSON}
			},
			wantStatus: http.StatusOK,
			wantTypes:  []string{"type-a"},
		},
		{
			name: "UpdateSpecType",
			request: func(specID, _ uint) request {
				return request{"PUT", specPath(specID), `{"trust_mark_type":"type-b"}`, fiber.MIMEApplicationJSON}
			},
			wantStatus: http.StatusOK,
			wantTypes:  []string{"type-a", "type-b"},
		},
		{
			name: "PatchSpecType",
			request: func(specID, _ uint) request {
				return request{"PATCH", specPath(specID), `{"trust_mark_type":"type-b"}`, fiber.MIMEApplicationJSON}
			},
			wantStatus: http.StatusOK,
			wantTypes:  []string{"type-a", "type-b"},
		},
		{
			name: "DeleteSpec",
			request: func(specID, _ uint) request {
				return request{"DELETE", specPath(specID), "", ""}
			},
			wantStatus: http.StatusNoContent,
			wantTypes:  []string{"type-a"},
		},
		// Failed writes invalidate nothing.
		{
			name: "CreateSubjectInvalidBody",
			request: func(specID, _ uint) request {
				return request{"POST", specPath(specID) + "/subjects", `{"entity_id":`, fiber.MIMEApplicationJSON}
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "CreateSubjectUnknownSpec",
			request: func(_, _ uint) request {
				return request{
					"POST", specPath(999) + "/subjects", `{"entity_id":"https://b.example.org"}`,
					fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "UpdateSubjectNotFound",
			request: func(specID, _ uint) request {
				return request{
					"PUT", subjectPath(specID, 999), `{"entity_id":"https://a.example.org"}`, fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "ChangeSubjectStatusInvalid",
			request: func(specID, subjectID uint) request {
				return request{"PUT", subjectPath(specID, subjectID) + "/status", "bogus", fiber.MIMETextPlain}
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "ChangeSubjectStatusNotFound",
			request: func(specID, _ uint) request {
				return request{"PUT", subjectPath(specID, 999) + "/status", "blocked", fiber.MIMETextPlain}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "DeleteSubjectNotFound",
			request: func(specID, _ uint) request {
				return request{"DELETE", subjectPath(specID, 999), "", ""}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "PutAdditionalClaimsNotFound",
			request: func(specID, _ uint) request {
				return request{
					"PUT", subjectPath(specID, 999) + "/additional-claims", `{"level":"gold"}`,
					fiber.MIMEApplicationJSON,
				}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "UpdateSpecNotFound",
			request: func(_, _ uint) request {
				return request{"PUT", specPath(999), `{"trust_mark_type":"type-a"}`, fiber.MIMEApplicationJSON}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "PatchSpecInvalidBody",
			request: func(specID, _ uint) request {
				return request{"PATCH", specPath(specID), `{`, fiber.MIMEApplicationJSON}
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "DeleteSpecNotFound",
			request: func(_, _ uint) request {
				return request{"DELETE", specPath(999), "", ""}
			},
			wantStatus: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				app, inv, specID, subjectID := setupCacheInvalidationApp(t)
				r := tt.request(specID, subjectID)
				req := httptest.NewRequest(r.method, r.path, strings.NewReader(r.body))
				if r.contentType != "" {
					req.Header.Set(fiber.HeaderContentType, r.contentType)
				}
				resp, body := doRequest(t, app, req)
				requireStatus(t, resp, body, tt.wantStatus)
				inv.assert(t, tt.wantMarks, tt.wantTypes)
			},
		)
	}
}

func TestTrustMarkIssuance_NilCacheInvalidator(t *testing.T) {
	t.Parallel()
	store := newTestStorage(t).TrustMarkSpecStorage()
	app := fiber.New()
	registerTrustMarkIssuance(app, store, nil)

	req := httptest.NewRequest(
		"POST", "/trust-marks/issuance-spec", strings.NewReader(`{"trust_mark_type":"type-a"}`),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusCreated)

	req = httptest.NewRequest(
		"POST", "/trust-marks/issuance-spec/type-a/subjects", strings.NewReader(`{"entity_id":"https://a.example.org"}`),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, body = doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusCreated)

	req = httptest.NewRequest("PUT", "/trust-marks/issuance-spec/type-a/subjects/1/status", strings.NewReader("blocked"))
	resp, body = doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusOK)

	req = httptest.NewRequest("DELETE", "/trust-marks/issuance-spec/type-a", http.NoBody)
	resp, body = doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusNoContent)
}

// TestTrustMarkSubjectStatusObjectBody covers the object form of the subject
// status change: expected_status makes it conditional, and message is
// accepted.
func TestTrustMarkSubjectStatusObjectBody(t *testing.T) {
	t.Parallel()

	put := func(t *testing.T, app *fiber.App, specID, subjectID uint, contentType, body string) (*http.Response, []byte) {
		t.Helper()
		req := httptest.NewRequest(
			"PUT", fmt.Sprintf("/trust-marks/issuance-spec/%d/subjects/%d/status", specID, subjectID),
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", contentType)
		return doRequest(t, app, req)
	}
	statusOf := func(t *testing.T, app *fiber.App, specID, subjectID uint) model.Status {
		t.Helper()
		req := httptest.NewRequest(
			"GET", fmt.Sprintf("/trust-marks/issuance-spec/%d/subjects/%d", specID, subjectID), http.NoBody,
		)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		var subject model.TrustMarkSubject
		if err := json.Unmarshal(body, &subject); err != nil {
			t.Fatalf("parse subject: %v", err)
		}
		return subject.Status
	}

	t.Run(
		"PlainBodyUnderJSONContentTypeStaysPlain", func(t *testing.T) {
			t.Parallel()
			app, inv, specID, subjectID := setupCacheInvalidationApp(t)
			resp, body := put(t, app, specID, subjectID, fiber.MIMEApplicationJSON, "blocked")
			requireStatus(t, resp, body, http.StatusOK)
			if got := statusOf(t, app, specID, subjectID); got != model.StatusBlocked {
				t.Errorf("status = %s, want blocked", got)
			}
			inv.assert(t, []string{"type-a|https://a.example.org"}, nil)
		},
	)

	t.Run(
		"ExpectedStatusAndMessage", func(t *testing.T) {
			t.Parallel()
			app, inv, specID, subjectID := setupCacheInvalidationApp(t)
			resp, body := put(
				t, app, specID, subjectID, fiber.MIMEApplicationJSON,
				`{"status":"blocked","expected_status":"active","message":"no longer a member"}`,
			)
			requireStatus(t, resp, body, http.StatusOK)
			if got := statusOf(t, app, specID, subjectID); got != model.StatusBlocked {
				t.Errorf("status = %s, want blocked", got)
			}
			inv.assert(t, []string{"type-a|https://a.example.org"}, nil)
		},
	)

	t.Run(
		"ExpectedStatusMismatchIsConflictAndChangesNothing", func(t *testing.T) {
			t.Parallel()
			app, inv, specID, subjectID := setupCacheInvalidationApp(t)
			resp, body := put(
				t, app, specID, subjectID, fiber.MIMEApplicationJSON,
				`{"status":"blocked","expected_status":"pending","message":"x"}`,
			)
			assertErrorResponse(t, resp, body, http.StatusConflict, "invalid_request")
			if got := statusOf(t, app, specID, subjectID); got != model.StatusActive {
				t.Errorf("status = %s, want active", got)
			}
			inv.assert(t, nil, nil)
		},
	)

	t.Run(
		"MessageTooLong", func(t *testing.T) {
			t.Parallel()
			app, _, specID, subjectID := setupCacheInvalidationApp(t)
			resp, body := put(
				t, app, specID, subjectID, fiber.MIMEApplicationJSON,
				fmt.Sprintf(`{"status":"blocked","message":%q}`, strings.Repeat("x", 1001)),
			)
			assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
			if got := statusOf(t, app, specID, subjectID); got != model.StatusActive {
				t.Errorf("status = %s, want active", got)
			}
		},
	)

	t.Run(
		"JWKSIsRefusedAndChangesNothing", func(t *testing.T) {
			t.Parallel()
			for name, jwks := range map[string]string{"Set": `{"keys":[]}`, "Null": `null`} {
				t.Run(
					name, func(t *testing.T) {
						t.Parallel()
						app, inv, specID, subjectID := setupCacheInvalidationApp(t)
						resp, body := put(
							t, app, specID, subjectID, fiber.MIMEApplicationJSON,
							fmt.Sprintf(`{"status":"blocked","jwks":%s}`, jwks),
						)
						assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
						if !strings.Contains(string(body), "jwks is not supported") {
							t.Errorf("error does not name jwks: %s", body)
						}
						if got := statusOf(t, app, specID, subjectID); got != model.StatusActive {
							t.Errorf("status = %s, want active", got)
						}
						inv.assert(t, nil, nil)
					},
				)
			}
		},
	)

	t.Run(
		"UnknownSubject", func(t *testing.T) {
			t.Parallel()
			app, _, specID, _ := setupCacheInvalidationApp(t)
			resp, body := put(t, app, specID, 999, fiber.MIMEApplicationJSON, `{"status":"blocked"}`)
			assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
		},
	)

	t.Run(
		"SecondChangeFromTheSameStatusIsConflict", func(t *testing.T) {
			t.Parallel()
			app, _, specID, subjectID := setupCacheInvalidationApp(t)
			resp, body := put(
				t, app, specID, subjectID, fiber.MIMEApplicationJSON, `{"status":"blocked","expected_status":"active"}`,
			)
			requireStatus(t, resp, body, http.StatusOK)
			resp, body = put(
				t, app, specID, subjectID, fiber.MIMEApplicationJSON, `{"status":"inactive","expected_status":"active"}`,
			)
			assertStatus(t, resp, body, http.StatusConflict)
			if got := statusOf(t, app, specID, subjectID); got != model.StatusBlocked {
				t.Errorf("status = %s, want blocked", got)
			}
		},
	)
}

// TestTrustMarkSubjectStatusConcurrent starts two overlapping conditional
// changes from the same status on a file-backed SQLite database: exactly one
// succeeds and the other is answered 409.
func TestTrustMarkSubjectStatusConcurrent(t *testing.T) {
	t.Parallel()
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	specs := store.TrustMarkSpecStorage()
	spec, err := specs.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-a"})
	if err != nil {
		t.Fatalf("create spec: %v", err)
	}
	app := fiber.New()
	registerTrustMarkIssuance(app, specs, nil)

	for i := range 10 {
		subject, err := specs.CreateSubject(
			"type-a", &model.AddTrustMarkSubject{
				EntityID: fmt.Sprintf("https://race%d.example.org", i), Status: model.StatusActive,
			},
		)
		if err != nil {
			t.Fatalf("create subject: %v", err)
		}
		targets := []string{"blocked", "inactive"}
		codes := make([]int, len(targets))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for j, target := range targets {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				req := httptest.NewRequest(
					"PUT", fmt.Sprintf("/trust-marks/issuance-spec/%d/subjects/%d/status", spec.ID, subject.ID),
					strings.NewReader(fmt.Sprintf(`{"status":%q,"expected_status":"active"}`, target)),
				)
				req.Header.Set("Content-Type", fiber.MIMEApplicationJSON)
				resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
				if err != nil {
					codes[j] = -1
					return
				}
				resp.Body.Close()
				codes[j] = resp.StatusCode
			}()
		}
		close(start)
		wg.Wait()

		winner := -1
		for j, code := range codes {
			switch code {
			case http.StatusOK:
				if winner != -1 {
					t.Fatalf("round %d: both changes succeeded", i)
				}
				winner = j
			case http.StatusConflict:
			default:
				t.Fatalf("round %d: unexpected status %d (codes %v)", i, code, codes)
			}
		}
		if winner == -1 {
			t.Fatalf("round %d: no change succeeded (codes %v)", i, codes)
		}
		got, _ := specs.GetSubject("type-a", fmt.Sprint(subject.ID))
		if got.Status.String() != targets[winner] {
			t.Errorf("round %d: status = %s, want %s", i, got.Status, targets[winner])
		}
	}
}
