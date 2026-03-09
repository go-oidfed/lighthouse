package config

import "github.com/go-oidfed/lighthouse/storage"

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
	},
}
