package lighthouse

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHTTPEntityChecker_AllowOn2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), []string{"openid_provider"})
	assert.True(t, ok)
	assert.Equal(t, 0, code)
	assert.Nil(t, errResp)
}

func TestHTTPEntityChecker_DenyOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("entity not authorized"))
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, http.StatusForbidden, code)
	require.NotNil(t, errResp)
	assert.Equal(t, "forbidden", errResp.Error)
	assert.Equal(t, "entity not authorized", errResp.ErrorDescription)
}

func TestHTTPEntityChecker_Deny4xxPassesThroughStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL}
	ok, code, _ := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestHTTPEntityChecker_5xxReturnsBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadGateway, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "error status")
}

func TestHTTPEntityChecker_NetworkErrorReturnsBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // close immediately to force a connection error

	c := &HTTPEntityChecker{URL: srv.URL, Timeout: 5}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadGateway, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "request failed")
}

func TestHTTPEntityChecker_DefaultMethodIsPOST(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL}
	_, _, _ = c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.Equal(t, http.MethodPost, gotMethod)
}

func TestHTTPEntityChecker_DefaultBodyModeIsEntityConfiguration(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	es := testEntityStatement("https://op.example.org")
	c := &HTTPEntityChecker{URL: srv.URL}
	_, _, _ = c.Check(es, nil)

	expected, _ := json.Marshal(es.EntityStatementPayload)
	assert.JSONEq(t, string(expected), string(gotBody))
}

func TestHTTPEntityChecker_BodyModeNone(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL, BodyMode: "none"}
	_, _, _ = c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.Empty(t, gotBody)
}

func TestHTTPEntityChecker_BodyModeEntityID(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL, BodyMode: "entity_id"}
	_, _, _ = c.Check(
		testEntityStatement("https://op.example.org"),
		[]string{"openid_provider"},
	)

	var body map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &body))
	assert.Equal(t, "https://op.example.org", body["sub"])
	assert.Equal(t, []any{"openid_provider"}, body["entity_types"])
}

func TestHTTPEntityChecker_EntityHeadersAlwaysSet(t *testing.T) {
	var gotEntityID, gotEntityTypes string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotEntityID = r.Header.Get("X-Entity-ID")
		gotEntityTypes = r.Header.Get("X-Entity-Types")
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL, BodyMode: "none"}
	_, _, _ = c.Check(
		testEntityStatement("https://op.example.org"),
		[]string{"openid_provider", "trust_anchor"},
	)
	assert.Equal(t, "https://op.example.org", gotEntityID)
	assert.Equal(t, "openid_provider,trust_anchor", gotEntityTypes)
}

func TestHTTPEntityChecker_CustomHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{
		URL:     srv.URL,
		Headers: map[string]string{"Authorization": "Bearer token123"},
	}
	_, _, _ = c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.Equal(t, "Bearer token123", gotAuth)
}

func TestHTTPEntityChecker_CustomMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL, Method: "PUT"}
	_, _, _ = c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.Equal(t, "PUT", gotMethod)
}

func TestHTTPEntityChecker_ContentTypeJSON(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
	}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL} // default body_mode = entity_configuration
	_, _, _ = c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.Equal(t, "application/json", gotCT)
}

func TestHTTPEntityChecker_MissingURL(t *testing.T) {
	c := &HTTPEntityChecker{}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, 500, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "url is required")
}

func TestHTTPEntityChecker_InvalidBodyMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	c := &HTTPEntityChecker{URL: srv.URL, BodyMode: "bogus"}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, 500, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "unknown body_mode")
}

func TestHTTPEntityChecker_UnmarshalYAML(t *testing.T) {
	yamlStr := `
url: https://decision.example.org/check
method: PUT
headers:
  Authorization: Bearer secret
timeout: 15
body_mode: entity_id
`
	var c HTTPEntityChecker
	err := yaml.Unmarshal([]byte(yamlStr), &c)
	require.NoError(t, err)
	assert.Equal(t, "https://decision.example.org/check", c.URL)
	assert.Equal(t, "PUT", c.Method)
	assert.Equal(t, "Bearer secret", c.Headers["Authorization"])
	assert.Equal(t, 15, c.Timeout)
	assert.Equal(t, "entity_id", c.BodyMode)
}

func TestHTTPEntityChecker_Registered(t *testing.T) {
	ctor, ok := entityCheckerRegistry["http"]
	require.True(t, ok, "http checker should be registered")
	assert.NotNil(t, ctor())
}
