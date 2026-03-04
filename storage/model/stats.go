package model

import (
	"io"
	"time"

	"github.com/go-oidfed/lighthouse/internal/stats"
)

// StatsStorageBackend defines the interface for statistics storage operations.
type StatsStorageBackend interface {
	// InsertBatch inserts multiple request logs in a single batch operation.
	InsertBatch(entries []*stats.RequestLog) error

	// Summary queries
	GetSummary(from, to time.Time) (*stats.Summary, error)

	// Top-N queries
	GetTopEndpoints(from, to time.Time, limit int) ([]stats.TopEntry, error)
	GetTopUserAgents(from, to time.Time, limit int) ([]stats.TopEntry, error)
	GetTopClients(from, to time.Time, limit int) ([]stats.TopEntry, error)
	GetTopCountries(from, to time.Time, limit int) ([]stats.TopEntry, error)
	GetTopQueryParams(from, to time.Time, endpoint string, limit int) ([]stats.TopEntry, error)

	// Time series queries
	GetTimeSeries(from, to time.Time, endpoint string, interval stats.Interval) ([]stats.TimeSeriesPoint, error)
	GetLatencyPercentiles(from, to time.Time, endpoint string) (*stats.LatencyStats, error)

	// Daily aggregation
	AggregateDailyStats(date time.Time) error
	GetDailyStats(from, to time.Time) ([]stats.DailyStats, error)

	// Maintenance
	PurgeDetailedLogs(before time.Time) (int64, error)
	PurgeAggregatedStats(before time.Time) (int64, error)

	// Export
	ExportCSV(from, to time.Time, w io.Writer) error
	ExportJSON(from, to time.Time, w io.Writer) error
}
