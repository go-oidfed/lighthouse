package lighthouse

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// TestTrustMarkEndpoint_AdminAPIStatusChangeInvalidatesCaches verifies that a
// subject blocked through the admin API no longer gets the trust mark that
// the trust mark endpoint issued and cached before.
func TestTrustMarkEndpoint_AdminAPIStatusChangeInvalidatesCaches(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	backends, err := store.Backends(storage.JTIStorageDB)
	require.NoError(t, err)

	spec, err := backends.TrustMarkSpecs.Create(
		&model.AddTrustMarkSpec{
			TrustMarkType: trustMarkIssuanceTestType,
			CacheTTL:      3600,
			EligibilityConfig: &model.EligibilityConfig{
				Mode:          model.EligibilityModeDBOnly,
				CheckCacheTTL: 3600,
			},
		},
	)
	require.NoError(t, err)
	subject, err := backends.TrustMarkSpecs.CreateSubject(
		trustMarkIssuanceTestType, &model.AddTrustMarkSubject{
			EntityID: trustMarkIssuanceTestSubject,
			Status:   model.StatusActive,
		},
	)
	require.NoError(t, err)

	fed := newTrustMarkIssuanceTestLightHouse(t, backends.TrustMarkSpecs)
	fed.storages = backends
	path := "/tm"
	require.NoError(
		t, fed.loadEndpointFromDB(
			&model.FederationEndpoint{
				Type: model.EndpointTypeTrustMark,
				Path: &path,
			},
		),
	)
	app := fiber.New()
	_, err = initAdminAPI(
		AdminAPIOptions{Enabled: true}, ServerConf{}, app, fed, trustMarkIssuanceTestEntityID, backends,
		fed.FederationEntity, adminapi.KeyManagement{}, nil,
	)
	require.NoError(t, err)
	app.All("/*", fed.dispatch)

	first := requestTrustMark(t, app, trustMarkIssuanceTestType, trustMarkIssuanceTestSubject)
	require.Equal(t, first, requestTrustMark(t, app, trustMarkIssuanceTestType, trustMarkIssuanceTestSubject),
		"the second request should be served from the cache")

	req := httptest.NewRequest(
		fiber.MethodPut,
		fmt.Sprintf("/api/v1/admin/trust-marks/issuance-spec/%d/subjects/%d/status", spec.ID, subject.ID),
		strings.NewReader(model.StatusBlocked.String()),
	)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	req = httptest.NewRequest(
		fiber.MethodGet,
		"/tm?sub="+url.QueryEscape(trustMarkIssuanceTestSubject)+
			"&trust_mark_type="+url.QueryEscape(trustMarkIssuanceTestType),
		nil,
	)
	resp, err = app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEqual(t, first, string(body), "the cached trust mark must not be served after blocking")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "body: %s", body)
}
