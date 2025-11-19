package config

type cachingConf struct {
	RedisAddr string `yaml:"redis_addr"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	RedisDB   int    `yaml:"redis_db"`
}
