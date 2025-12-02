package config

import (
	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/fileutils"
)

// loggingConf holds all logging-related configuration under the `logging` key.
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
	Access   LoggerConf         `yaml:"access"`
	Internal internalLoggerConf `yaml:"internal"`
	Banner   bannerConf         `yaml:"banner"`
}

// bannerConf controls whether startup banners are printed.
//   - logo: prints the Lighthouse logo banner when starting.
//   - version: prints the version banner alongside the logo.
//     The version banner uses the visible width of the logo banner (ANSI removed)
//     for alignment and is horizontally centered to match.
type bannerConf struct {
	// Logo prints the Lighthouse logo banner on startup.
	Logo bool `yaml:"logo"`
	// Version prints the current Lighthouse version as an ASCII banner.
	// The banner is rendered from digit/period glyphs and centered to the
	// logo banner's visible width.
	Version bool `yaml:"version"`
}

// internalLoggerConf configures application-internal logging.
// Level accepts standard log levels (e.g. DEBUG, INFO, WARN, ERROR).
// When Smart logging is enabled, errors are duplicated to a dedicated directory.
type internalLoggerConf struct {
	LoggerConf `yaml:",inline"`
	// Level sets the verbosity for internal logs (e.g. DEBUG, INFO).
	Level string `yaml:"level"`
	// Smart enables additional error-focused logging alongside general logs.
	Smart smartLoggerConf `yaml:"smart"`
}

// LoggerConf holds configuration related to logging
type LoggerConf struct {
	Dir    string `yaml:"dir"`
	StdErr bool   `yaml:"stderr"`
}

// smartLoggerConf enables and configures 'smart' logging.
// If Enabled, error logs are also written to `Dir`. If `Dir` is empty, it
// falls back to the internal logger's `Dir`.
type smartLoggerConf struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
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
