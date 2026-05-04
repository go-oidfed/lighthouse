package config

import (
	"time"

	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/fileutils"

	apistats "github.com/go-oidfed/lighthouse/api/stats"
)

// StatsConf holds all statistics collection configuration.
//
// Environment variables (with prefix LH_STATS_):
//   - LH_STATS_ENABLED: Enable statistics collection
//   - LH_STATS_ENDPOINTS: Endpoints to track (comma-separated)
//   - LH_STATS_BUFFER_SIZE: Ring buffer size
//   - LH_STATS_BUFFER_FLUSH_INTERVAL: Flush interval (e.g., "5s")
//   - LH_STATS_BUFFER_FLUSH_THRESHOLD: Flush threshold (0-1)
//   - LH_STATS_CAPTURE_CLIENT_IP: Capture client IP
//   - LH_STATS_CAPTURE_USER_AGENT: Capture User-Agent
//   - LH_STATS_CAPTURE_QUERY_PARAMS: Capture query parameters
//   - LH_STATS_CAPTURE_GEO_IP_ENABLED: Enable GeoIP lookup
//   - LH_STATS_CAPTURE_GEO_IP_DATABASE_PATH: Path to GeoLite2 database
//   - LH_STATS_RETENTION_DETAILED_DAYS: Days to keep detailed logs
//   - LH_STATS_RETENTION_AGGREGATED_DAYS: Days to keep aggregated stats
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
	// Env: LH_STATS_ENABLED
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`

	// Buffer configures the in-memory ring buffer for request logs.
	// Env prefix: LH_STATS_BUFFER_
	Buffer StatsBufferConf `yaml:"buffer" envconfig:"BUFFER"`

	// Capture controls what data is collected from each request.
	// Env prefix: LH_STATS_CAPTURE_
	Capture StatsCaptureConf `yaml:"capture" envconfig:"CAPTURE"`

	// Retention defines how long data is kept.
	// Env prefix: LH_STATS_RETENTION_
	Retention StatsRetentionConf `yaml:"retention" envconfig:"RETENTION"`

	// Endpoints is a list of endpoint paths to track.
	// If empty, all federation endpoints are tracked.
	// Example: ["/.well-known/openid-federation", "/fetch", "/resolve"]
	// Env: LH_STATS_ENDPOINTS (comma-separated)
	Endpoints []string `yaml:"endpoints" envconfig:"ENDPOINTS"`
}

// StatsBufferConf configures the in-memory ring buffer.
//
// Environment variables (with prefix LH_STATS_BUFFER_):
//   - LH_STATS_BUFFER_SIZE: Ring buffer size
//   - LH_STATS_BUFFER_FLUSH_INTERVAL: Flush interval (e.g., "5s")
//   - LH_STATS_BUFFER_FLUSH_THRESHOLD: Flush threshold (0-1)
type StatsBufferConf struct {
	// Size is the maximum number of entries in the ring buffer.
	// Default: 10000
	// Env: LH_STATS_BUFFER_SIZE
	Size int `yaml:"size" envconfig:"SIZE"`

	// FlushInterval is how often the buffer is flushed to the database.
	// Default: 5s
	// Env: LH_STATS_BUFFER_FLUSH_INTERVAL
	FlushInterval time.Duration `yaml:"flush_interval" envconfig:"FLUSH_INTERVAL"`

	// FlushThreshold triggers a flush when the buffer is this percentage full.
	// Value between 0 and 1. Default: 0.8
	// Env: LH_STATS_BUFFER_FLUSH_THRESHOLD
	FlushThreshold float64 `yaml:"flush_threshold" envconfig:"FLUSH_THRESHOLD"`
}

// StatsCaptureConf controls what request data is captured.
//
// Environment variables (with prefix LH_STATS_CAPTURE_):
//   - LH_STATS_CAPTURE_CLIENT_IP: Capture client IP
//   - LH_STATS_CAPTURE_USER_AGENT: Capture User-Agent
//   - LH_STATS_CAPTURE_QUERY_PARAMS: Capture query parameters
//   - LH_STATS_CAPTURE_GEO_IP_ENABLED: Enable GeoIP lookup
//   - LH_STATS_CAPTURE_GEO_IP_DATABASE_PATH: Path to GeoLite2 database
type StatsCaptureConf struct {
	// ClientIP records the client's IP address.
	// Env: LH_STATS_CAPTURE_CLIENT_IP
	ClientIP bool `yaml:"client_ip" envconfig:"CLIENT_IP"`

	// UserAgent records the User-Agent header.
	// Env: LH_STATS_CAPTURE_USER_AGENT
	UserAgent bool `yaml:"user_agent" envconfig:"USER_AGENT"`

	// QueryParams records URL query parameters as JSON.
	// Env: LH_STATS_CAPTURE_QUERY_PARAMS
	QueryParams bool `yaml:"query_params" envconfig:"QUERY_PARAMS"`

	// GeoIP enables country lookup from IP addresses.
	// Env prefix: LH_STATS_CAPTURE_GEO_IP_
	GeoIP StatsGeoIPConf `yaml:"geo_ip" envconfig:"GEO_IP"`
}

// StatsGeoIPConf configures GeoIP lookup.
//
// Environment variables (with prefix LH_STATS_CAPTURE_GEO_IP_):
//   - LH_STATS_CAPTURE_GEO_IP_ENABLED: Enable GeoIP lookup
//   - LH_STATS_CAPTURE_GEO_IP_DATABASE_PATH: Path to GeoLite2 database
type StatsGeoIPConf struct {
	// Enabled turns on GeoIP country lookup.
	// Env: LH_STATS_CAPTURE_GEO_IP_ENABLED
	Enabled bool `yaml:"enabled" envconfig:"ENABLED"`

	// DatabasePath is the path to a MaxMind GeoLite2-Country.mmdb file.
	// Env: LH_STATS_CAPTURE_GEO_IP_DATABASE_PATH
	DatabasePath string `yaml:"database_path" envconfig:"DATABASE_PATH"`
}

// StatsRetentionConf defines data retention periods.
//
// Environment variables (with prefix LH_STATS_RETENTION_):
//   - LH_STATS_RETENTION_DETAILED_DAYS: Days to keep detailed logs
//   - LH_STATS_RETENTION_AGGREGATED_DAYS: Days to keep aggregated stats
type StatsRetentionConf struct {
	// DetailedDays is how many days to keep individual request logs.
	// Default: 90
	// Env: LH_STATS_RETENTION_DETAILED_DAYS
	DetailedDays int `yaml:"detailed_days" envconfig:"DETAILED_DAYS"`

	// AggregatedDays is how many days to keep daily aggregated statistics.
	// Default: 365
	// Env: LH_STATS_RETENTION_AGGREGATED_DAYS
	AggregatedDays int `yaml:"aggregated_days" envconfig:"AGGREGATED_DAYS"`
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

// ToAPIConfig converts config.StatsConf to api/stats.Config.
func (s *StatsConf) ToAPIConfig() apistats.Config {
	return apistats.Config{
		Enabled:             s.Enabled,
		BufferSize:          s.Buffer.Size,
		FlushInterval:       s.Buffer.FlushInterval,
		FlushThreshold:      s.Buffer.FlushThreshold,
		CaptureClientIP:     s.Capture.ClientIP,
		CaptureUserAgent:    s.Capture.UserAgent,
		CaptureQueryParams:  s.Capture.QueryParams,
		GeoIPEnabled:        s.Capture.GeoIP.Enabled,
		GeoIPDBPath:         s.Capture.GeoIP.DatabasePath,
		DetailedRetention:   s.DetailedRetention(),
		AggregatedRetention: s.AggregatedRetention(),
		Endpoints:           s.Endpoints,
	}
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
