package adminapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// newFileIssuanceEnv is setupIssuanceEnv on a file-backed SQLite database, on
// which overlapping transactions contend for the database lock.
func newFileIssuanceEnv(t *testing.T) (*issuanceEnv, *storage.Storage) {
	t.Helper()
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	return setupIssuanceEnv(t, store), store
}

type concurrentRequest struct {
	app         *fiber.App
	method, url string
	contentType string
	body        string
}

// runConcurrently starts reqs at the same time and returns their status codes.
func runConcurrently(reqs ...concurrentRequest) []int {
	codes := make([]int, len(reqs))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, r := range reqs {
		wg.Go(
			func() {
				<-start
				req := httptest.NewRequest(r.method, r.url, strings.NewReader(r.body))
				if r.contentType != "" {
					req.Header.Set(fiber.HeaderContentType, r.contentType)
				}
				resp, err := r.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
				if err != nil {
					codes[i] = -1
					return
				}
				resp.Body.Close()
				codes[i] = resp.StatusCode
			},
		)
	}
	close(start)
	wg.Wait()
	return codes
}

func requireCodesIn(t *testing.T, round int, codes []int, allowed ...int) {
	t.Helper()
	for _, code := range codes {
		ok := false
		for _, a := range allowed {
			ok = ok || code == a
		}
		if !ok {
			t.Fatalf("round %d: codes %v, want each one of %v", round, codes, allowed)
		}
	}
}

// TestTrustMarkSubjectWritesConcurrent runs overlapping writes of the routes
// that use a transaction on file-backed SQLite: they serialize, and none
// fails with a 500.
func TestTrustMarkSubjectWritesConcurrent(t *testing.T) {
	t.Parallel()
	env, _ := newFileIssuanceEnv(t)
	for i := range 10 {
		subject := env.createSubject(t, fmt.Sprintf("https://plain%d.example.org", i), model.StatusActive)
		url := fmt.Sprintf("%s/%d", env.subjectsURL(), subject.ID)

		// Unconditional plain-text status changes: both succeed, the last wins.
		codes := runConcurrently(
			concurrentRequest{env.app, "PUT", url + "/status", fiber.MIMETextPlain, "blocked"},
			concurrentRequest{env.app, "PUT", url + "/status", fiber.MIMETextPlain, "inactive"},
		)
		requireCodesIn(t, i, codes, http.StatusOK)

		// The additional-claims replacement and copy, which write without an
		// outer transaction, overlapping each other and a status change.
		codes = runConcurrently(
			concurrentRequest{env.app, "PUT", url + "/additional-claims", fiber.MIMEApplicationJSON, `{"level":"gold"}`},
			concurrentRequest{env.app, "POST", url + "/additional-claims", "", ""},
			concurrentRequest{env.app, "PUT", url + "/status", fiber.MIMETextPlain, "active"},
		)
		requireCodesIn(t, i, codes, http.StatusOK)

		// Two replacements, then a replacement and a delete.
		body := `{"entity_id":"` + subject.EntityID + `","status":"%s"}`
		codes = runConcurrently(
			concurrentRequest{env.app, "PUT", url, fiber.MIMEApplicationJSON, fmt.Sprintf(body, "active")},
			concurrentRequest{env.app, "PUT", url, fiber.MIMEApplicationJSON, fmt.Sprintf(body, "pending")},
		)
		requireCodesIn(t, i, codes, http.StatusOK)
		codes = runConcurrently(
			concurrentRequest{env.app, "PUT", url, fiber.MIMEApplicationJSON, fmt.Sprintf(body, "blocked")},
			concurrentRequest{env.app, "DELETE", url, "", ""},
		)
		requireCodesIn(t, i, codes, http.StatusOK, http.StatusNoContent, http.StatusNotFound)

		// Creates of different entities under one spec all succeed.
		codes = runConcurrently(
			concurrentRequest{
				env.app, "POST", env.subjectsURL(), fiber.MIMEApplicationJSON,
				fmt.Sprintf(`{"entity_id":"https://new%d-a.example.org"}`, i),
			},
			concurrentRequest{
				env.app, "POST", env.subjectsURL(), fiber.MIMEApplicationJSON,
				fmt.Sprintf(`{"entity_id":"https://new%d-b.example.org","issue":true}`, i),
			},
			concurrentRequest{
				env.app, "POST", env.subjectsURL(), fiber.MIMEApplicationJSON,
				fmt.Sprintf(`{"entity_id":"https://new%d-c.example.org","status":"pending"}`, i),
			},
		)
		requireCodesIn(t, i, codes, http.StatusCreated)
	}
}

// TestTrustMarkIssueTwoProcessesConcurrent simulates two LightHouse
// processes sharing one database: two handler sets with their own locks issue
// the same trust mark to the same active subject at the same time. The
// in-process lock does not serialize them; the transaction does, so exactly
// one instance is recorded and the other request is answered 409
// already_held.
func TestTrustMarkIssueTwoProcessesConcurrent(t *testing.T) {
	t.Parallel()
	env, _ := newFileIssuanceEnv(t)
	apps := make([]*fiber.App, 2)
	var barrier sync.WaitGroup
	for i := range apps {
		signer := &fakeSigner{prefix: fmt.Sprintf("p%d-", i)}
		// Both have passed the check before the transaction once both sign.
		signer.during = func() {
			barrier.Done()
			barrier.Wait()
		}
		h := &trustMarkSubjectHandlers{storages: env.backends, signer: signer, locks: &keyedMutex{}}
		apps[i] = fiber.New()
		apps[i].Put("/trust-marks/issuance-spec/:trustMarkSpecID/subjects/:trustMarkSubjectID/status", h.updateStatus)
	}
	for round := range 5 {
		entityID := fmt.Sprintf("https://twice%d.example.org", round)
		subject := env.createSubject(t, entityID, model.StatusActive)
		url := fmt.Sprintf("%s/%d/status", env.subjectsURL(), subject.ID)
		barrier.Add(len(apps))
		codes := runConcurrently(
			concurrentRequest{apps[0], "PUT", url, fiber.MIMEApplicationJSON, `{"status":"active","issue":true}`},
			concurrentRequest{apps[1], "PUT", url, fiber.MIMEApplicationJSON, `{"status":"active","issue":true}`},
		)
		ok, held := 0, 0
		for _, code := range codes {
			switch code {
			case http.StatusOK:
				ok++
			case http.StatusConflict:
				held++
			}
		}
		if ok != 1 || held != 1 {
			t.Fatalf("round %d: codes %v, want one 200 and one 409", round, codes)
		}
		live := 0
		for _, inst := range env.instances(t, entityID) {
			if !inst.Revoked {
				live++
			}
		}
		if live != 1 {
			t.Fatalf("round %d: live instances = %d, want 1", round, live)
		}
	}
}
