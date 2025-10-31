package lighthouse

type ServerConf struct {
	IPListen          string   `yaml:"ip_listen"`
	Port              int      `yaml:"port"`
	AdminAPIPort      int      `yaml:"-"`
	TLS               tlsConf  `yaml:"tls"`
	TrustedProxies    []string `yaml:"trusted_proxies"`
	ForwardedIPHeader string   `yaml:"forwarded_ip_header"`
	// Secure bool    `yaml:"-"`
	// Basepath       string       `yaml:"-"`
}

type tlsConf struct {
	Enabled      bool   `yaml:"enabled"`
	RedirectHTTP bool   `yaml:"redirect_http"`
	Cert         string `yaml:"cert"`
	Key          string `yaml:"key"`
}
