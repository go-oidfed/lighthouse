package config

import (
	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/fileutils"
)

// loggingConf holds all logging-related configuration under the `logging` key.
//
// Environment variables (with prefix LH_LOGGING_):
//   - LH_LOGGING_ACCESS_DIR: Directory for access logs
//   - LH_LOGGING_ACCESS_STDERR: Log access to stderr
//   - LH_LOGGING_INTERNAL_DIR: Directory for internal logs
//   - LH_LOGGING_INTERNAL_STDERR: Log internal to stderr
//   - LH_LOGGING_INTERNAL_LEVEL: Log level (DEBUG, INFO, WARN, ERROR)
//   - LH_LOGGING_INTERNAL_SMART_ENABLED: Enable smart logging
//   - LH_LOGGING_INTERNAL_SMART_DIR: Smart log directory
//   - LH_LOGGING_BANNER_LOGO: Print logo on startup
//   - LH_LOGGING_BANNER_VERSION: Print version on startup
//
// Shortcut: LH_LOG_LEVEL is an alias for LH_LOGGING_INTERNAL_LEVEL
//
// YAML example:
//
//	logging:
//	  access:
//	    dir: /var/log/lighthouse
//	    stderr: false
//	  internal:
//	    dir: /var/log/lighthouse
//	    stderr: false
//	    level: INFO
//	    smart:
//	      enabled: false
//	      dir: /var/log/lighthouse/smart
//	  banner:
//	    logo: true
//	    version: true
type loggingConf struct {
	// Access holds access log configuration.
	// Env prefix: LH_LOGGING_ACCESS_
	Access LoggerConf `yaml:"access" envconfig:"ACCESS"`
	// Internal holds internal log configuration.
	// Env prefix: LH_LOGGING_INTERNAL_
	Internal internalLoggerConf `yaml:"internal" envconfig:"INTERNAL"`
	// Banner holds startup banner configuration.
	// Env prefix: LH_LOGGING_BANNER_
	Banner bannerConf `yaml:"banner" envconfig:"BANNER"`
}

// bannerConf controls whether startup banners are printed.
//
// Environment variables (with prefix LH_LOGGING_BANNER_):
//   - LH_LOGGING_BANNER_LOGO: Print logo on startup
//   - LH_LOGGING_BANNER_VERSION: Print version on startup
type bannerConf struct {
	// Logo prints the Lighthouse logo banner on startup.
	// Env: LH_LOGGING_BANNER_LOGO
	Logo bool `yaml:"logo" envconfig:"LOGO"`
	// Version prints the current Lighthouse version as an ASCII banner.
	// The banner is rendered from digit/period glyphs and centered to the
	// logo banner's visible width.
	// Env: LH_LOGGING_BANNER_VERSION
	Version bool `yaml:"version" envconfig:"VERSION"`
}

// internalLoggerConf configures application-internal logging.
// Level accepts standard log levels (e.g. DEBUG, INFO, WARN, ERROR).
// When Smart logging is enabled, errors are duplicated to a dedicated directory.
//
// Environment variables (with prefix LH_LOGGING_INTERNAL_):
//   - LH_LOGGING_INTERNAL_DIR: Directory for internal logs
//   - LH_LOGGING_INTERNAL_STDERR: Log to stderr
//   - LH_LOGGING_INTERNAL_LEVEL: Log level (DEBUG, INFO, WARN, ERROR)
//   - LH_LOGGING_INTERNAL_SMART_ENABLED: Enable smart logging
//   - LH_LOGGING_INTERNAL_SMART_DIR: Smart log directory
type internalLoggerConf struct {
	LoggerConf `yaml:",inline"`
	// Level sets the verbosity for internal logs (e.g. DEBUG, INFO).
	// Env: LH_LOGGING_INTERNAL_LEVEL or LH_LOG_LEVEL (shortcut)
	Level string `yaml:"level" envconfig:"LEVEL"`
	// Smart enables additional error-focused logging alongside general logs.
	// Env prefix: LH_LOGGING_INTERNAL_SMART_
	Smart smartLoggerConf `yaml:"smart" envconfig:"SMART"`
}

// LoggerConf holds configuration related to logging.
//
// Environment variables depend on context:
//   - Access logs: LH_LOGGING_ACCESS_DIR, LH_LOGGING_ACCESS_STDERR
//   - Internal logs: LH_LOGGING_INTERNAL_DIR, LH_LOGGING_INTERNAL_STDERR
type LoggerConf struct {
	// Dir is the directory for log files.
	// Env: LH_LOGGING_ACCESS_DIR or LH_LOGGING_INTERNAL_DIR
	Dir string `yaml:"dir" envconfig:"DIR"`
	// StdErr enables logging to stderr.
	// Env: LH_LOGGING_ACCESS_STDERR or LH_LOGGING_INTERNAL_STDERR
	StdErr bool `yaml:"stderr" envconfig:"STDERR"`
}

// smartLoggerConf enables and configures 'smart' logging.
// If Enabled, error logs are also written to `Dir`. If `Dir` is empty, it
// falls back to the internal logger's `Dir`.
//
// Environment variables (with prefix LH_LOGGING_INTERNAL_SMART_):
//   - LH_LOGGING_INTERNAL_SMART_ENABLED: Enable smart logging
//   - LH_LOGGING_INTERNAL_SMART_DIR: Smart log directory
type smartLoggerConf struct {
	// Enabled enables smart logging.
	// Env: LH_LOGGING_INTERNAL_SMART_ENABLED
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`
	// Dir is the directory for smart logs.
	// Env: LH_LOGGING_INTERNAL_SMART_DIR
	Dir string `yaml:"dir" envconfig:"DIR"`
}

func checkLoggingDirExists(dir string) error {
	if dir != "" && !fileutils.FileExists(dir) {
		return errors.Errorf("logging directory '%s' does not exist", dir)
	}
	return nil
}

func (log *loggingConf) validate() error {
	if err := checkLoggingDirExists(log.Access.Dir); err != nil {
		return err
	}
	if err := checkLoggingDirExists(log.Internal.Dir); err != nil {
		return err
	}
	if log.Internal.Smart.Enabled {
		if log.Internal.Smart.Dir == "" {
			log.Internal.Smart.Dir = log.Internal.Dir
		}
		if err := checkLoggingDirExists(log.Internal.Smart.Dir); err != nil {
			return err
		}
	}
	return nil
}

var defaultLoggingConf = loggingConf{
	Banner: bannerConf{
		Logo:    true,
		Version: true,
	},
	Internal: internalLoggerConf{
		Level: "INFO",
	},
}
