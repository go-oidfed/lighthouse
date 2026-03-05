package config

import (
	"time"

	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/fileutils"
)

// StatsConf holds all statistics collection configuration.
//
// YAML example:
//
//	stats:
//	  enabled: true
//	  buffer:
//	    size: 10000
//	    flush_interval: 5s
//	    flush_threshold: 0.8
//	  capture:
//	    client_ip: true
//	    user_agent: true
//	    query_params: true
//	    geo_ip:
//	      enabled: false
//	      database_path: /path/to/GeoLite2-Country.mmdb
//	  retention:
//	    detailed_days: 90
//	    aggregated_days: 365
//	  endpoints: []
type StatsConf struct {
	// Enabled controls whether statistics collection is active.
	Enabled bool `yaml:"enabled"`

	// Buffer configures the in-memory ring buffer for request logs.
	Buffer StatsBufferConf `yaml:"buffer"`

	// Capture controls what data is collected from each request.
	Capture StatsCaptureConf `yaml:"capture"`

	// Retention defines how long data is kept.
	Retention StatsRetentionConf `yaml:"retention"`

	// Endpoints is a list of endpoint paths to track.
	// If empty, all federation endpoints are tracked.
	// Example: ["/.well-known/openid-federation", "/fetch", "/resolve"]
	Endpoints []string `yaml:"endpoints"`
}

// StatsBufferConf configures the in-memory ring buffer.
type StatsBufferConf struct {
	// Size is the maximum number of entries in the ring buffer.
	// Default: 10000
	Size int `yaml:"size"`

	// FlushInterval is how often the buffer is flushed to the database.
	// Default: 5s
	FlushInterval time.Duration `yaml:"flush_interval"`

	// FlushThreshold triggers a flush when the buffer is this percentage full.
	// Value between 0 and 1. Default: 0.8
	FlushThreshold float64 `yaml:"flush_threshold"`
}

// StatsCaptureConf controls what request data is captured.
type StatsCaptureConf struct {
	// ClientIP records the client's IP address.
	ClientIP bool `yaml:"client_ip"`

	// UserAgent records the User-Agent header.
	UserAgent bool `yaml:"user_agent"`

	// QueryParams records URL query parameters as JSON.
	QueryParams bool `yaml:"query_params"`

	// GeoIP enables country lookup from IP addresses.
	GeoIP StatsGeoIPConf `yaml:"geo_ip"`
}

// StatsGeoIPConf configures GeoIP lookup.
type StatsGeoIPConf struct {
	// Enabled turns on GeoIP country lookup.
	Enabled bool `yaml:"enabled"`

	// DatabasePath is the path to a MaxMind GeoLite2-Country.mmdb file.
	DatabasePath string `yaml:"database_path"`
}

// StatsRetentionConf defines data retention periods.
type StatsRetentionConf struct {
	// DetailedDays is how many days to keep individual request logs.
	// Default: 90
	DetailedDays int `yaml:"detailed_days"`

	// AggregatedDays is how many days to keep daily aggregated statistics.
	// Default: 365
	AggregatedDays int `yaml:"aggregated_days"`
}

// validate checks the stats configuration for errors.
func (s *StatsConf) validate() error {
	if !s.Enabled {
		return nil
	}

	if s.Buffer.Size <= 0 {
		s.Buffer.Size = 10000
	}

	if s.Buffer.FlushInterval <= 0 {
		s.Buffer.FlushInterval = 5 * time.Second
	}

	if s.Buffer.FlushThreshold <= 0 || s.Buffer.FlushThreshold > 1 {
		s.Buffer.FlushThreshold = 0.8
	}

	if s.Retention.DetailedDays <= 0 {
		s.Retention.DetailedDays = 90
	}

	if s.Retention.AggregatedDays <= 0 {
		s.Retention.AggregatedDays = 365
	}

	if s.Capture.GeoIP.Enabled {
		if s.Capture.GeoIP.DatabasePath == "" {
			return errors.New("geo_ip.database_path is required when geo_ip.enabled is true")
		}
		if !fileutils.FileExists(s.Capture.GeoIP.DatabasePath) {
			return errors.Errorf("geo_ip database file does not exist: %s", s.Capture.GeoIP.DatabasePath)
		}
	}

	return nil
}

// DetailedRetention returns the retention period for detailed logs as a Duration.
func (s *StatsConf) DetailedRetention() time.Duration {
	return time.Duration(s.Retention.DetailedDays) * 24 * time.Hour
}

// AggregatedRetention returns the retention period for aggregated stats as a Duration.
func (s *StatsConf) AggregatedRetention() time.Duration {
	return time.Duration(s.Retention.AggregatedDays) * 24 * time.Hour
}

var defaultStatsConf = StatsConf{
	Enabled: false,
	Buffer: StatsBufferConf{
		Size:           10000,
		FlushInterval:  5 * time.Second,
		FlushThreshold: 0.8,
	},
	Capture: StatsCaptureConf{
		ClientIP:    true,
		UserAgent:   true,
		QueryParams: true,
		GeoIP: StatsGeoIPConf{
			Enabled: false,
		},
	},
	Retention: StatsRetentionConf{
		DetailedDays:   90,
		AggregatedDays: 365,
	},
	Endpoints: nil,
}
