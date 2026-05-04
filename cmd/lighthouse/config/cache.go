package config

import (
	"github.com/zachmann/go-utils/duration"
)

// CachingConf holds caching configuration.
//
// Environment variables (with prefix LH_CACHE_):
//   - LH_CACHE_REDIS_ADDR: Redis server address
//   - LH_CACHE_USERNAME: Redis username
//   - LH_CACHE_PASSWORD: Redis password
//   - LH_CACHE_REDIS_DB: Redis database number
//   - LH_CACHE_DISABLED: Disable caching
//   - LH_CACHE_MAX_LIFETIME: Maximum cache lifetime (e.g., "1h", "30m")
type CachingConf struct {
	// RedisAddr is the Redis server address.
	// Env: LH_CACHE_REDIS_ADDR
	RedisAddr string `yaml:"redis_addr" envconfig:"REDIS_ADDR"`
	// Username is the Redis username.
	// Env: LH_CACHE_USERNAME
	Username string `yaml:"username" envconfig:"USERNAME"`
	// Password is the Redis password.
	// Env: LH_CACHE_PASSWORD
	Password string `yaml:"password" envconfig:"PASSWORD"`
	// RedisDB is the Redis database number.
	// Env: LH_CACHE_REDIS_DB
	RedisDB int `yaml:"redis_db" envconfig:"REDIS_DB"`
	// Disabled disables caching.
	// Env: LH_CACHE_DISABLED
	Disabled bool `yaml:"disabled" envconfig:"DISABLED"`
	// MaxLifetime is the maximum cache lifetime.
	// Env: LH_CACHE_MAX_LIFETIME
	MaxLifetime duration.DurationOption `yaml:"max_lifetime" envconfig:"MAX_LIFETIME"`
}
