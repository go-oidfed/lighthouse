package lighthouse

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	apistats "github.com/go-oidfed/lighthouse/api/stats"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// testListenerToken is a 40-byte service token value.
const testListenerToken = "service-listener-token-0123456789abcdef"

// freeLoopbackPort returns a loopback TCP port that was free a moment ago.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// getWithAuthorization GETs u, with authorization as Authorization header
// when it is not empty.
func getWithAuthorization(t *testing.T, u, authorization string) (int, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, u, http.NoBody)
	require.NoError(t, err)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// writeListenerTokenFile writes the test token to a file and returns its path.
func writeListenerTokenFile(t *testing.T) string {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(testListenerToken+"\n"), 0o600))
	return tokenFile
}

// newListenerTestLightHouse builds a LightHouse on SQLite storage with the
// given admin options and starts it.
func newListenerTestLightHouse(t *testing.T, serverPort int, admin AdminAPIOptions) *LightHouse {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", url.PathEscape(t.Name()))
	store, err := storage.NewStorage(storage.Config{Driver: storage.DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	backends, err := store.Backends(storage.JTIStorageDB)
	require.NoError(t, err)

	signing := SigningConf{KMS: KMSFilesystem, PKBackend: PKBackendFilesystem, AutoGenerateKeys: true}
	signing.FileSystemBackend.KeyDir = t.TempDir()
	fed, err := NewLightHouse(
		ServerConf{IPListen: "127.0.0.1", Port: serverPort, AccessLogWriter: io.Discard},
		"https://lh-service.example.org", signing, backends, admin, apistats.Config{},
	)
	require.NoError(t, err)
	fed.LogoBanner, fed.VersionBanner = false, false
	go fed.Start()
	t.Cleanup(func() { assert.NoError(t, fed.Stop()) })
	return fed
}

func TestServiceListener_ServesAdminAPIToTokensOnly(t *testing.T) {
	serverPort, servicePort := freeLoopbackPort(t), freeLoopbackPort(t)
	fed := newListenerTestLightHouse(
		t, serverPort, AdminAPIOptions{
			Enabled: true, UsersEnabled: true, ServicePort: servicePort,
			ServiceTokens: []ServiceTokenConf{
				{Name: "operator-ui", TokenFile: writeListenerTokenFile(t), ActForUsers: true},
			},
		},
	)

	serviceURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/admin/subordinates", servicePort)
	mainURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/admin/subordinates", serverPort)
	require.Eventually(
		t, func() bool {
			s1, err1 := getWithAuthorization(t, serviceURL, "Bearer "+testListenerToken)
			_, err2 := getWithAuthorization(t, mainURL, "")
			return err1 == nil && err2 == nil && s1 == http.StatusOK
		}, 5*time.Second, 20*time.Millisecond,
	)

	status, err := getWithAuthorization(t, serviceURL, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, status, "the service listener needs a token, also without users")
	status, err = getWithAuthorization(t, serviceURL, "Bearer wrong-token-0123456789abcdef0123456789")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)

	// Once there is a user, the admin port takes Basic and refuses the token.
	_, err = fed.storages.Users.Create("alice", "secret-password", "")
	require.NoError(t, err)
	status, err = getWithAuthorization(t, mainURL, "Bearer "+testListenerToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, status, "the admin port refuses service tokens")
	req, err := http.NewRequest(http.MethodGet, mainURL, http.NoBody)
	require.NoError(t, err)
	req.SetBasicAuth("alice", "secret-password")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "the admin port keeps Basic")
	req, err = http.NewRequest(http.MethodGet, serviceURL, http.NoBody)
	require.NoError(t, err)
	req.SetBasicAuth("alice", "secret-password")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "the service listener refuses Basic")
}

func TestServiceListener_OffWithoutServicePort(t *testing.T) {
	app, tokens, err := initServiceAPI(
		AdminAPIOptions{Enabled: true}, ServerConf{Port: 8080}, nil, "https://ta.example.org",
		model.Backends{}, nil, adminapi.KeyManagement{}, nil,
	)
	require.NoError(t, err)
	assert.Nil(t, app, "no service port, no service listener")
	assert.Empty(t, tokens)

	app, tokens, err = initServiceAPI(
		AdminAPIOptions{
			Enabled: false, ServicePort: 7700,
			ServiceTokens: []ServiceTokenConf{{Name: "svc", TokenFile: writeListenerTokenFile(t)}},
		},
		ServerConf{Port: 8080}, nil, "https://ta.example.org", model.Backends{}, nil, adminapi.KeyManagement{}, nil,
	)
	require.NoError(t, err)
	assert.Nil(t, app, "admin API disabled, no service listener")
	assert.Empty(t, tokens)
}

func TestServiceListener_RefusesSharedPort(t *testing.T) {
	tokens := []ServiceTokenConf{{Name: "svc", TokenFile: writeListenerTokenFile(t)}}
	for name, tc := range map[string]struct {
		admin  AdminAPIOptions
		server ServerConf
	}{
		"server port": {AdminAPIOptions{Enabled: true, ServicePort: 8080, ServiceTokens: tokens}, ServerConf{Port: 8080}},
		"admin port": {
			AdminAPIOptions{Enabled: true, Port: 9090, ServicePort: 9090, ServiceTokens: tokens},
			ServerConf{Port: 8080},
		},
		"TLS port": {
			AdminAPIOptions{Enabled: true, ServicePort: 443, ServiceTokens: tokens},
			ServerConf{Port: 8080, TLS: tlsConf{Enabled: true}},
		},
	} {
		t.Run(
			name, func(t *testing.T) {
				_, _, err := initServiceAPI(
					tc.admin, tc.server, nil, "https://ta.example.org",
					model.Backends{}, nil, adminapi.KeyManagement{}, nil,
				)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "api.admin.service_port")
			},
		)
	}
}

func TestServiceListener_RefusesBadTokenFiles(t *testing.T) {
	short := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(short, []byte("too-short"), 0o600))
	for name, tokens := range map[string][]ServiceTokenConf{
		"none":    nil,
		"short":   {{Name: "svc", TokenFile: short}},
		"missing": {{Name: "svc", TokenFile: filepath.Join(t.TempDir(), "missing")}},
	} {
		t.Run(
			name, func(t *testing.T) {
				_, _, err := initServiceAPI(
					AdminAPIOptions{Enabled: true, ServicePort: 7700, ServiceTokens: tokens},
					ServerConf{Port: 8080}, nil, "https://ta.example.org", model.Backends{}, nil,
					adminapi.KeyManagement{}, nil,
				)
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "too-short")
			},
		)
	}
}

func TestServiceListener_ListenAddress(t *testing.T) {
	assert.Equal(
		t, "0.0.0.0:7700", serviceListenAddress("0.0.0.0", "", 7700),
		"no service_ip_listen: server.ip_listen",
	)
	assert.Equal(t, ":7700", serviceListenAddress("", "", 7700))
	assert.Equal(t, "127.0.0.1:7700", serviceListenAddress("0.0.0.0", "127.0.0.1", 7700))
	assert.Equal(t, "[::1]:7700", serviceListenAddress("0.0.0.0", "::1", 7700))
}

func TestServiceListener_BindsServiceIPListen(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback")
	}
	servicePort := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	newListenerTestLightHouse(
		t, freeLoopbackPort(t), AdminAPIOptions{
			Enabled: true, ServicePort: servicePort, ServiceIPListen: "::1",
			ServiceTokens: []ServiceTokenConf{{Name: "svc", TokenFile: writeListenerTokenFile(t)}},
		},
	)

	v6URL := fmt.Sprintf("http://[::1]:%d/api/v1/admin/subordinates", servicePort)
	require.Eventually(
		t, func() bool {
			s, err := getWithAuthorization(t, v6URL, "Bearer "+testListenerToken)
			return err == nil && s == http.StatusOK
		}, 5*time.Second, 20*time.Millisecond,
	)
	_, err = getWithAuthorization(
		t, fmt.Sprintf("http://127.0.0.1:%d/api/v1/admin/subordinates", servicePort), "Bearer "+testListenerToken,
	)
	assert.Error(t, err, "the service listener is not on server.ip_listen")
}

func TestServiceListener_WarnsWhenOnAllInterfaces(t *testing.T) {
	for addr, warn := range map[string]bool{
		":7700":          true,
		"0.0.0.0:7700":   true,
		"[::]:7700":      true,
		"127.0.0.1:7700": false,
		"10.0.0.5:7700":  false,
		"[::1]:7700":     false,
	} {
		var buf bytes.Buffer
		warnServiceListenerOnAllInterfaces(zerolog.New(&buf), addr)
		out := buf.String()
		assert.Equal(t, warn, strings.Contains(out, `"level":"warn"`), addr)
		if warn {
			assert.Contains(t, out, "api.admin.service_ip_listen", addr)
			assert.Equal(t, 1, strings.Count(out, "\n"), "one warning")
		}
	}
}
