package adminapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

const (
	issueType   = "https://tm.example.org/a"
	issueEntity = "https://a.example.org"
)

// fakeSigner signs trust marks with sequential JTIs. err, if set, fails the
// signing; during, if set, runs while a trust mark is being signed.
type fakeSigner struct {
	mu     sync.Mutex
	n      int
	prefix string
	err    error
	during func()
	claims []map[string]any
	served []string
}

func (f *fakeSigner) ServeTrustMark(trustMarkType, sub string, signed SignedTrustMark, cacheTTL time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.served = append(f.served, fmt.Sprintf("%s|%s|%s|%s", trustMarkType, sub, signed.JTI, cacheTTL))
}

func (f *fakeSigner) SignTrustMark(trustMarkType, sub string, claims map[string]any) (SignedTrustMark, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return SignedTrustMark{}, f.err
	}
	f.n++
	f.claims = append(f.claims, maps.Clone(claims))
	return SignedTrustMark{
		JWT:       "jwt-" + trustMarkType + "-" + sub,
		JTI:       fmt.Sprintf("jti-%s%d", f.prefix, f.n),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, nil
}

type issuanceEnv struct {
	app      *fiber.App
	backends model.Backends
	signer   *fakeSigner
	inv      *recordingCacheInvalidator
	specID   uint
}

// setupIssuanceEnv registers the issuance and instance routes on real
// storage, with a spec of issueType described "Type A".
func setupIssuanceEnv(t *testing.T, store *storage.Storage) *issuanceEnv {
	t.Helper()
	backends := testBackendsOf(t, store)
	spec, err := backends.TrustMarkSpecs.Create(
		&model.AddTrustMarkSpec{
			TrustMarkType: issueType, Description: "Type A",
			AdditionalClaims: map[string]any{"level": "standard"},
		},
	)
	if err != nil {
		t.Fatalf("create spec: %v", err)
	}
	env := &issuanceEnv{
		app: fiber.New(), backends: backends, signer: &fakeSigner{}, inv: &recordingCacheInvalidator{},
		specID: spec.ID,
	}
	registerTrustMarkIssuance(env.app, backends, env.inv, env.signer)
	registerTrustMarkInstances(env.app, backends, env.inv)
	return env
}

func (e *issuanceEnv) subjectsURL() string {
	return fmt.Sprintf("/trust-marks/issuance-spec/%d/subjects", e.specID)
}

func (e *issuanceEnv) do(t *testing.T, method, url, body string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return doRequest(t, e.app, req)
}

func (e *issuanceEnv) createSubject(t *testing.T, entityID string, status model.Status) *model.TrustMarkSubject {
	t.Helper()
	subject, err := e.backends.TrustMarkSpecs.CreateSubject(
		issueType, &model.AddTrustMarkSubject{EntityID: entityID, Status: status},
	)
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	return subject
}

func (e *issuanceEnv) addInstance(t *testing.T, jti, entityID string, subjectID uint) {
	t.Helper()
	if err := e.backends.TrustMarkInstances.Create(
		&model.IssuedTrustMarkInstance{
			JTI: jti, TrustMarkType: issueType, Subject: entityID, TrustMarkSubjectID: subjectID,
			ExpiresAt: int(time.Now().Add(time.Hour).Unix()),
		},
	); err != nil {
		t.Fatalf("create instance: %v", err)
	}
}

func (e *issuanceEnv) instances(t *testing.T, entityID string) []model.IssuedTrustMarkInstance {
	t.Helper()
	instances, err := e.backends.TrustMarkInstances.ListBySubject(issueType, entityID)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	return instances
}

// events returns the events about entityID, oldest first, as "type: message".
func (e *issuanceEnv) events(t *testing.T, entityID string) []string {
	t.Helper()
	events, _, err := e.backends.SubordinateEvents.Query(model.EventQueryOpts{EntityID: &entityID})
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	out := make([]string, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		msg := ""
		if events[i].Message != nil {
			msg = *events[i].Message
		}
		out = append(out, events[i].Type+": "+msg)
	}
	return out
}

func assertCodedError(t *testing.T, resp *http.Response, body []byte, status int, code string) {
	t.Helper()
	assertErrorResponse(t, resp, body, status, code)
}

func assertStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s =\n  %q\nwant\n  %q", what, got, want)
	}
}

func TestTrustMarkIssue_OnCreate(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))

	resp, body := env.do(
		t, "POST", env.subjectsURL(),
		`{"entity_id":"`+issueEntity+`","issue":true,"message":"approved by the board"}`,
	)
	requireStatus(t, resp, body, http.StatusCreated)
	var created model.TrustMarkSubject
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Status != model.StatusActive {
		t.Errorf("status = %s, want active", created.Status)
	}

	instances := env.instances(t, issueEntity)
	if len(instances) != 1 {
		t.Fatalf("instances = %d, want 1", len(instances))
	}
	inst := instances[0]
	if inst.JTI != "jti-1" || inst.Revoked || inst.TrustMarkSubjectID != created.ID || inst.ExpiresAt == 0 {
		t.Errorf("instance = %+v, want jti-1, unrevoked, linked to subject %d, with expiry", inst, created.ID)
	}
	// Without subject claims the spec's claims are signed.
	if len(env.signer.claims) != 1 || env.signer.claims[0]["level"] != "standard" {
		t.Errorf("signed claims = %v, want the spec's", env.signer.claims)
	}
	assertStrings(
		t, "events", env.events(t, issueEntity), []string{
			"trust_mark_granted: approved by the board",
			"trust_mark_issued: approved by the board",
		},
	)
	env.inv.assert(t, []string{issueType + "|" + issueEntity}, nil)
	// The recorded mark is handed to the endpoint to serve (the spec has no
	// cache_ttl, so for 0s).
	assertStrings(t, "served", env.signer.served, []string{issueType + "|" + issueEntity + "|jti-1|0s"})
}

func TestTrustMarkIssue_OnCreateWithSubjectClaims(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))

	resp, body := env.do(
		t, "POST", env.subjectsURL(),
		`{"entity_id":"`+issueEntity+`","issue":true,"additional_claims":{"level":"premium"}}`,
	)
	requireStatus(t, resp, body, http.StatusCreated)
	if len(env.signer.claims) != 1 || env.signer.claims[0]["level"] != "premium" {
		t.Errorf("signed claims = %v, want the subject's", env.signer.claims)
	}
	// Without a message the events name the mark by its spec.
	assertStrings(
		t, "events", env.events(t, issueEntity), []string{
			"trust_mark_granted: Type A (" + issueType + ")",
			"trust_mark_issued: Type A (" + issueType + ")",
		},
	)
}

func TestTrustMarkIssue_CreateRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		setup      func(t *testing.T, env *issuanceEnv)
		wantStatus int
		wantError  string
	}{
		{
			name:       "not active",
			body:       `{"entity_id":"` + issueEntity + `","status":"pending","issue":true}`,
			wantStatus: http.StatusBadRequest, wantError: "invalid_request",
		},
		{
			name: "already held",
			body: `{"entity_id":"` + issueEntity + `","issue":true}`,
			setup: func(t *testing.T, env *issuanceEnv) {
				env.addInstance(t, "held", issueEntity, 0)
			},
			wantStatus: http.StatusConflict, wantError: "already_held",
		},
		{
			name: "issuance unavailable",
			body: `{"entity_id":"` + issueEntity + `","issue":true}`,
			setup: func(_ *testing.T, env *issuanceEnv) {
				env.signer.err = fmt.Errorf("type: %w", ErrIssuanceUnavailable)
			},
			wantStatus: http.StatusServiceUnavailable, wantError: "issuance_unavailable",
		},
		{
			name: "signing fails",
			body: `{"entity_id":"` + issueEntity + `","issue":true}`,
			setup: func(_ *testing.T, env *issuanceEnv) {
				env.signer.err = errors.New("signer broken")
			},
			wantStatus: http.StatusInternalServerError, wantError: "server_error",
		},
		{
			// Another process records a live mark while this one signs: the
			// check in the transaction refuses it and nothing is written.
			name: "held when recorded",
			body: `{"entity_id":"` + issueEntity + `","issue":true}`,
			setup: func(t *testing.T, env *issuanceEnv) {
				env.signer.during = func() { env.addInstance(t, "other-process", issueEntity, 0) }
			},
			wantStatus: http.StatusConflict, wantError: "already_held",
		},
		{
			name:       "message too long",
			body:       `{"entity_id":"` + issueEntity + `","issue":true,"message":"` + strings.Repeat("x", 1001) + `"}`,
			wantStatus: http.StatusBadRequest, wantError: "invalid_request",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				env := setupIssuanceEnv(t, newTestStorage(t))
				if tt.setup != nil {
					tt.setup(t, env)
				}

				resp, body := env.do(t, "POST", env.subjectsURL(), tt.body)
				assertCodedError(t, resp, body, tt.wantStatus, tt.wantError)

				if _, err := env.backends.TrustMarkSpecs.GetSubject(issueType, issueEntity); err == nil {
					t.Error("the subject was created")
				}
				for _, inst := range env.instances(t, issueEntity) {
					if strings.HasPrefix(inst.JTI, "jti-") {
						t.Errorf("the signed instance %s was recorded", inst.JTI)
					}
				}
				if got := env.events(t, issueEntity); len(got) != 0 {
					t.Errorf("events = %q, want none", got)
				}
			},
		)
	}
}

func TestTrustMarkIssue_NoSigner(t *testing.T) {
	t.Parallel()
	backends := testBackendsOf(t, newTestStorage(t))
	if _, err := backends.TrustMarkSpecs.Create(&model.AddTrustMarkSpec{TrustMarkType: issueType}); err != nil {
		t.Fatalf("create spec: %v", err)
	}
	app := fiber.New()
	registerTrustMarkIssuance(app, backends, nil, nil)

	req := httptest.NewRequest(
		"POST", "/trust-marks/issuance-spec/1/subjects",
		strings.NewReader(`{"entity_id":"`+issueEntity+`","issue":true}`),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, body := doRequest(t, app, req)
	assertCodedError(t, resp, body, http.StatusServiceUnavailable, "issuance_unavailable")
}

func TestTrustMarkIssue_OnActivate(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))
	subject := env.createSubject(t, issueEntity, model.StatusPending)
	url := fmt.Sprintf("%s/%d/status", env.subjectsURL(), subject.ID)

	resp, body := env.do(
		t, "PUT", url, `{"status":"active","expected_status":"pending","issue":true,"message":"checked"}`,
	)
	requireStatus(t, resp, body, http.StatusOK)
	var updated model.TrustMarkSubject
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.Status != model.StatusActive {
		t.Errorf("status = %s, want active", updated.Status)
	}
	instances := env.instances(t, issueEntity)
	if len(instances) != 1 || instances[0].TrustMarkSubjectID != subject.ID || instances[0].Revoked {
		t.Fatalf("instances = %+v, want one live instance linked to subject %d", instances, subject.ID)
	}
	assertStrings(
		t, "events", env.events(t, issueEntity), []string{"trust_mark_granted: checked", "trust_mark_issued: checked"},
	)
	env.inv.assert(t, []string{issueType + "|" + issueEntity}, nil)
}

// TestTrustMarkIssue_ActiveSubject issues to a subject that is already active
// but holds no live mark: the status stays and only the issuance is recorded.
func TestTrustMarkIssue_ActiveSubject(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))
	subject := env.createSubject(t, issueEntity, model.StatusActive)

	resp, body := env.do(
		t, "PUT", fmt.Sprintf("%s/%d/status", env.subjectsURL(), subject.ID), `{"status":"active","issue":true}`,
	)
	requireStatus(t, resp, body, http.StatusOK)
	if got := len(env.instances(t, issueEntity)); got != 1 {
		t.Errorf("instances = %d, want 1", got)
	}
	assertStrings(t, "events", env.events(t, issueEntity), []string{"trust_mark_issued: Type A (" + issueType + ")"})
}

func TestTrustMarkIssue_ActivateRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		held       bool
		wantStatus int
		wantError  string
	}{
		{
			name:       "already held",
			body:       `{"status":"active","expected_status":"pending","issue":true}`,
			held:       true,
			wantStatus: http.StatusConflict, wantError: "already_held",
		},
		{
			name:       "not active",
			body:       `{"status":"blocked","issue":true}`,
			wantStatus: http.StatusBadRequest, wantError: "invalid_request",
		},
		{
			name:       "status changed",
			body:       `{"status":"active","expected_status":"inactive","issue":true}`,
			wantStatus: http.StatusConflict, wantError: "invalid_request",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				env := setupIssuanceEnv(t, newTestStorage(t))
				subject := env.createSubject(t, issueEntity, model.StatusPending)
				if tt.held {
					env.addInstance(t, "held", issueEntity, 0)
				}
				before := len(env.instances(t, issueEntity))

				resp, body := env.do(t, "PUT", fmt.Sprintf("%s/%d/status", env.subjectsURL(), subject.ID), tt.body)
				assertCodedError(t, resp, body, tt.wantStatus, tt.wantError)

				got, err := env.backends.TrustMarkSpecs.GetSubject(issueType, issueEntity)
				if err != nil || got.Status != model.StatusPending {
					t.Errorf("subject = %+v, %v; want it still pending", got, err)
				}
				if n := len(env.instances(t, issueEntity)); n != before {
					t.Errorf("instances = %d, want %d", n, before)
				}
				if events := env.events(t, issueEntity); len(events) != 0 {
					t.Errorf("events = %q, want none", events)
				}
			},
		)
	}
}

// TestTrustMarkIssue_ActivateRolledBack makes the issuance fail in the
// transaction that activates the subject: the status change is rolled back
// with it.
func TestTrustMarkIssue_ActivateRolledBack(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))
	subject := env.createSubject(t, issueEntity, model.StatusPending)
	env.signer.during = func() { env.addInstance(t, "other-process", issueEntity, 0) }

	resp, body := env.do(
		t, "PUT", fmt.Sprintf("%s/%d/status", env.subjectsURL(), subject.ID),
		`{"status":"active","expected_status":"pending","issue":true}`,
	)
	assertCodedError(t, resp, body, http.StatusConflict, "already_held")
	got, err := env.backends.TrustMarkSpecs.GetSubject(issueType, issueEntity)
	if err != nil || got.Status != model.StatusPending {
		t.Errorf("subject = %+v, %v; want it still pending", got, err)
	}
	if events := env.events(t, issueEntity); len(events) != 0 {
		t.Errorf("events = %q, want none", events)
	}
}

// TestTrustMarkIssueConcurrent starts two overlapping activations with issue
// of the same subject on a file-backed SQLite database: exactly one issues and
// the other is answered 409, and one live instance is recorded.
func TestTrustMarkIssueConcurrent(t *testing.T) {
	t.Parallel()
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	env := setupIssuanceEnv(t, store)
	for i := range 5 {
		entityID := fmt.Sprintf("https://race%d.example.org", i)
		subject := env.createSubject(t, entityID, model.StatusPending)
		url := fmt.Sprintf("%s/%d/status", env.subjectsURL(), subject.ID)
		codes := make([]int, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for j := range codes {
			wg.Go(
				func() {
					<-start
					req := httptest.NewRequest(
						"PUT", url, strings.NewReader(`{"status":"active","expected_status":"pending","issue":true}`),
					)
					req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
					resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
					if err != nil {
						codes[j] = -1
						return
					}
					resp.Body.Close()
					codes[j] = resp.StatusCode
				},
			)
		}
		close(start)
		wg.Wait()
		ok, conflict := 0, 0
		for _, code := range codes {
			switch code {
			case http.StatusOK:
				ok++
			case http.StatusConflict:
				conflict++
			}
		}
		if ok != 1 || conflict != 1 {
			t.Fatalf("round %d: codes %v, want one 200 and one 409", i, codes)
		}
		if n := len(env.instances(t, entityID)); n != 1 {
			t.Fatalf("round %d: instances = %d, want 1", i, n)
		}
	}
}

func TestTrustMarkInstances_ListAndCounts(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))
	now := time.Now()
	for _, inst := range []model.IssuedTrustMarkInstance{
		{JTI: "a-live", TrustMarkType: issueType, Subject: "https://one.example.org", ExpiresAt: int(now.Add(time.Hour).Unix())},
		{JTI: "a-forever", TrustMarkType: issueType, Subject: "https://two.example.org"},
		{JTI: "a-expired", TrustMarkType: issueType, Subject: "https://two.example.org", ExpiresAt: int(now.Add(-time.Hour).Unix())},
		{JTI: "b-revoked", TrustMarkType: "https://tm.example.org/b", Subject: "https://one.example.org", Revoked: true},
	} {
		if err := env.backends.TrustMarkInstances.Create(&inst); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	type listResult struct {
		Instances []struct {
			JTI           string `json:"jti"`
			TrustMarkType string `json:"trust_mark_type"`
			Subject       string `json:"subject"`
			IssuedAt      int64  `json:"issued_at"`
			ExpiresAt     int64  `json:"expires_at"`
			State         string `json:"state"`
		} `json:"instances"`
		Pagination struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"pagination"`
	}
	list := func(query string) listResult {
		t.Helper()
		resp, body := doRequest(t, env.app, httptest.NewRequest("GET", "/trust-marks/instances"+query, http.NoBody))
		requireStatus(t, resp, body, http.StatusOK)
		var res listResult
		if err := json.Unmarshal(body, &res); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return res
	}
	states := func(res listResult) []string {
		var out []string
		for _, inst := range res.Instances {
			out = append(out, inst.JTI+"="+inst.State)
		}
		return out
	}

	all := list("")
	if all.Pagination.Total != 4 || all.Pagination.Limit != 50 || len(all.Instances) != 4 {
		t.Errorf("list = %+v, want 4 instances, limit 50", all)
	}
	got := states(all)
	for _, want := range []string{"a-live=active", "a-forever=active", "a-expired=expired", "b-revoked=revoked"} {
		if !strings.Contains(strings.Join(got, " "), want) {
			t.Errorf("states %v lack %s", got, want)
		}
	}
	assertStrings(t, "by type and state", states(list("?trust_mark_type="+issueType+"&state=expired")), []string{"a-expired=expired"})
	assertStrings(t, "by state", states(list("?state=revoked")), []string{"b-revoked=revoked"})
	if res := list("?sub=https://one.example.org"); len(res.Instances) != 2 {
		t.Errorf("by sub = %v, want 2", states(res))
	}
	if res := list("?limit=1&offset=1"); len(res.Instances) != 1 || res.Pagination.Total != 4 || res.Pagination.Offset != 1 {
		t.Errorf("page = %+v, want 1 of 4 at offset 1", res)
	}
	if res := list("?limit=1000"); res.Pagination.Limit != 100 {
		t.Errorf("limit = %d, want capped at 100", res.Pagination.Limit)
	}
	for _, bad := range []string{"?state=live", "?limit=0", "?limit=x", "?offset=-1"} {
		resp, body := doRequest(t, env.app, httptest.NewRequest("GET", "/trust-marks/instances"+bad, http.NoBody))
		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	}

	resp, body := doRequest(t, env.app, httptest.NewRequest("GET", "/trust-marks/instances/counts", http.NoBody))
	requireStatus(t, resp, body, http.StatusOK)
	if string(body) != `{"active":2,"revoked":1}` {
		t.Errorf("counts = %s, want active 2, revoked 1", body)
	}
}

func TestTrustMarkInstances_Revoke(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))
	const other = "https://not-a-subject.example.org"
	env.addInstance(t, "x1", other, 0)
	env.addInstance(t, "x2", other, 0)

	resp, body := env.do(
		t, "POST", "/trust-marks/instances/revoke",
		`{"trust_mark_type":"`+issueType+`","sub":"`+other+`","message":"key compromise"}`,
	)
	requireStatus(t, resp, body, http.StatusOK)
	if string(body) != `{"revoked":2}` {
		t.Errorf("body = %s, want revoked 2", body)
	}
	for _, inst := range env.instances(t, other) {
		if !inst.Revoked {
			t.Errorf("instance %s not revoked", inst.JTI)
		}
	}
	assertStrings(t, "events", env.events(t, other), []string{"trust_mark_revoked: key compromise"})
	env.inv.assert(t, []string{issueType + "|" + other}, nil)

	// Nothing left to revoke: no event.
	resp, body = env.do(t, "POST", "/trust-marks/instances/revoke", `{"trust_mark_type":"`+issueType+`","sub":"`+other+`"}`)
	requireStatus(t, resp, body, http.StatusOK)
	if string(body) != `{"revoked":0}` {
		t.Errorf("body = %s, want revoked 0", body)
	}
	if n := len(env.events(t, other)); n != 1 {
		t.Errorf("events = %d, want 1", n)
	}

	for _, bad := range []string{
		`{"sub":"` + other + `"}`,
		`{"trust_mark_type":"` + issueType + `"}`,
		`{"trust_mark_type":"` + issueType + `","sub":"` + other + `","message":"` + strings.Repeat("x", 1001) + `"}`,
	} {
		resp, body = env.do(t, "POST", "/trust-marks/instances/revoke", bad)
		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	}
}

func TestTrustMarkInstances_RevokeActiveSubject(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))
	subject := env.createSubject(t, issueEntity, model.StatusActive)
	env.addInstance(t, "live", issueEntity, subject.ID)

	resp, body := env.do(
		t, "POST", "/trust-marks/instances/revoke", `{"trust_mark_type":"`+issueType+`","sub":"`+issueEntity+`"}`,
	)
	assertCodedError(t, resp, body, http.StatusConflict, "subject_active")
	if inst := env.instances(t, issueEntity); len(inst) != 1 || inst[0].Revoked {
		t.Errorf("instances = %+v, want the live one unrevoked", inst)
	}
	if events := env.events(t, issueEntity); len(events) != 0 {
		t.Errorf("events = %q, want none", events)
	}
}

// TestTrustMarkSubject_LeavingActiveRevokes checks that every way a subject
// leaves active revokes its issued instances (linked to it, and unlinked ones
// of the type issued to its entity) and records trust_mark_revoked.
func TestTrustMarkSubject_LeavingActiveRevokes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		wantEvent   string
	}{
		{
			name: "conditional status change", method: "PUT", path: "/status",
			contentType: fiber.MIMEApplicationJSON,
			body:        `{"status":"blocked","expected_status":"active","message":"terms violated"}`,
			wantEvent:   "trust_mark_revoked: terms violated",
		},
		{
			name: "status change", method: "PUT", path: "/status",
			contentType: fiber.MIMEApplicationJSON, body: `{"status":"pending","message":"re-review"}`,
			wantEvent: "trust_mark_revoked: re-review",
		},
		{
			name: "plain text status change", method: "PUT", path: "/status",
			contentType: fiber.MIMETextPlain, body: "inactive",
			wantEvent: "trust_mark_revoked: Type A (" + issueType + ")",
		},
		{
			name: "update", method: "PUT", contentType: fiber.MIMEApplicationJSON,
			body:      `{"entity_id":"` + issueEntity + `","status":"blocked"}`,
			wantEvent: "trust_mark_revoked: Type A (" + issueType + ")",
		},
		{
			name: "delete", method: "DELETE",
			wantEvent: "trust_mark_revoked: Type A (" + issueType + ")",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				env := setupIssuanceEnv(t, newTestStorage(t))
				subject := env.createSubject(t, issueEntity, model.StatusActive)
				env.addInstance(t, "linked", issueEntity, subject.ID)
				env.addInstance(t, "unlinked", issueEntity, 0)

				req := httptest.NewRequest(
					tt.method, fmt.Sprintf("%s/%d%s", env.subjectsURL(), subject.ID, tt.path),
					strings.NewReader(tt.body),
				)
				if tt.contentType != "" {
					req.Header.Set(fiber.HeaderContentType, tt.contentType)
				}
				resp, body := doRequest(t, env.app, req)
				assertStatusOneOf(t, resp, http.StatusOK, http.StatusNoContent)
				if resp.StatusCode >= 300 {
					t.Fatalf("body: %s", body)
				}
				for _, inst := range env.instances(t, issueEntity) {
					if !inst.Revoked {
						t.Errorf("instance %s not revoked", inst.JTI)
					}
				}
				assertStrings(t, "events", env.events(t, issueEntity), []string{tt.wantEvent})
			},
		)
	}
}

// TestTrustMarkSubject_EventMessages checks that a message replaces the text
// of the event a subject create or status change records.
func TestTrustMarkSubject_EventMessages(t *testing.T) {
	t.Parallel()
	env := setupIssuanceEnv(t, newTestStorage(t))

	resp, body := env.do(t, "POST", env.subjectsURL(), `{"entity_id":"`+issueEntity+`","message":"founding member"}`)
	requireStatus(t, resp, body, http.StatusCreated)
	var created model.TrustMarkSubject
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// A subject created pending is not granted the mark: no event.
	resp, body = env.do(
		t, "POST", env.subjectsURL(), `{"entity_id":"https://b.example.org","status":"pending","message":"x"}`,
	)
	requireStatus(t, resp, body, http.StatusCreated)
	var pending model.TrustMarkSubject
	if err := json.Unmarshal(body, &pending); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	resp, body = env.do(
		t, "PUT", fmt.Sprintf("%s/%d/status", env.subjectsURL(), pending.ID),
		`{"status":"blocked","expected_status":"pending","message":"denied: no contract"}`,
	)
	requireStatus(t, resp, body, http.StatusOK)

	assertStrings(t, "events", env.events(t, issueEntity), []string{"trust_mark_granted: founding member"})
	assertStrings(
		t, "events", env.events(t, "https://b.example.org"), []string{"trust_mark_status_updated: denied: no contract"},
	)
	if got := len(env.instances(t, issueEntity)); got != 0 {
		t.Errorf("instances = %d, want none without issue", got)
	}
}
