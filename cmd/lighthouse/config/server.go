package config

import (
	"github.com/go-oidfed/lighthouse"
)

var defaultServerConf = lighthouse.ServerConf{
	Port:              7672,
	ForwardedIPHeader: "X-Forwarded-For",
}
