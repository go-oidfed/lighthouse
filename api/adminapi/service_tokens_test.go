package adminapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// testServiceSecret is a 40-byte service token value.
const testServiceSecret = "svc-token-0123456789abcdef0123456789abcd"

// otherServiceSecret is a second, different 40-byte service token value.
const otherServiceSecret = "svc-token-other-456789abcdef0123456789ab"

// testSubordinateID is the entity ID of the subordinate the tests change.
const testSubordinateID = "https://member.example.org"

// writeServiceTokenFile writes content to a file of its own and returns its path.
func writeServiceTokenFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to write token file: %v", err)
	}
	return path
}

// mustServiceToken loads a service token with secret as its value.
func mustServiceToken(t *testing.T, name, secret string, actForUsers bool) ServiceToken {
	t.Helper()
	tok, err := LoadServiceToken(name, writeServiceTokenFile(t, secret+"\n"), actForUsers)
	if err != nil {
		t.Fatalf("LoadServiceToken: %v", err)
	}
	return tok
}

// newServiceTokenTestApp builds the admin API through Register on SQLite
// storage. With serviceListener the app is the service listener.
func newServiceTokenTestApp(
	t *testing.T, serviceListener bool, tokens ...ServiceToken,
) (*fiber.App, model.Backends) {
	t.Helper()
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	app := fiber.New()
	if err := Register(
		app, "https://ta.example.org", backends, nil, KeyManagement{}, nil,
		&Options{UsersEnabled: true, ServiceTokens: tokens, ServiceListener: serviceListener},
	); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return app, backends
}

func createTestUser(t *testing.T, backends model.Backends, username string, disabled bool) {
	t.Helper()
	if _, err := backends.Users.Create(username, "secret-password", ""); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	if disabled {
		if _, err := backends.Users.Update(username, nil, nil, &disabled); err != nil {
			t.Fatalf("Failed to disable user: %v", err)
		}
	}
}

// addPendingSubordinate stores a pending subordinate and returns its database ID.
func addPendingSubordinate(t *testing.T, backends model.Backends) uint {
	t.Helper()
	if err := backends.Subordinates.Add(
		model.ExtendedSubordinateInfo{
			BasicSubordinateInfo: model.BasicSubordinateInfo{
				EntityID: testSubordinateID, Status: model.StatusPending,
			},
		},
	); err != nil {
		t.Fatalf("Failed to add subordinate: %v", err)
	}
	saved, err := backends.Subordinates.Get(testSubordinateID)
	if err != nil || saved == nil {
		t.Fatalf("Failed to get subordinate: %v", err)
	}
	return saved.ID
}

// putStatus blocks the subordinate with the given Authorization header,
// naming actor in X-Actor when it is not empty.
func putStatus(t *testing.T, app *fiber.App, id uint, authorization, actor string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(
		http.MethodPut, fmt.Sprintf("/subordinates/%d/status", id), strings.NewReader("blocked"),
	)
	req.Header.Set("Content-Type", "text/plain")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if actor != "" {
		req.Header.Set("X-Actor", actor)
	}
	return doRequest(t, app, req)
}

// statusEvents returns the status_updated events of the subordinate.
func statusEvents(t *testing.T, backends model.Backends, id uint) []model.SubordinateEvent {
	t.Helper()
	events, _, err := backends.SubordinateEvents.GetBySubordinateID(id, model.EventQueryOpts{})
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}
	var out []model.SubordinateEvent
	for _, e := range events {
		if e.Type == model.EventTypeStatusUpdated {
			out = append(out, e)
		}
	}
	return out
}

// eventActor returns the actor of an event, "" when it has none.
func eventActor(e model.SubordinateEvent) string {
	if e.Actor == nil {
		return ""
	}
	return *e.Actor
}

// requireStatusActor checks that the subordinate has one status_updated
// event, recorded with actor.
func requireStatusActor(t *testing.T, backends model.Backends, id uint, actor string) {
	t.Helper()
	events := statusEvents(t, backends, id)
	if len(events) != 1 {
		t.Fatalf("Expected 1 status event, got %d", len(events))
	}
	if got := eventActor(events[0]); got != actor {
		t.Errorf("Expected actor %q, got %q", actor, got)
	}
}

// requireNothingWritten checks that the subordinate is still pending and has
// no status event.
func requireNothingWritten(t *testing.T, backends model.Backends, id uint) {
	t.Helper()
	if n := len(statusEvents(t, backends, id)); n != 0 {
		t.Errorf("Expected no status event, got %d", n)
	}
	sub, err := backends.Subordinates.Get(testSubordinateID)
	if err != nil {
		t.Fatalf("Failed to get subordinate: %v", err)
	}
	if sub.Status != model.StatusPending {
		t.Errorf("Expected status pending, got %s", sub.Status)
	}
}

func getUsers(t *testing.T, app *fiber.App, authorization string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return doRequest(t, app, req)
}

func bearer(secret string) string {
	return "Bearer " + secret
}

func TestLoadServiceToken(t *testing.T) {
	t.Parallel()

	t.Run("ShortTokenRefusedWithoutEchoingIt", func(t *testing.T) {
		t.Parallel()
		short := strings.Repeat("a", 31)
		_, err := LoadServiceToken("svc", writeServiceTokenFile(t, short), true)
		if err == nil {
			t.Fatal("Expected an error for a 31-byte token")
		}
		if strings.Contains(err.Error(), short) {
			t.Errorf("Error echoes the token: %v", err)
		}
		// 31 bytes and a newline are still too short.
		if _, err := LoadServiceToken("svc", writeServiceTokenFile(t, short+"\n"), true); err == nil {
			t.Error("Expected an error for a 31-byte token followed by a newline")
		}
	})

	t.Run("ThirtyTwoBytesAccepted", func(t *testing.T) {
		t.Parallel()
		if _, err := LoadServiceToken("svc", writeServiceTokenFile(t, strings.Repeat("a", 32)), true); err != nil {
			t.Errorf("Expected a 32-byte token to load, got %v", err)
		}
	})

	t.Run("MissingFileRefused", func(t *testing.T) {
		t.Parallel()
		if _, err := LoadServiceToken("svc", filepath.Join(t.TempDir(), "missing"), true); err == nil {
			t.Error("Expected an error for a missing file")
		}
	})

	t.Run("EmptyNameRefused", func(t *testing.T) {
		t.Parallel()
		if _, err := LoadServiceToken("", writeServiceTokenFile(t, testServiceSecret), true); err == nil {
			t.Error("Expected an error for an empty name")
		}
	})

	t.Run("TrailingNewlineRemoved", func(t *testing.T) {
		t.Parallel()
		tok, err := LoadServiceToken("svc", writeServiceTokenFile(t, testServiceSecret+"\r\n"), true)
		if err != nil {
			t.Fatalf("LoadServiceToken: %v", err)
		}
		if !tok.matches([]byte(testServiceSecret)) {
			t.Error("Expected the token without its trailing newline to match")
		}
		if tok.matches([]byte(testServiceSecret + "\n")) {
			t.Error("Expected the token with a newline not to match")
		}
		if tok.Name != "svc" || !tok.ActForUsers {
			t.Errorf("Unexpected token fields: %+v", tok)
		}
	})

	t.Run("OnlyTheHashIsKept", func(t *testing.T) {
		t.Parallel()
		tok := mustServiceToken(t, "svc", testServiceSecret, false)
		if strings.Contains(fmt.Sprintf("%+v %#v", tok, tok), testServiceSecret) {
			t.Error("The token value is kept in the ServiceToken")
		}
	})
}

func TestServiceTokens_ActForUsers(t *testing.T) {
	t.Parallel()
	plain := mustServiceToken(t, "automation", testServiceSecret, false)
	acting := mustServiceToken(t, "operator-ui", otherServiceSecret, true)
	if (ServiceTokens{}).ActForUsers() {
		t.Error("No tokens cannot act for users")
	}
	if (ServiceTokens{plain}).ActForUsers() {
		t.Error("A token without act_for_users cannot act for users")
	}
	if !(ServiceTokens{plain, acting}).ActForUsers() {
		t.Error("Expected ActForUsers with an act_for_users token")
	}
}

func TestRegister_ServiceListenerRefusesBadTokenSets(t *testing.T) {
	t.Parallel()
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	a := mustServiceToken(t, "a", testServiceSecret, false)
	sameName := mustServiceToken(t, "a", otherServiceSecret, false)
	sameValue := mustServiceToken(t, "b", testServiceSecret, false)
	for name, tokens := range map[string]ServiceTokens{
		"NoTokens":      nil,
		"DuplicateName": {a, sameName},
		"DuplicateHash": {a, sameValue},
	} {
		err := Register(
			fiber.New(), "https://ta.example.org", backends, nil, KeyManagement{}, nil,
			&Options{ServiceTokens: tokens, ServiceListener: true},
		)
		if err == nil {
			t.Errorf("%s: expected Register to fail", name)
		} else if strings.Contains(err.Error(), testServiceSecret) {
			t.Errorf("%s: error echoes the token: %v", name, err)
		}
	}
}

func TestServiceListener_AcceptsBearerToken(t *testing.T) {
	t.Parallel()
	for _, scheme := range []string{"Bearer ", "bearer ", "BEARER "} {
		t.Run(strings.TrimSpace(scheme), func(t *testing.T) {
			t.Parallel()
			app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "svc", testServiceSecret, false))
			createTestUser(t, backends, "alice", false)
			resp, body := getUsers(t, app, scheme+testServiceSecret)
			requireStatus(t, resp, body, http.StatusOK)
		})
	}
}

func TestServiceListener_AcceptsEachConfiguredToken(t *testing.T) {
	t.Parallel()
	app, backends := newServiceTokenTestApp(t, true,
		mustServiceToken(t, "first", testServiceSecret, false),
		mustServiceToken(t, "second", otherServiceSecret, false),
	)
	id := addPendingSubordinate(t, backends)
	resp, body := putStatus(t, app, id, bearer(otherServiceSecret), "")
	requireStatus(t, resp, body, http.StatusOK)
	requireStatusActor(t, backends, id, "second")
}

func TestServiceListener_RefusesBasic(t *testing.T) {
	t.Parallel()
	for name, withUsers := range map[string]bool{"WithUsers": true, "NoUsers": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "svc", testServiceSecret, false))
			if withUsers {
				createTestUser(t, backends, "alice", false)
			}
			resp, body := getUsers(t, app, basicAuthHeader("alice", "secret-password"))
			assertErrorResponse(t, resp, body, http.StatusUnauthorized, "invalid_client")
			if got := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
				t.Errorf("Expected a Bearer challenge, got %q", got)
			}
		})
	}
}

func TestServiceListener_RefusesMissingOrUnknownToken(t *testing.T) {
	t.Parallel()
	unknown := strings.Repeat("x", 40)
	for name, authorization := range map[string]string{
		"Missing":     "",
		"Unknown":     bearer(unknown),
		"EmptyBearer": "Bearer ",
		"Prefix":      bearer(testServiceSecret[:32]),
		"Longer":      bearer(testServiceSecret + "x"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// No users: the service listener still requires a token.
			app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "svc", testServiceSecret, true))
			id := addPendingSubordinate(t, backends)
			resp, body := putStatus(t, app, id, authorization, "")
			assertErrorResponse(t, resp, body, http.StatusUnauthorized, "invalid_client")
			if strings.Contains(string(body), unknown) || strings.Contains(string(body), testServiceSecret[:32]) {
				t.Errorf("Response echoes the token: %s", body)
			}
			requireNothingWritten(t, backends, id)
		})
	}
}

func TestAdminPort_RefusesBearer(t *testing.T) {
	t.Parallel()
	app, backends := newServiceTokenTestApp(t, false, mustServiceToken(t, "svc", testServiceSecret, true))
	createTestUser(t, backends, "alice", false)
	id := addPendingSubordinate(t, backends)
	resp, body := putStatus(t, app, id, bearer(testServiceSecret), "alice")
	assertErrorResponse(t, resp, body, http.StatusUnauthorized, "invalid_client")
	requireNothingWritten(t, backends, id)
}

func TestAdminPort_BasicKeepsWorking(t *testing.T) {
	t.Parallel()
	app, backends := newServiceTokenTestApp(t, false, mustServiceToken(t, "svc", testServiceSecret, true))
	createTestUser(t, backends, "alice", false)
	id := addPendingSubordinate(t, backends)
	resp, body := putStatus(t, app, id, basicAuthHeader("alice", "secret-password"), "")
	requireStatus(t, resp, body, http.StatusOK)
	requireStatusActor(t, backends, id, "alice")
}

func TestAdminPort_NoUsersGivesATokenNoIdentity(t *testing.T) {
	t.Parallel()
	// While there are no users the admin port requires no authentication; a
	// bearer token there is not checked and does not name the actor.
	app, backends := newServiceTokenTestApp(t, false, mustServiceToken(t, "svc", testServiceSecret, true))
	id := addPendingSubordinate(t, backends)
	resp, body := putStatus(t, app, id, bearer(testServiceSecret), "")
	requireStatus(t, resp, body, http.StatusOK)
	requireStatusActor(t, backends, id, "")
}

func TestActorDelegation(t *testing.T) {
	t.Parallel()

	t.Run("EnabledUserIsTheActor", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
		createTestUser(t, backends, "alice", false)
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "alice")
		requireStatus(t, resp, body, http.StatusOK)
		requireStatusActor(t, backends, id, "alice")
	})

	t.Run("WithoutHeaderTheTokenIsTheActor", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
		createTestUser(t, backends, "alice", false)
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "")
		requireStatus(t, resp, body, http.StatusOK)
		requireStatusActor(t, backends, id, "operator-ui")
	})

	t.Run("UnknownUserRefused", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
		createTestUser(t, backends, "alice", false)
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "mallory")
		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_actor")
		requireNothingWritten(t, backends, id)
	})

	t.Run("DisabledUserRefused", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
		createTestUser(t, backends, "alice", false)
		createTestUser(t, backends, "bob", true)
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "bob")
		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_actor")
		requireNothingWritten(t, backends, id)
	})

	t.Run("TokenWithoutActForUsersIgnoresHeader", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "automation", testServiceSecret, false))
		createTestUser(t, backends, "alice", false)
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "alice")
		requireStatus(t, resp, body, http.StatusOK)
		requireStatusActor(t, backends, id, "automation")
	})

	t.Run("TokenWithoutActForUsersIgnoresUnknownName", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "automation", testServiceSecret, false))
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "mallory")
		requireStatus(t, resp, body, http.StatusOK)
		requireStatusActor(t, backends, id, "automation")
	})

	t.Run("NoUsersRefusesAnyName", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
		id := addPendingSubordinate(t, backends)
		for _, name := range []string{"alice", "operator-ui", "admin"} {
			resp, body := putStatus(t, app, id, bearer(testServiceSecret), name)
			assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_actor")
		}
		requireNothingWritten(t, backends, id)
	})

	t.Run("NoUsersWithoutHeaderTheTokenIsTheActor", func(t *testing.T) {
		t.Parallel()
		app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
		id := addPendingSubordinate(t, backends)
		resp, body := putStatus(t, app, id, bearer(testServiceSecret), "")
		requireStatus(t, resp, body, http.StatusOK)
		requireStatusActor(t, backends, id, "operator-ui")
	})

	t.Run("ConfiguredActorHeaderNotUsedOnServiceListener", func(t *testing.T) {
		t.Parallel()
		backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
		if err != nil {
			t.Fatalf("Failed to create backends: %v", err)
		}
		app := fiber.New()
		if err := Register(
			app, "https://ta.example.org", backends, nil, KeyManagement{}, nil,
			&Options{
				ServiceTokens:   ServiceTokens{mustServiceToken(t, "automation", testServiceSecret, false)},
				ServiceListener: true,
				Actor:           ActorConfig{Header: "X-Proxy-User", Source: ActorSourceHeader},
			},
		); err != nil {
			t.Fatalf("Register: %v", err)
		}
		id := addPendingSubordinate(t, backends)
		req := httptest.NewRequest(
			http.MethodPut, fmt.Sprintf("/subordinates/%d/status", id), strings.NewReader("blocked"),
		)
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Authorization", bearer(testServiceSecret))
		req.Header.Set("X-Proxy-User", "mallory")
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		requireStatusActor(t, backends, id, "automation")
	})
}

// namesCaseInsensitively is a user store that, like a case-insensitive
// database collation, finds a user whatever the case of the name.
type namesCaseInsensitively struct{ model.UsersStore }

func (s namesCaseInsensitively) Get(username string) (*model.User, error) {
	return s.UsersStore.Get(strings.ToLower(username))
}

func TestActorDelegation_RecordsCanonicalUsername(t *testing.T) {
	t.Parallel()
	_, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
	createTestUser(t, backends, "alice", false)
	actor, ok, err := delegatedActor(namesCaseInsensitively{backends.Users}, "ALICE")
	if err != nil || !ok || actor != "alice" {
		t.Fatalf("delegatedActor = %q, %v, %v; want alice, true, nil", actor, ok, err)
	}
	if actor, ok, err := delegatedActor(backends.Users, "mallory"); err != nil || ok || actor != "" {
		t.Errorf("Unknown user accepted: %q, %v, %v", actor, ok, err)
	}
}

func TestActorDelegation_RecordedInEvents(t *testing.T) {
	t.Parallel()
	app, backends := newServiceTokenTestApp(t, true, mustServiceToken(t, "operator-ui", testServiceSecret, true))
	createTestUser(t, backends, "alice", false)
	id := addPendingSubordinate(t, backends)

	resp, body := putStatus(t, app, id, bearer(testServiceSecret), "alice")
	requireStatus(t, resp, body, http.StatusOK)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/subordinates/%d", id), http.NoBody)
	req.Header.Set("Authorization", bearer(testServiceSecret))
	req.Header.Set("X-Actor", "alice")
	resp, _ = doRequest(t, app, req)
	assertStatusOneOf(t, resp, http.StatusOK, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodGet, "/events?entity_id="+testSubordinateID, http.NoBody)
	req.Header.Set("Authorization", bearer(testServiceSecret))
	resp, body = doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusOK)
	var page struct {
		Events []struct {
			Type  string `json:"type"`
			Actor string `json:"actor"`
		} `json:"events"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("Failed to parse events: %v (%s)", err, body)
	}
	got := map[string]string{}
	for _, e := range page.Events {
		got[e.Type] = e.Actor
	}
	for _, typ := range []string{model.EventTypeStatusUpdated, model.EventTypeDeleted} {
		if got[typ] != "alice" {
			t.Errorf("Expected the %s event to be recorded with actor alice, got %q (events %s)", typ, got[typ], body)
		}
	}
}

func TestOpenAPI_DocumentsBothSecuritySchemes(t *testing.T) {
	t.Parallel()
	app, _ := newServiceTokenTestApp(t, true, mustServiceToken(t, "svc", testServiceSecret, true))
	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/openapi.json", http.NoBody))
	requireStatus(t, resp, body, http.StatusOK)
	var doc struct {
		Info struct {
			Description string `json:"description"`
		} `json:"info"`
		Security   []map[string]any `json:"security"`
		Components struct {
			SecuritySchemes map[string]struct {
				Type   string `json:"type"`
				Scheme string `json:"scheme"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("Failed to parse openapi.json: %v", err)
	}
	if s := doc.Components.SecuritySchemes["serviceToken"]; s.Type != "http" || s.Scheme != "bearer" {
		t.Errorf("Expected an http bearer serviceToken scheme, got %+v", s)
	}
	if s := doc.Components.SecuritySchemes["basicAuth"]; s.Type != "http" || s.Scheme != "basic" {
		t.Errorf("Expected an http basic basicAuth scheme, got %+v", s)
	}
	found := map[string]bool{}
	for _, req := range doc.Security {
		for name := range req {
			found[name] = true
		}
	}
	if !found["basicAuth"] || !found["serviceToken"] {
		t.Errorf("Expected basicAuth and serviceToken as security alternatives, got %v", doc.Security)
	}
	for _, section := range []string{"## Authentication", "## Acting for an operator"} {
		if !strings.Contains(doc.Info.Description, section) {
			t.Errorf("Expected info.description to have a %q section", section)
		}
	}
}

// failingUserLookups is a user store whose lookups fail.
type failingUserLookups struct{ model.UsersStore }

func (failingUserLookups) Get(string) (*model.User, error) {
	return nil, errors.New("database unavailable")
}

func TestActorDelegation_UserLookupFailureWritesNothing(t *testing.T) {
	t.Parallel()
	backends, err := newTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	backends.Users = failingUserLookups{backends.Users}
	app := fiber.New()
	if err := Register(
		app, "https://ta.example.org", backends, nil, KeyManagement{}, nil,
		&Options{
			ServiceTokens:   ServiceTokens{mustServiceToken(t, "operator-ui", testServiceSecret, true)},
			ServiceListener: true,
		},
	); err != nil {
		t.Fatalf("Register: %v", err)
	}
	id := addPendingSubordinate(t, backends)
	resp, body := putStatus(t, app, id, bearer(testServiceSecret), "alice")
	assertErrorResponse(t, resp, body, http.StatusInternalServerError, "server_error")
	requireNothingWritten(t, backends, id)
}

func TestLoadServiceToken_WarnsAboutReadableFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not checked on Windows")
	}
	for name, tc := range map[string]struct {
		mode os.FileMode
		warn bool
	}{
		"OwnerOnly":     {0o600, false},
		"GroupReadable": {0o640, true},
		"WorldReadable": {0o604, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := writeServiceTokenFile(t, testServiceSecret)
			if err := os.Chmod(file, tc.mode); err != nil {
				t.Fatalf("Chmod: %v", err)
			}
			var buf bytes.Buffer
			if _, err := loadServiceToken("operator-ui", file, false, zerolog.New(&buf)); err != nil {
				t.Fatalf("loadServiceToken: %v", err)
			}
			out := buf.String()
			if strings.Contains(out, testServiceSecret) {
				t.Fatalf("The log contains the token: %s", out)
			}
			if got := strings.Contains(out, `"level":"warn"`); got != tc.warn {
				t.Fatalf("warning logged = %v, want %v (log: %s)", got, tc.warn, out)
			}
			if tc.warn && !strings.Contains(out, `"service_token":"operator-ui"`) {
				t.Errorf("Expected the warning to name the token: %s", out)
			}
		})
	}
}

// openAPIDoc parses one of the embedded OpenAPI documents.
func openAPIDoc(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := assets.ReadFile(name)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", name, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("Failed to parse %s: %v", name, err)
	}
	return doc
}

func TestOpenAPIUsers_AcceptsServiceTokens(t *testing.T) {
	t.Parallel()
	doc := openAPIDoc(t, "openapi-users.yaml")
	schemes := doc["components"].(map[string]any)["securitySchemes"].(map[string]any)
	if _, ok := schemes["serviceToken"]; !ok {
		t.Fatal("Expected a serviceToken security scheme in the users spec")
	}
	for p, item := range doc["paths"].(map[string]any) {
		for method, op := range item.(map[string]any) {
			sec, _ := op.(map[string]any)["security"].([]any)
			found := map[string]bool{}
			for _, req := range sec {
				for name := range req.(map[string]any) {
					found[name] = true
				}
			}
			if !found["basicAuth"] || !found["serviceToken"] {
				t.Errorf("%s %s: security %v, want basicAuth and serviceToken", method, p, sec)
			}
		}
	}
}

func TestOpenAPI_WriteOperationsDocumentXActor(t *testing.T) {
	t.Parallel()
	doc := openAPIDoc(t, "openapi.yaml")
	components := doc["components"].(map[string]any)
	param, _ := components["parameters"].(map[string]any)["XActor"].(map[string]any)
	if param["in"] != "header" || param["name"] != "X-Actor" || param["required"] != false {
		t.Errorf("Unexpected XActor parameter: %v", param)
	}
	if _, ok := components["responses"].(map[string]any)["InvalidActor"]; !ok {
		t.Error("Expected an InvalidActor response")
	}
	const paramRef = "#/components/parameters/XActor"
	for p, item := range doc["paths"].(map[string]any) {
		for method, op := range item.(map[string]any) {
			switch method {
			case "post", "put", "patch", "delete":
			default:
				continue
			}
			o := op.(map[string]any)
			hasParam := false
			params, _ := o["parameters"].([]any)
			for _, prm := range params {
				if m, ok := prm.(map[string]any); ok && m["$ref"] == paramRef {
					hasParam = true
				}
			}
			if !hasParam {
				t.Errorf("%s %s does not reference %s", method, p, paramRef)
			}
			if _, ok := o["responses"].(map[string]any)["400"]; !ok {
				t.Errorf("%s %s documents no 400 answer", method, p)
			}
		}
	}
}
