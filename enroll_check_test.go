package lighthouse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// recordingChecker is an EntityChecker with a fixed decision that records
// what it was asked.
type recordingChecker struct {
	ok          bool
	status      int
	description string
	calls       int
	subject     string
	types       []string
}

func (c *recordingChecker) Check(ec *oidfed.EntityStatement, types []string) (bool, int, *oidfed.Error) {
	c.calls++
	c.subject, c.types = ec.Subject, types
	if c.ok {
		return true, 0, nil
	}
	return false, c.status, oidfed.ErrorInvalidRequest(c.description)
}

func (*recordingChecker) UnmarshalYAML(*yaml.Node) error { return nil }

func enrollCheckerFor(fed *LightHouse) adminEnrollChecker { return adminEnrollChecker{fed: fed} }

func TestAdminEnrollChecker_NoChecker(t *testing.T) {
	// No entity is fetched without a checker: the ID is not even reachable.
	fed := &LightHouse{}
	res, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), "http://127.0.0.1:1", []string{"x"})
	require.NoError(t, err)
	assert.False(t, res.Configured)
	assert.False(t, res.OK)
	assert.Empty(t, res.Reason)
	assert.Equal(t, []string{"x"}, res.EntityTypes)
}

func TestAdminEnrollChecker_Passing(t *testing.T) {
	entityID := selfSignedEntityServer(t)
	checker := &recordingChecker{ok: true}
	fed := &LightHouse{}
	fed.setEnrollChecker(checker)

	res, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), entityID, nil)
	require.NoError(t, err)
	assert.True(t, res.Configured)
	assert.True(t, res.OK)
	assert.Empty(t, res.Reason)
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, entityID, checker.subject)
	// Without registered types they are guessed from the metadata, as the
	// enroll endpoint does.
	assert.Equal(t, []string{"openid_relying_party"}, res.EntityTypes)
	assert.Equal(t, res.EntityTypes, checker.types)
}

func TestAdminEnrollChecker_FailingWithReason(t *testing.T) {
	entityID := selfSignedEntityServer(t)
	checker := &recordingChecker{status: http.StatusForbidden, description: "missing trust mark"}
	fed := &LightHouse{}
	fed.setEnrollChecker(checker)

	res, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), entityID, []string{"openid_relying_party"})
	require.NoError(t, err)
	assert.True(t, res.Configured)
	assert.False(t, res.OK)
	assert.Equal(t, "missing trust mark", res.Reason)
	assert.Equal(t, []string{"openid_relying_party"}, checker.types)
}

func TestAdminEnrollChecker_UnreachableEntity(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	entityID := srv.URL
	srv.Close()
	checker := &recordingChecker{ok: true}
	fed := &LightHouse{}
	fed.setEnrollChecker(checker)

	res, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), entityID, nil)
	require.NoError(t, err)
	assert.True(t, res.Configured)
	assert.False(t, res.OK)
	assert.Equal(t, "could not obtain or verify entity configuration", res.Reason)
	assert.Zero(t, checker.calls, "the checker must not run without a configuration")
}

func TestAdminEnrollChecker_TimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-release:
				case <-r.Context().Done():
				}
			},
		),
	)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	old := enrollCheckTimeout
	enrollCheckTimeout = 100 * time.Millisecond
	t.Cleanup(func() { enrollCheckTimeout = old })
	checker := &recordingChecker{ok: true}
	fed := &LightHouse{}
	fed.setEnrollChecker(checker)

	start := time.Now()
	res, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.True(t, res.Configured)
	assert.False(t, res.OK)
	assert.Equal(t, "the entity did not answer in time", res.Reason)
	assert.Zero(t, checker.calls)
}

// The admin check and the enroll endpoint must decide alike for the same
// checker and entity.
func TestAdminEnrollChecker_MatchesEnrollEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name    string
		checker *recordingChecker
		want    int
	}{
		{"accepted", &recordingChecker{ok: true}, http.StatusCreated},
		{"refused", &recordingChecker{status: http.StatusForbidden, description: "not allowed"}, http.StatusForbidden},
	} {
		t.Run(
			tc.name, func(t *testing.T) {
				entityID := selfSignedEntityServer(t)
				fed, app := setupEnrollCheckerApp(t, tc.checker)

				adminRes, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), entityID, nil)
				require.NoError(t, err)
				resp, err := app.Test(httptest.NewRequest("GET", enrollURL("/enroll", entityID), http.NoBody))
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, tc.want, resp.StatusCode)
				assert.Equal(t, tc.want == http.StatusCreated, adminRes.OK)
				assert.True(t, adminRes.Configured)
			},
		)
	}
}

func TestEnrollEndpoint_RegistersItsChecker(t *testing.T) {
	checker := &recordingChecker{ok: true}
	fed, _ := setupEnrollCheckerApp(t, checker)
	assert.Same(t, checker, fed.currentEnrollChecker())

	fed.setEnrollChecker(nil)
	assert.Nil(t, fed.currentEnrollChecker())
}

// A reload that finds no enroll endpoint must not leave the old checker
// behind.
func TestLoadEndpointsFromDB_ClearsEnrollChecker(t *testing.T) {
	st := newTestStorage(t)
	fed := &LightHouse{
		storages:         model.Backends{FederationEndpoints: storage.NewFederationEndpointStorage(st.DB())},
		endpointRegistry: NewEndpointRegistry(),
	}
	fed.setEnrollChecker(&recordingChecker{ok: true})
	require.NoError(t, fed.LoadEndpointsFromDB())
	assert.Nil(t, fed.currentEnrollChecker())
}

// setupEnrollCheckerApp registers an enroll endpoint with checker.
func setupEnrollCheckerApp(t *testing.T, checker EntityChecker) (*LightHouse, *fiber.App) {
	t.Helper()
	st := newTestStorage(t)
	store := st.SubordinateStorage()
	fed := &LightHouse{
		FederationEntity: stubSigningFedEntity{jwt: []byte("signed-statement-jwt")},
		storages:         model.Backends{Subordinates: store, KV: st.KeyValue()},
		fedMetadata:      oidfed.FederationEntityMetadata{},
	}
	require.NoError(t, fed.AddEnrollEndpoint(EndpointConf{Path: "/enroll"}, store, checker))
	app := fiber.New()
	app.All("/*", fed.dispatch)
	return fed, app
}

func TestAdminEnrollChecker_Configured(t *testing.T) {
	fed := &LightHouse{}
	assert.False(t, enrollCheckerFor(fed).Configured())
	fed.setEnrollChecker(&recordingChecker{ok: true})
	assert.True(t, enrollCheckerFor(fed).Configured())
	fed.setEnrollChecker(nil)
	assert.False(t, enrollCheckerFor(fed).Configured())
}

// While endpoints load, the current checker stays; the loaded one replaces
// it when the load is done.
func TestEnrollChecker_SwappedAtTheEndOfALoad(t *testing.T) {
	fed := &LightHouse{}
	old, loaded := &recordingChecker{ok: true}, &recordingChecker{ok: true}
	fed.setEnrollChecker(old)

	fed.beginEnrollCheckerLoad()
	assert.Same(t, old, fed.currentEnrollChecker())
	fed.setEnrollChecker(loaded)
	assert.Same(t, old, fed.currentEnrollChecker())
	fed.endEnrollCheckerLoad()
	assert.Same(t, loaded, fed.currentEnrollChecker())

	// A load without an enroll endpoint leaves no checker.
	fed.beginEnrollCheckerLoad()
	fed.endEnrollCheckerLoad()
	assert.Nil(t, fed.currentEnrollChecker())
}

func TestAdminEnrollChecker_NoMetadata(t *testing.T) {
	entityID := selfSignedEntityServerWithMetadata(t, nil)
	checker := &recordingChecker{ok: true}
	fed := &LightHouse{}
	fed.setEnrollChecker(checker)

	res, err := enrollCheckerFor(fed).CheckEnroll(context.Background(), entityID, nil)
	require.NoError(t, err)
	assert.True(t, res.Configured)
	assert.False(t, res.OK)
	assert.Equal(t, "entity configuration has no metadata", res.Reason)
	assert.Zero(t, checker.calls)

	// The library panics on a configuration without metadata when types are
	// requested; that must not end the process.
	res, err = enrollCheckerFor(fed).CheckEnroll(context.Background(), entityID, []string{"openid_provider"})
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, "could not obtain or verify entity configuration", res.Reason)
}

func enrollRaw(t *testing.T, app *fiber.App, url string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", url, http.NoBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	return resp.StatusCode, m
}

func TestEnrollEndpoint_NoMetadataAndNoTypesIsRefused(t *testing.T) {
	entityID := selfSignedEntityServerWithMetadata(t, nil)
	fed, app := setupEnrollCheckerApp(t, nil)

	status, body := enrollRaw(t, app, "/enroll?sub="+entityID)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_request", body["error"])
	assert.Equal(t, "entity configuration has no metadata", body["error_description"])
	stored, err := fed.storages.Subordinates.Get(entityID)
	require.NoError(t, err)
	assert.Nil(t, stored, "nothing may be stored")
}

func TestEnrollEndpoint_GuessesTypesWithoutRequestedOnes(t *testing.T) {
	entityID := selfSignedEntityServer(t)
	checker := &recordingChecker{ok: true}
	fed, app := setupEnrollCheckerApp(t, checker)

	status, _ := enrollRaw(t, app, "/enroll?sub="+entityID)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, []string{"openid_relying_party"}, checker.types)
	stored, err := fed.storages.Subordinates.Get(entityID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Len(t, stored.SubordinateEntityTypes, 1)
	assert.Equal(t, "openid_relying_party", stored.SubordinateEntityTypes[0].EntityType)
}

func TestEnrollEndpoint_ResolveFailure(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	entityID := srv.URL
	srv.Close()
	fed, app := setupEnrollCheckerApp(t, &recordingChecker{ok: true})

	status, body := enrollRaw(t, app, enrollURL("/enroll", entityID))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_request", body["error"])
	assert.Equal(t, "could not obtain or verify entity configuration", body["error_description"])
	stored, err := fed.storages.Subordinates.Get(entityID)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

// A blocking resolver: at most max resolutions run, further entities are
// refused as busy, and concurrent checks of one entity share a resolution.
func TestAdminEnrollChecker_BoundsResolutions(t *testing.T) {
	const max = 3
	release := make(chan struct{})
	var mu sync.Mutex
	started := map[string]int{}
	running, peak := 0, 0
	resolutions := newEnrollResolutions(
		max, func(entityID string, _ []string) *oidfed.EntityStatement {
			mu.Lock()
			started[entityID]++
			running++
			peak = max2(peak, running)
			mu.Unlock()
			<-release
			mu.Lock()
			running--
			mu.Unlock()
			return nil
		},
	)
	fed := &LightHouse{}
	fed.setEnrollChecker(&recordingChecker{ok: true})
	checker := adminEnrollChecker{fed: fed, resolutions: resolutions}

	old := enrollCheckTimeout
	enrollCheckTimeout = 30 * time.Second
	t.Cleanup(func() { enrollCheckTimeout = old })

	var wg sync.WaitGroup
	results := make(chan error, 20)
	call := func(id string) {
		defer wg.Done()
		_, err := checker.CheckEnroll(context.Background(), id, nil)
		results <- err
	}
	// Five concurrent checks of one entity share one resolution.
	for range 5 {
		wg.Add(1)
		go call("https://same.example.org")
	}
	require.Eventually(
		t, func() bool { mu.Lock(); defer mu.Unlock(); return started["https://same.example.org"] == 1 },
		5*time.Second, 5*time.Millisecond,
	)
	// Two more entities fill the cap; the next ones are refused.
	for _, id := range []string{"https://a.example.org", "https://b.example.org"} {
		wg.Add(1)
		go call(id)
	}
	require.Eventually(
		t, func() bool { mu.Lock(); defer mu.Unlock(); return running == max }, 5*time.Second, 5*time.Millisecond,
	)
	for _, id := range []string{"https://c.example.org", "https://d.example.org", "https://e.example.org"} {
		_, err := checker.CheckEnroll(context.Background(), id, nil)
		assert.ErrorIs(t, err, adminapi.ErrEnrollCheckBusy, id)
	}
	// Even a refused call starts nothing.
	mu.Lock()
	assert.Equal(t, max, running)
	assert.Equal(t, 1, started["https://same.example.org"])
	assert.Zero(t, started["https://c.example.org"])
	mu.Unlock()

	close(release)
	wg.Wait()
	close(results)
	for err := range results {
		assert.NoError(t, err)
	}
	assert.LessOrEqual(t, peak, max)
	// The slots are free again.
	require.Eventually(
		t, func() bool { mu.Lock(); defer mu.Unlock(); return running == 0 }, 5*time.Second, 5*time.Millisecond,
	)
	_, err := checker.CheckEnroll(context.Background(), "https://c.example.org", nil)
	assert.NoError(t, err)
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
