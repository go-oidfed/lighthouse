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
//   - LH_LOGGING_INTERNAL_STDERR_FORMAT: Format for stderr output (console, json)
//   - LH_LOGGING_INTERNAL_DIR_FORMAT: Format for file output (console, json)
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
//	    stderr_format: console
//	    dir_format: json
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
//
// Environment variables (with prefix LH_LOGGING_INTERNAL_):
//   - LH_LOGGING_INTERNAL_DIR: Directory for internal logs
//   - LH_LOGGING_INTERNAL_STDERR: Log to stderr
//   - LH_LOGGING_INTERNAL_LEVEL: Log level (DEBUG, INFO, WARN, ERROR)
//   - LH_LOGGING_INTERNAL_STDERR_FORMAT: Format for stderr (console, json)
//   - LH_LOGGING_INTERNAL_DIR_FORMAT: Format for file (console, json)
type internalLoggerConf struct {
	LoggerConf `yaml:",inline"`
	// Level sets the verbosity for internal logs (e.g. DEBUG, INFO).
	// Env: LH_LOGGING_INTERNAL_LEVEL or LH_LOG_LEVEL (shortcut)
	Level string `yaml:"level" envconfig:"LEVEL"`
	// StdErrFormat selects the output format for stderr: "console"
	// (human-friendly, colored) or "json" (structured JSON).
	// Default: "console".
	// Env: LH_LOGGING_INTERNAL_STDERR_FORMAT
	StdErrFormat string `yaml:"stderr_format" envconfig:"STDERR_FORMAT"`
	// DirFormat selects the output format for log files: "console"
	// (human-friendly, no colors) or "json" (structured JSON).
	// Default: "json".
	// Env: LH_LOGGING_INTERNAL_DIR_FORMAT
	DirFormat string `yaml:"dir_format" envconfig:"DIR_FORMAT"`
}

// LoggerConf holds configuration related to logging.
type LoggerConf struct {
	// Dir is the directory for log files.
	// Env: LH_LOGGING_INTERNAL_DIR or LH_LOGGING_ACCESS_DIR
	Dir string `yaml:"dir" envconfig:"DIR"`
	// StdErr enables logging to stderr.
	// Env: LH_LOGGING_INTERNAL_STDERR or LH_LOGGING_ACCESS_STDERR
	StdErr bool `yaml:"stderr" envconfig:"STDERR"`
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
	return nil
}

var defaultLoggingConf = loggingConf{
	Banner: bannerConf{
		Logo:    true,
		Version: true,
	},
	Internal: internalLoggerConf{
		Level:        "INFO",
		StdErrFormat: "console",
		DirFormat:    "json",
	},
}
