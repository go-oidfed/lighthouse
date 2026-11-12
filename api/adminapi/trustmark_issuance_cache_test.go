package adminapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"

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
