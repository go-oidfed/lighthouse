package stats

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// RequestLog represents a single request to a federation endpoint.
// This is stored in the database for detailed analytics.
type RequestLog struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Timestamp time.Time `gorm:"index:idx_rl_ts;not null" json:"timestamp"`

	// Request info
	Endpoint   string `gorm:"size:100;index:idx_rl_endpoint;not null" json:"endpoint"`
	Method     string `gorm:"size:10;not null" json:"method"`
	StatusCode int    `gorm:"type:smallint;not null" json:"status_code"`
	DurationMs int    `gorm:"not null" json:"duration_ms"`

	// Client info
	ClientIP      string `gorm:"size:45;index:idx_rl_ip" json:"client_ip,omitempty"`
	CountryCode   string `gorm:"size:2" json:"country_code,omitempty"`
	UserAgent     string `gorm:"type:text" json:"user_agent,omitempty"`
	UserAgentHash uint32 `gorm:"index:idx_rl_ua_hash" json:"user_agent_hash,omitempty"`

	// Request details
	QueryParams  json.RawMessage `gorm:"type:json" json:"query_params,omitempty"`
	RequestSize  int             `gorm:"default:0" json:"request_size"`
	ResponseSize int             `gorm:"default:0" json:"response_size"`

	// Error info
	ErrorType string `gorm:"size:100" json:"error_type,omitempty"`
}

// TableName returns the table name for RequestLog.
func (RequestLog) TableName() string {
	return "federation_request_logs"
}

// DailyStats represents aggregated statistics for a single day.
// This is used for long-term retention with smaller storage footprint.
type DailyStats struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Primary key fields for grouping
	Date       time.Time `gorm:"type:date;uniqueIndex:idx_ds_date_endpoint_status;not null" json:"date"`
	Endpoint   string    `gorm:"size:100;uniqueIndex:idx_ds_date_endpoint_status;not null" json:"endpoint"`
	StatusCode int       `gorm:"type:smallint;uniqueIndex:idx_ds_date_endpoint_status;not null" json:"status_code"`

	// Counts
	RequestCount int64 `gorm:"not null;default:0" json:"request_count"`
	ErrorCount   int64 `gorm:"not null;default:0" json:"error_count"`

	// Duration statistics (in milliseconds)
	DurationP50Ms int `gorm:"default:0" json:"duration_p50_ms"`
	DurationP95Ms int `gorm:"default:0" json:"duration_p95_ms"`
	DurationP99Ms int `gorm:"default:0" json:"duration_p99_ms"`
	DurationAvgMs int `gorm:"default:0" json:"duration_avg_ms"`
	DurationMinMs int `gorm:"default:0" json:"duration_min_ms"`
	DurationMaxMs int `gorm:"default:0" json:"duration_max_ms"`

	// Top entries as JSON arrays
	TopUserAgents json.RawMessage `gorm:"type:json" json:"top_user_agents,omitempty"`
	TopCountries  json.RawMessage `gorm:"type:json" json:"top_countries,omitempty"`
	TopClientIPs  json.RawMessage `gorm:"type:json" json:"top_client_ips,omitempty"`
	TopParams     json.RawMessage `gorm:"type:json" json:"top_params,omitempty"`
}

// TableName returns the table name for DailyStats.
func (DailyStats) TableName() string {
	return "federation_daily_stats"
}

// BeforeCreate sets the date to midnight UTC.
func (d *DailyStats) BeforeCreate(_ *gorm.DB) error {
	d.Date = time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), 0, 0, 0, 0, time.UTC)
	return nil
}

// Summary represents an overall statistics summary.
type Summary struct {
	TotalRequests      int64            `json:"total_requests"`
	TotalErrors        int64            `json:"total_errors"`
	ErrorRate          float64          `json:"error_rate"`
	AvgLatencyMs       float64          `json:"avg_latency_ms"`
	P50LatencyMs       int              `json:"p50_latency_ms"`
	P95LatencyMs       int              `json:"p95_latency_ms"`
	P99LatencyMs       int              `json:"p99_latency_ms"`
	UniqueClients      int64            `json:"unique_clients"`
	UniqueUserAgents   int64            `json:"unique_user_agents"`
	RequestsByStatus   map[int]int64    `json:"requests_by_status"`
	RequestsByEndpoint map[string]int64 `json:"requests_by_endpoint"`
}

// TopEntry represents a single entry in a top-N list.
type TopEntry struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// TimeSeriesPoint represents a single data point in a time series.
type TimeSeriesPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	RequestCount int64     `json:"request_count"`
	ErrorCount   int64     `json:"error_count"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
}

// LatencyStats holds latency percentile statistics.
type LatencyStats struct {
	P50Ms int     `json:"p50_ms"`
	P75Ms int     `json:"p75_ms"`
	P90Ms int     `json:"p90_ms"`
	P95Ms int     `json:"p95_ms"`
	P99Ms int     `json:"p99_ms"`
	AvgMs float64 `json:"avg_ms"`
	MinMs int     `json:"min_ms"`
	MaxMs int     `json:"max_ms"`
}

// Interval represents a time interval for aggregation.
type Interval string

const (
	IntervalMinute Interval = "minute"
	IntervalHour   Interval = "hour"
	IntervalDay    Interval = "day"
	IntervalWeek   Interval = "week"
	IntervalMonth  Interval = "month"
)

// ParseInterval parses an interval string.
func ParseInterval(s string) Interval {
	switch s {
	case "minute":
		return IntervalMinute
	case "hour":
		return IntervalHour
	case "day":
		return IntervalDay
	case "week":
		return IntervalWeek
	case "month":
		return IntervalMonth
	default:
		return IntervalHour
	}
}
