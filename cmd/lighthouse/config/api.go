package config

import (
	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/storage"
)

// apiConf holds API-related configuration.
//
// Environment variables (with prefix LH_API_):
//   - LH_API_ADMIN_ENABLED: Enable admin API
//   - LH_API_ADMIN_USERS_ENABLED: Enable user management
//   - LH_API_ADMIN_PORT: Admin API port (0 = use main server)
//   - LH_API_ADMIN_PASSWORD_HASHING_TIME: Argon2id time parameter
//   - LH_API_ADMIN_PASSWORD_HASHING_MEMORY_KIB: Argon2id memory in KiB
//   - LH_API_ADMIN_PASSWORD_HASHING_PARALLELISM: Argon2id parallelism
//   - LH_API_ADMIN_PASSWORD_HASHING_KEY_LEN: Argon2id key length
//   - LH_API_ADMIN_PASSWORD_HASHING_SALT_LEN: Argon2id salt length
type apiConf struct {
	// Admin holds admin API configuration.
	// Env prefix: LH_API_ADMIN_
	Admin adminAPIConf `yaml:"admin" envconfig:"ADMIN"`
}

// adminAPIConf holds admin API configuration.
//
// Environment variables (with prefix LH_API_ADMIN_):
//   - LH_API_ADMIN_ENABLED: Enable admin API
//   - LH_API_ADMIN_USERS_ENABLED: Enable user management
//   - LH_API_ADMIN_PORT: Admin API port (0 = use main server)
//   - LH_API_ADMIN_PASSWORD_HASHING_*: Password hashing parameters
//   - LH_API_ADMIN_ACTOR_HEADER: HTTP header name for actor extraction
//   - LH_API_ADMIN_ACTOR_SOURCE: Preferred actor source ("basic_auth" or "header")
//   - LH_API_ADMIN_CORS_*: CORS configuration (see CORSConf)
//   - LH_API_ADMIN_TLS_ENABLED: Enable TLS for admin API
//   - LH_API_ADMIN_TLS_CERT: Path to TLS certificate for admin API
//   - LH_API_ADMIN_TLS_KEY: Path to TLS private key for admin API
//   - LH_API_ADMIN_SERVICE_PORT: Port of the service listener (0 = off)
//   - LH_API_ADMIN_SERVICE_IP_LISTEN: IP address of the service listener (empty = server.ip_listen)
//
// The service tokens (service_tokens) can be set in the config file only.
type adminAPIConf struct {
	// Enabled enables the admin API.
	// Env: LH_API_ADMIN_ENABLED
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`
	// UsersEnabled enables user management.
	// Env: LH_API_ADMIN_USERS_ENABLED
	UsersEnabled bool `yaml:"users_enabled" envconfig:"USERS_ENABLED"`
	// Port is the admin API port (0 = use main server).
	// Env: LH_API_ADMIN_PORT
	Port int `yaml:"port" envconfig:"PORT"`
	// Argon2idParams holds password hashing parameters.
	// Env prefix: LH_API_ADMIN_PASSWORD_HASHING_
	Argon2idParams storage.Argon2idParams `yaml:"password_hashing" envconfig:"PASSWORD_HASHING"`
	// ActorHeader is the HTTP header name to extract the actor from.
	// Env: LH_API_ADMIN_ACTOR_HEADER
	ActorHeader string `yaml:"actor_header" envconfig:"ACTOR_HEADER"`
	// ActorSource is the preferred source for actor extraction.
	// Valid values: "basic_auth" (default), "header".
	// The system tries the preferred source first, then falls back to the other.
	// Env: LH_API_ADMIN_ACTOR_SOURCE
	ActorSource string `yaml:"actor_source" envconfig:"ACTOR_SOURCE"`
	// CORS holds CORS configuration for the admin API.
	// Env prefix: LH_API_ADMIN_CORS_
	CORS lighthouse.CORSConf `yaml:"cors" envconfig:"CORS"`
	// TLS holds TLS configuration for the admin API.
	// When enabled with a custom port, the admin API will serve HTTPS instead of HTTP.
	// Env prefix: LH_API_ADMIN_TLS_
	TLS lighthouse.TLSConf `yaml:"tls" envconfig:"TLS"`
	// ServicePort, when not 0, starts the service listener: the admin API a
	// second time on this port (plain HTTP), accepting only service tokens.
	// Env: LH_API_ADMIN_SERVICE_PORT
	ServicePort int `yaml:"service_port" envconfig:"SERVICE_PORT"`
	// ServiceIPListen is the IP address the service listener binds to. Empty
	// means server.ip_listen.
	// Env: LH_API_ADMIN_SERVICE_IP_LISTEN
	ServiceIPListen string `yaml:"service_ip_listen" envconfig:"SERVICE_IP_LISTEN"`
	// ServiceTokens are the bearer tokens accepted on the service listener.
	// Config file only.
	ServiceTokens []lighthouse.ServiceTokenConf `yaml:"service_tokens" ignored:"true"`
}

var defaultAPIConf = apiConf{
	Admin: adminAPIConf{
		Enabled:      true,
		UsersEnabled: true,
		Port:         0, // 0 means use main server
		Argon2idParams: storage.Argon2idParams{
			Time:        1,
			MemoryKiB:   64 * 1024,
			Parallelism: 4,
			KeyLen:      64,
			SaltLen:     32,
		},
		ActorHeader: "X-Actor",
		ActorSource: "basic_auth",
		CORS: lighthouse.CORSConf{
			Enabled:          false,
			AllowOrigins:     "*",
			AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
			AllowCredentials: true,
			MaxAge:           3600,
		},
		TLS: lighthouse.TLSConf{
			Enabled:      false,
			RedirectHTTP: false,
			Cert:         "",
			Key:          "",
		},
	},
}
