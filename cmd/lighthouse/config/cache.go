package config

import (
	"github.com/zachmann/go-utils/duration"
)

type cachingConf struct {
	RedisAddr   string                  `yaml:"redis_addr"`
	Username    string                  `yaml:"username"`
	Password    string                  `yaml:"password"`
	RedisDB     int                     `yaml:"redis_db"`
	Disabled    bool                    `yaml:"disabled"`
	MaxLifetime duration.DurationOption `yaml:"max_lifetime"`
}
