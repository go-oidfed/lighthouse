package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/go-oidfed/lighthouse/internal/stats"
)

// newSQLiteStatsStorage creates an isolated file-backed SQLite StatsStorage with
// the RequestLog table migrated, for use in unit tests. Each test gets its own
// database file under t.TempDir() so tests never share state.
func newSQLiteStatsStorage(t *testing.T) *StatsStorage {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "stats_test.db")
	db, err := gorm.Open(
		sqlite.Open(dbPath),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().CreateTable(&stats.RequestLog{}))
	return NewStatsStorage(db)
}

// insertLogs inserts the given request logs and reports the number inserted.
func insertLogs(t *testing.T, s *StatsStorage, logs ...*stats.RequestLog) {
	t.Helper()
	for _, l := range logs {
		l.Timestamp = l.Timestamp.UTC()
	}
	require.NoError(t, s.InsertBatch(logs))
}

func TestGetTimeSeries_SqliteHourBuckets(t *testing.T) {
	s := newSQLiteStatsStorage(t)

	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	logs := []*stats.RequestLog{
		{Timestamp: base, Endpoint: "fetch", Method: "GET", StatusCode: 200, DurationMs: 10},
		{Timestamp: base.Add(5 * time.Minute), Endpoint: "fetch", Method: "GET", StatusCode: 200, DurationMs: 20},
		{Timestamp: base.Add(1 * time.Hour), Endpoint: "fetch", Method: "GET", StatusCode: 500, DurationMs: 30},
		{Timestamp: base.Add(1 * time.Hour).Add(2 * time.Minute), Endpoint: "fetch", Method: "GET", StatusCode: 200, DurationMs: 40},
	}
	insertLogs(t, s, logs...)

	from := base.Add(-time.Second)
	to := base.Add(2 * time.Hour)

	points, err := s.GetTimeSeries(from, to, "", stats.IntervalHour)
	require.NoError(t, err)
	require.Len(t, points, 2)

	// Buckets are returned ascending.
	assert.True(t, points[0].Timestamp.Equal(base))
	assert.Equal(t, int64(2), points[0].RequestCount)
	assert.Equal(t, int64(0), points[0].ErrorCount)

	assert.True(t, points[1].Timestamp.Equal(base.Add(1*time.Hour)))
	assert.Equal(t, int64(2), points[1].RequestCount)
	assert.Equal(t, int64(1), points[1].ErrorCount)
}

func TestGetTimeSeries_SqliteDayBuckets(t *testing.T) {
	s := newSQLiteStatsStorage(t)

	day1 := time.Date(2026, 7, 1, 13, 30, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 2, 2, 15, 0, 0, time.UTC)
	logs := []*stats.RequestLog{
		{Timestamp: day1, Endpoint: "resolve", Method: "GET", StatusCode: 200, DurationMs: 5},
		{Timestamp: day1.Add(3 * time.Hour), Endpoint: "resolve", Method: "GET", StatusCode: 404, DurationMs: 7},
		{Timestamp: day2, Endpoint: "resolve", Method: "GET", StatusCode: 200, DurationMs: 9},
	}
	insertLogs(t, s, logs...)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	points, err := s.GetTimeSeries(from, to, "", stats.IntervalDay)
	require.NoError(t, err)
	require.Len(t, points, 2)

	assert.True(t, points[0].Timestamp.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, int64(2), points[0].RequestCount)
	assert.Equal(t, int64(1), points[0].ErrorCount)

	assert.True(t, points[1].Timestamp.Equal(time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, int64(1), points[1].RequestCount)
	assert.Equal(t, int64(0), points[1].ErrorCount)
}

func TestGetTimeSeries_SqliteEndpointFilter(t *testing.T) {
	s := newSQLiteStatsStorage(t)

	ts := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	logs := []*stats.RequestLog{
		{Timestamp: ts, Endpoint: "fetch", Method: "GET", StatusCode: 200, DurationMs: 10},
		{Timestamp: ts, Endpoint: "resolve", Method: "GET", StatusCode: 200, DurationMs: 10},
		{Timestamp: ts, Endpoint: "fetch", Method: "GET", StatusCode: 500, DurationMs: 10},
	}
	insertLogs(t, s, logs...)

	points, err := s.GetTimeSeries(ts.Add(-time.Second), ts.Add(time.Hour), "fetch", stats.IntervalHour)
	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.Equal(t, int64(2), points[0].RequestCount)
	assert.Equal(t, int64(1), points[0].ErrorCount)
}

// TestGetDailyStats_SqliteRange verifies that GetDailyStats returns the rows
// stored in federation_daily_stats for a time range bounded by full RFC3339
// timestamps, the same kind of bounds the HTTP handler forwards from
// ?from=...&to=... query params. This guards against a regression where a
// date-bucket comparison silently matches zero rows.
func TestGetDailyStats_SqliteRange(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "stats_daily_test.db")
	db, err := gorm.Open(
		sqlite.Open(dbPath),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().CreateTable(&stats.DailyStats{}))
	s := NewStatsStorage(db)

	midnight := func(y, m, d int) time.Time {
		return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	}
	rows := []stats.DailyStats{
		{Date: midnight(2026, 7, 19), Endpoint: "fetch", StatusCode: 200, RequestCount: 2814},
		{Date: midnight(2026, 7, 20), Endpoint: "fetch", StatusCode: 200, RequestCount: 4830},
		{Date: midnight(2026, 7, 23), Endpoint: "fetch", StatusCode: 500, RequestCount: 12, ErrorCount: 12},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}

	from, err := time.Parse(time.RFC3339, "2026-07-01T00:00:00Z")
	require.NoError(t, err)
	to, err := time.Parse(time.RFC3339, "2026-07-23T23:59:59Z")
	require.NoError(t, err)

	got, err := s.GetDailyStats(from, to)
	require.NoError(t, err)
	require.Len(t, got, 3, "expected all 3 daily rows within the RFC3339-bounded range")

	// Out-of-range window matches nothing.
	gotEmpty, err := s.GetDailyStats(
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	assert.Empty(t, gotEmpty)
}

// TestAggregateAndBackfillDailyStats_Sqlite verifies the full aggregation
// pipeline that the stats aggregator now drives on startup: detailed request
// logs are rolled up into federation_daily_stats, HasDailyStatsForDate skips
// already-aggregated days, and GetDailyStats returns the rolled-up rows for an
// RFC3339-bounded range.
func TestAggregateAndBackfillDailyStats_Sqlite(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "stats_agg_test.db")
	db, err := gorm.Open(
		sqlite.Open(dbPath),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().CreateTable(&stats.RequestLog{}))
	require.NoError(t, db.Migrator().CreateTable(&stats.DailyStats{}))
	s := NewStatsStorage(db)

	day := func(y, m, d, hour int) time.Time {
		return time.Date(y, time.Month(m), d, hour, 0, 0, 0, time.UTC)
	}
	// Logs across two days; one day is pre-aggregated to exercise the skip path.
	insertLogs(t, s,
		&stats.RequestLog{Timestamp: day(2026, 7, 18, 10), Endpoint: "fetch", Method: "GET", StatusCode: 200, DurationMs: 5},
		&stats.RequestLog{Timestamp: day(2026, 7, 18, 22), Endpoint: "fetch", Method: "GET", StatusCode: 500, DurationMs: 7},
		&stats.RequestLog{Timestamp: day(2026, 7, 19, 1), Endpoint: "fetch", Method: "GET", StatusCode: 200, DurationMs: 9},
	)
	require.NoError(t, s.AggregateDailyStats(day(2026, 7, 18, 0)))

	exists18, err := s.HasDailyStatsForDate(day(2026, 7, 18, 0))
	require.NoError(t, err)
	require.True(t, exists18, "pre-aggregated day should report existing rows")
	exists19, err := s.HasDailyStatsForDate(day(2026, 7, 19, 0))
	require.NoError(t, err)
	require.False(t, exists19, "not-yet-aggregated day should report no rows")
	exists20, err := s.HasDailyStatsForDate(day(2026, 7, 20, 0))
	require.NoError(t, err)
	require.False(t, exists20, "day with no logs should report no rows")

	// Backfill the missing day via the aggregator.
	agg := stats.NewAggregator(s, 90*24*time.Hour, 365*24*time.Hour)
	agg.Backfill(context.Background())

	// After backfill, 07-19 has rows; 07-18 is unchanged (skipped, not overwritten).
	exists19After, err := s.HasDailyStatsForDate(day(2026, 7, 19, 0))
	require.NoError(t, err)
	require.True(t, exists19After, "backfilled day should now report existing rows")

	from, err := time.Parse(time.RFC3339, "2026-07-01T00:00:00Z")
	require.NoError(t, err)
	to, err := time.Parse(time.RFC3339, "2026-07-31T23:59:59Z")
	require.NoError(t, err)
	got, err := s.GetDailyStats(from, to)
	require.NoError(t, err)
	// 07-18: two endpoint/status combos; 07-19: one combo.
	require.Len(t, got, 3, "expected 3 daily rows across the two aggregated days")
}

func TestGetTimeSeries_SqliteEmptyRange(t *testing.T) {
	s := newSQLiteStatsStorage(t)
	insertLogs(t, s, &stats.RequestLog{
		Timestamp:  time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		Endpoint:   "fetch",
		Method:     "GET",
		StatusCode: 200,
		DurationMs: 10,
	})

	points, err := s.GetTimeSeries(
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		"", stats.IntervalHour,
	)
	require.NoError(t, err)
	assert.Empty(t, points)
}

func TestBucketScannerScan(t *testing.T) {
	t.Run("time", func(t *testing.T) {
		var b bucketScanner
		ts := time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC)
		require.NoError(t, b.Scan(ts))
		assert.True(t, b.Time.Equal(ts))
	})

	t.Run("string", func(t *testing.T) {
		cases := []struct {
			in   string
			want time.Time
		}{
			{"2026-07-01 10:00:00", time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)},
			{"2026-07-01", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
			{"2026-07-01T10:00:00Z", time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)},
		}
		for _, c := range cases {
			var b bucketScanner
			require.NoError(t, b.Scan(c.in))
			assert.True(t, b.Time.Equal(c.want), "input %q got %v want %v", c.in, b.Time, c.want)
		}
	})

	t.Run("bytes", func(t *testing.T) {
		var b bucketScanner
		require.NoError(t, b.Scan([]byte("2026-07-01 10:00:00")))
		assert.True(t, b.Time.Equal(time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)))
	})

	t.Run("nil", func(t *testing.T) {
		var b bucketScanner
		require.NoError(t, b.Scan(nil))
		assert.True(t, b.Time.IsZero())
	})

	t.Run("unsupported", func(t *testing.T) {
		var b bucketScanner
		err := b.Scan(42)
		require.Error(t, err)
	})

	t.Run("unparseable", func(t *testing.T) {
		var b bucketScanner
		err := b.Scan("not-a-date")
		require.Error(t, err)
	})
}

// TestGetTimeSeries_SqliteIntervalCoverage exercises every supported interval
// to ensure each truncation expression scans cleanly under SQLite.
func TestGetTimeSeries_SqliteIntervalCoverage(t *testing.T) {
	s := newSQLiteStatsStorage(t)

	ts := time.Date(2026, 7, 8, 10, 15, 0, 0, time.UTC)
	insertLogs(t, s, &stats.RequestLog{
		Timestamp:  ts,
		Endpoint:   "fetch",
		Method:     "GET",
		StatusCode: 200,
		DurationMs: 10,
	})

	intervals := []stats.Interval{
		stats.IntervalMinute,
		stats.IntervalHour,
		stats.IntervalDay,
		stats.IntervalWeek,
		stats.IntervalMonth,
	}

	for _, iv := range intervals {
		iv := iv
		t.Run(string(iv), func(t *testing.T) {
			points, err := s.GetTimeSeries(ts.Add(-time.Hour), ts.Add(time.Hour), "", iv)
			require.NoError(t, err)
			require.Len(t, points, 1)
			// Bucket must be at or before the event timestamp and in UTC.
			assert.True(t, points[0].Timestamp.Before(ts.Add(time.Second)) || points[0].Timestamp.Equal(ts),
				"bucket %v should be <= event %v for interval %s", points[0].Timestamp, ts, iv)
			assert.Equal(t, time.UTC, points[0].Timestamp.Location())
		})
	}
}
