package config

import (
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse"
)

// newBaseConf returns a config that passes Validate, without a service listener.
func newBaseConf() Config {
	cfg := Config{
		EntityID: "https://ta.example.org",
		Server:   defaultServerConf,
		Logging:  defaultLoggingConf,
		Storage:  defaultStorageConf,
		Signing:  defaultSigningConf,
		API:      defaultAPIConf,
		Stats:    defaultStatsConf,
	}
	cfg.Storage.DataDir = "/data"
	cfg.Signing.KMS = lighthouse.KMSFilesystem
	cfg.Signing.FileSystemBackend.KeyDir = "/keys"
	return cfg
}

// newServiceConf returns a config with one service token and the given ports.
func newServiceConf(adminPort, servicePort int) *Config {
	cfg := newBaseConf()
	cfg.API.Admin.Port = adminPort
	cfg.API.Admin.ServicePort = servicePort
	cfg.API.Admin.ServiceTokens = []lighthouse.ServiceTokenConf{
		{Name: "operator-ui", TokenFile: "/run/secrets/operator-ui-token", ActForUsers: true},
	}
	return &cfg
}

func TestServiceListenerConfig_OffByDefault(t *testing.T) {
	cfg := newBaseConf()
	assert.Zero(t, cfg.API.Admin.ServicePort)
	assert.Empty(t, cfg.API.Admin.ServiceTokens)
	assert.NoError(t, cfg.Validate())
}

func TestServiceListenerConfig_TokensNeedServicePort(t *testing.T) {
	err := newServiceConf(0, 0).validateServiceListener()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.admin.service_tokens")
	assert.Contains(t, err.Error(), "api.admin.service_port")

	assert.NoError(t, newServiceConf(0, defaultServerConf.Port+2).validateServiceListener())
}

func TestServiceListenerConfig_ServicePortNeedsTokens(t *testing.T) {
	cfg := newServiceConf(0, defaultServerConf.Port+2)
	cfg.API.Admin.ServiceTokens = nil
	err := cfg.validateServiceListener()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.admin.service_tokens")
}

func TestServiceListenerConfig_ServicePortNeedsAdminAPI(t *testing.T) {
	cfg := newServiceConf(0, defaultServerConf.Port+2)
	cfg.API.Admin.Enabled = false
	err := cfg.validateServiceListener()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.admin.enabled")
}

func TestServiceListenerConfig_ServicePortMustDiffer(t *testing.T) {
	sameAsServer := newServiceConf(0, defaultServerConf.Port)
	err := sameAsServer.validateServiceListener()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.admin.service_port")

	adminPort := defaultServerConf.Port + 1
	err = newServiceConf(adminPort, adminPort).validateServiceListener()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.admin.service_port")

	tls := newServiceConf(0, 443)
	tls.Server.TLS.Enabled = true
	err = tls.validateServiceListener()
	require.Error(t, err, "with TLS the main server listens on 443")
	assert.Contains(t, err.Error(), "api.admin.service_port")

	redirect := newServiceConf(0, 80)
	redirect.Server.TLS.Enabled = true
	redirect.Server.TLS.RedirectHTTP = true
	require.Error(t, redirect.validateServiceListener(), "with the HTTP redirect the main server listens on 80")

	for _, port := range []int{-1, 65536} {
		require.Error(t, newServiceConf(0, port).validateServiceListener(), port)
	}
}

func TestServiceListenerConfig_TokenEntries(t *testing.T) {
	port := defaultServerConf.Port + 2

	noName := newServiceConf(0, port)
	noName.API.Admin.ServiceTokens[0].Name = ""
	assert.Error(t, noName.validateServiceListener())

	noFile := newServiceConf(0, port)
	noFile.API.Admin.ServiceTokens[0].TokenFile = ""
	assert.Error(t, noFile.validateServiceListener())

	dup := newServiceConf(0, port)
	dup.API.Admin.ServiceTokens = append(dup.API.Admin.ServiceTokens, dup.API.Admin.ServiceTokens[0])
	err := dup.validateServiceListener()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestServiceListenerConfig_ServiceIPListen(t *testing.T) {
	port := defaultServerConf.Port + 2
	for _, ip := range []string{"", "127.0.0.1", "10.0.0.5", "::1", "0.0.0.0"} {
		cfg := newServiceConf(0, port)
		cfg.API.Admin.ServiceIPListen = ip
		assert.NoError(t, cfg.validateServiceListener(), ip)
	}
	for _, ip := range []string{"localhost", "127.0.0.1:7700", "300.1.1.1", " 127.0.0.1"} {
		cfg := newServiceConf(0, port)
		cfg.API.Admin.ServiceIPListen = ip
		err := cfg.validateServiceListener()
		require.Error(t, err, ip)
		assert.Contains(t, err.Error(), "api.admin.service_ip_listen")
	}
}

func TestServiceListenerConfig_ValidateChecksIt(t *testing.T) {
	assert.Error(t, newServiceConf(0, 0).Validate())
	assert.NoError(t, newServiceConf(0, defaultServerConf.Port+2).Validate())
}

func TestServiceListenerConfig_FromYAMLAndEnv(t *testing.T) {
	cfg := newBaseConf()
	require.NoError(
		t, yaml.Unmarshal(
			[]byte(`
api:
  admin:
    service_port: 7700
    service_ip_listen: 127.0.0.1
    service_tokens:
      - name: operator-ui
        token_file: /run/secrets/operator-ui-token
        act_for_users: true
      - name: automation
        token_file: /run/secrets/automation-token
`), &cfg,
		),
	)
	assert.Equal(t, 7700, cfg.API.Admin.ServicePort)
	assert.Equal(t, "127.0.0.1", cfg.API.Admin.ServiceIPListen)

	t.Setenv("LH_API_ADMIN_SERVICE_PORT", "7701")
	t.Setenv("LH_API_ADMIN_SERVICE_IP_LISTEN", "10.0.0.5")
	require.NoError(t, envconfig.Process("LH", &cfg))
	assert.Equal(t, 7701, cfg.API.Admin.ServicePort)
	assert.Equal(t, "10.0.0.5", cfg.API.Admin.ServiceIPListen)
	assert.Equal(
		t, []lighthouse.ServiceTokenConf{
			{Name: "operator-ui", TokenFile: "/run/secrets/operator-ui-token", ActForUsers: true},
			{Name: "automation", TokenFile: "/run/secrets/automation-token"},
		}, cfg.API.Admin.ServiceTokens,
	)
	assert.NoError(t, cfg.Validate())
}
