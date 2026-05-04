package lighthouse

// ServerConf holds the server configuration.
//
// Environment variables (accent prefix LH_SERVER_):
//   - LH_SERVER_IP_LISTEN: IP address to listen on
//   - LH_SERVER_PORT: HTTP server port
//   - LH_SERVER_PREFORK: Enable multiple processes (prefork)
//   - LH_SERVER_TRUSTED_PROXIES: Comma-separated list of trusted proxy IPs
//   - LH_SERVER_FORWARDED_IP_HEADER: Header name for forwarded IP
//   - LH_SERVER_TLS_ENABLED: Enable TLS
//   - LH_SERVER_TLS_REDIRECT_HTTP: Redirect HTTP to HTTPS
//   - LH_SERVER_TLS_CERT: Path to TLS certificate
//   - LH_SERVER_TLS_KEY: Path to TLS private key
//   - LH_SERVER_CORS_*: CORS configuration (see CORSConf)
type ServerConf struct {
	// IPListen is the IP address to listen on.
	// Env: LH_SERVER_IP_LISTEN
	IPListen string `yaml:"ip_listen" envconfig:"IP_LISTEN"`
	// Port is the HTTP server port.
	// Env: LH_SERVER_PORT
	Port int `yaml:"port" envconfig:"PORT"`
	// Prefork enables multiple processes listening on the same port.
	// When enabled, Fiber spawns child processes to distribute connections
	// across CPU cores for improved performance.
	// Note: When using prefork, it is strongly recommended to use Redis for
	// caching to ensure cache consistency across processes.
	// Env: LH_SERVER_PREFORK
	Prefork bool `yaml:"prefork" envconfig:"PREFORK"`
	// AdminAPIPort is set internally and not configurable via env.
	AdminAPIPort int `yaml:"-" envconfig:"-"`
	// TLS holds TLS configuration.
	// Env prefix: LH_SERVER_TLS_
	TLS tlsConf `yaml:"tls" envconfig:"TLS"`
	// TrustedProxies is a list of trusted proxy IPs.
	// Env: LH_SERVER_TRUSTED_PROXIES (comma-separated)
	TrustedProxies []string `yaml:"trusted_proxies" envconfig:"TRUSTED_PROXIES"`
	// ForwardedIPHeader is the header name for forwarded IP.
	// Env: LH_SERVER_FORWARDED_IP_HEADER
	ForwardedIPHeader string `yaml:"forwarded_ip_header" envconfig:"FORWARDED_IP_HEADER"`
	// CORS holds CORS middleware configuration for the main server.
	// Env prefix: LH_SERVER_CORS_
	CORS CORSConf `yaml:"cors" envconfig:"CORS"`
	// Secure bool    `yaml:"-"`
	// Basepath       string       `yaml:"-"`
}

// CORSConf holds CORS middleware configuration.
//
// Environment variables (with prefix based on parent, e.g., LH_SERVER_CORS_ or LH_API_ADMIN_CORS_):
//   - *_ENABLED: Enable CORS middleware
//   - *_ALLOW_ORIGINS: Comma-separated allowed origins or "*" for all
//   - *_ALLOW_METHODS: Comma-separated allowed HTTP methods
//   - *_ALLOW_HEADERS: Comma-separated allowed request headers
//   - *_ALLOW_CREDENTIALS: Allow credentials (cookies, authorization headers)
//   - *_EXPOSE_HEADERS: Comma-separated headers to expose to the browser
//   - *_MAX_AGE: Preflight request cache duration in seconds
type CORSConf struct {
	// Enabled enables CORS middleware.
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`
	// AllowOrigins is a comma-separated list of allowed origins, or "*" for all.
	AllowOrigins string `yaml:"allow_origins" envconfig:"ALLOW_ORIGINS"`
	// AllowMethods is a comma-separated list of allowed HTTP methods.
	AllowMethods string `yaml:"allow_methods" envconfig:"ALLOW_METHODS"`
	// AllowHeaders is a comma-separated list of allowed request headers.
	AllowHeaders string `yaml:"allow_headers" envconfig:"ALLOW_HEADERS"`
	// AllowCredentials indicates whether credentials (cookies, authorization headers) are allowed.
	AllowCredentials bool `yaml:"allow_credentials" envconfig:"ALLOW_CREDENTIALS"`
	// ExposeHeaders is a comma-separated list of headers to expose to the browser.
	ExposeHeaders string `yaml:"expose_headers" envconfig:"EXPOSE_HEADERS"`
	// MaxAge is the preflight request cache duration in seconds.
	MaxAge int `yaml:"max_age" envconfig:"MAX_AGE"`
}

// tlsConf holds TLS configuration.
//
// Environment variables (with prefix LH_SERVER_TLS_):
//   - LH_SERVER_TLS_ENABLED: Enable TLS
//   - LH_SERVER_TLS_REDIRECT_HTTP: Redirect HTTP to HTTPS
//   - LH_SERVER_TLS_CERT: Path to TLS certificate
//   - LH_SERVER_TLS_KEY: Path to TLS private key
type tlsConf struct {
	// Enabled enables TLS.
	// Env: LH_SERVER_TLS_ENABLED
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`
	// RedirectHTTP redirects HTTP to HTTPS.
	// Env: LH_SERVER_TLS_REDIRECT_HTTP
	RedirectHTTP bool `yaml:"redirect_http" envconfig:"REDIRECT_HTTP"`
	// Cert is the path to the TLS certificate.
	// Env: LH_SERVER_TLS_CERT
	Cert string `yaml:"cert" envconfig:"CERT"`
	// Key is the path to the TLS private key.
	// Env: LH_SERVER_TLS_KEY
	Key string `yaml:"key" envconfig:"KEY"`
}
