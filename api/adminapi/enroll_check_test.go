package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

type fakeEnrollChecker struct {
	res       EnrollCheckResult
	err       error
	calls     int
	gotEntity string
	gotTypes  []string
	ctxGiven  bool
}

func (f *fakeEnrollChecker) Configured() bool { return f.res.Configured }

func (f *fakeEnrollChecker) CheckEnroll(
	ctx context.Context, entityID string, entityTypes []string,
) (EnrollCheckResult, error) {
	f.calls++
	f.gotEntity, f.gotTypes = entityID, entityTypes
	f.ctxGiven = ctx != nil
	return f.res, f.err
}

// setupEnrollCheckApp registers the base subordinate routes and the enroll
// check with checker, and adds a subordinate with the given entity types.
func setupEnrollCheckApp(t *testing.T, checker EnrollChecker, types ...string) (*fiber.App, string) {
	t.Helper()
	app, b := setupSubordinateBaseApp(t)
	registerEnrollCheck(app, b.Subordinates, checker)
	info := model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID: "https://sub.example.org", Status: model.StatusPending,
		},
	}
	for _, et := range types {
		info.SubordinateEntityTypes = append(info.SubordinateEntityTypes, model.SubordinateEntityType{EntityType: et})
	}
	if err := b.Subordinates.Add(info); err != nil {
		t.Fatalf("add subordinate: %v", err)
	}
	stored, err := b.Subordinates.Get("https://sub.example.org")
	if err != nil || stored == nil {
		t.Fatalf("get subordinate: %v", err)
	}
	return app, strconv.Itoa(int(stored.ID))
}

func postEnrollCheck(t *testing.T, app *fiber.App, id string) (*http.Response, []byte) {
	t.Helper()
	return doRequest(t, app, httptest.NewRequest("POST", "/subordinates/"+id+"/enroll-check", http.NoBody))
}

func decodeEnrollCheck(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("parse %s: %v", body, err)
	}
	return m
}

func TestEnrollCheck_NoCheckerHook(t *testing.T) {
	t.Parallel()
	app, id := setupEnrollCheckApp(t, nil, "openid_provider")
	resp, body := postEnrollCheck(t, app, id)
	requireStatus(t, resp, body, http.StatusOK)
	m := decodeEnrollCheck(t, body)
	if m["configured"] != false || m["ok"] != false {
		t.Fatalf("unexpected answer: %s", body)
	}
	if _, has := m["reason"]; has {
		t.Fatalf("reason must be omitted: %s", body)
	}
	if got := m["entity_types"].([]any); len(got) != 1 || got[0] != "openid_provider" {
		t.Fatalf("unexpected entity_types: %s", body)
	}
}

func TestEnrollCheck_NotConfigured(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{res: EnrollCheckResult{EntityTypes: nil}}
	app, id := setupEnrollCheckApp(t, f)
	resp, body := postEnrollCheck(t, app, id)
	requireStatus(t, resp, body, http.StatusOK)
	m := decodeEnrollCheck(t, body)
	if m["configured"] != false || m["ok"] != false {
		t.Fatalf("unexpected answer: %s", body)
	}
	if got, ok := m["entity_types"].([]any); !ok || len(got) != 0 {
		t.Fatalf("entity_types must be an empty list: %s", body)
	}
}

func TestEnrollCheck_Passing(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{res: EnrollCheckResult{Configured: true, OK: true, EntityTypes: []string{"openid_provider"}}}
	app, id := setupEnrollCheckApp(t, f, "openid_provider")
	resp, body := postEnrollCheck(t, app, id)
	requireStatus(t, resp, body, http.StatusOK)
	m := decodeEnrollCheck(t, body)
	if m["configured"] != true || m["ok"] != true {
		t.Fatalf("unexpected answer: %s", body)
	}
	if _, has := m["reason"]; has {
		t.Fatalf("reason must be omitted: %s", body)
	}
	// The entity ID and types come from the stored subordinate; the request
	// carries nothing.
	if f.calls != 1 || f.gotEntity != "https://sub.example.org" ||
		len(f.gotTypes) != 1 || f.gotTypes[0] != "openid_provider" || !f.ctxGiven {
		t.Fatalf("unexpected call: %+v", f)
	}
}

func TestEnrollCheck_FailingWithReason(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{res: EnrollCheckResult{Configured: true, Reason: "no trust mark", EntityTypes: []string{"x"}}}
	app, id := setupEnrollCheckApp(t, f)
	resp, body := postEnrollCheck(t, app, id)
	requireStatus(t, resp, body, http.StatusOK)
	m := decodeEnrollCheck(t, body)
	if m["configured"] != true || m["ok"] != false || m["reason"] != "no trust mark" {
		t.Fatalf("unexpected answer: %s", body)
	}
}

func TestEnrollCheck_UnknownSubordinate(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{}
	app, _ := setupEnrollCheckApp(t, f)
	resp, body := postEnrollCheck(t, app, "9999")
	assertErrorResponse(t, resp, body, http.StatusNotFound, "not_found")
	if f.calls != 0 {
		t.Fatalf("checker must not run for an unknown subordinate")
	}
}

func TestEnrollCheck_CheckerError(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{err: errors.New("boom")}
	app, id := setupEnrollCheckApp(t, f)
	resp, body := postEnrollCheck(t, app, id)
	assertStatus(t, resp, body, http.StatusInternalServerError)
}

func TestEnrollCheck_IgnoresRequestBody(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{res: EnrollCheckResult{Configured: true, OK: true}}
	app, id := setupEnrollCheckApp(t, f, "openid_provider")
	req := httptest.NewRequest(
		"POST", "/subordinates/"+id+"/enroll-check",
		strings.NewReader(`{"entity_id":"https://evil.example","entity_configuration":"x","entity_types":["a"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusOK)
	if f.gotEntity != "https://sub.example.org" || len(f.gotTypes) != 1 || f.gotTypes[0] != "openid_provider" {
		t.Fatalf("client input reached the checker: %+v", f)
	}
}

func TestEnrollCheck_Busy(t *testing.T) {
	t.Parallel()
	f := &fakeEnrollChecker{err: ErrEnrollCheckBusy}
	app, id := setupEnrollCheckApp(t, f)
	resp, body := postEnrollCheck(t, app, id)
	assertErrorResponse(t, resp, body, http.StatusServiceUnavailable, "enroll_check_busy")
}
