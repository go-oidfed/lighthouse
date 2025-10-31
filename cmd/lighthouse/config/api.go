package config

import "github.com/go-oidfed/lighthouse/storage"

// apiConf holds API-related configuration
type apiConf struct {
	Admin adminAPIConf `yaml:"admin"`
}

type adminAPIConf struct {
	Enabled        bool                   `yaml:"enabled"`
	UsersEnabled   bool                   `yaml:"users_enabled"`
	Port           int                    `yaml:"port"`
	Argon2idParams storage.Argon2idParams `yaml:"password_hashing"`
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
