package stats

import "time"

// Config holds configuration for the stats collector.
type Config struct {
	// Enabled controls whether statistics collection is active.
	Enabled bool

	// Buffer configuration
	BufferSize     int
	FlushInterval  time.Duration
	FlushThreshold float64

	// Capture options
	CaptureClientIP    bool
	CaptureUserAgent   bool
	CaptureQueryParams bool

	// GeoIP configuration
	GeoIPEnabled bool
	GeoIPDBPath  string

	// Retention
	DetailedRetention   time.Duration
	AggregatedRetention time.Duration

	// Endpoints to track (empty = all federation endpoints)
	Endpoints []string
}
