package storage

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/internal/stats"
)

// StatsStorage implements the StatsStorageBackend interface using GORM.
type StatsStorage struct {
	db     *gorm.DB
	driver string
}

// NewStatsStorage creates a new stats storage instance.
func NewStatsStorage(db *gorm.DB) *StatsStorage {
	driver := db.Dialector.Name()
	return &StatsStorage{
		db:     db,
		driver: driver,
	}
}

// InsertBatch inserts multiple request logs in a single batch operation.
func (s *StatsStorage) InsertBatch(entries []*stats.RequestLog) error {
	if len(entries) == 0 {
		return nil
	}

	// Use batch size based on driver
	batchSize := 500
	if s.driver == "sqlite" {
		batchSize = 100 // SQLite has lower limits
	}

	return s.db.CreateInBatches(entries, batchSize).Error
}

// GetSummary returns overall statistics for the given time range.
func (s *StatsStorage) GetSummary(from, to time.Time) (*stats.Summary, error) {
	var summary stats.Summary

	// Get total counts
	var result struct {
		TotalRequests int64
		TotalErrors   int64
		AvgLatency    float64
	}

	err := s.db.Model(&stats.RequestLog{}).
		Select("COUNT(*) as total_requests, "+
			"SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as total_errors, "+
			"AVG(duration_ms) as avg_latency").
		Where("timestamp BETWEEN ? AND ?", from, to).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}

	summary.TotalRequests = result.TotalRequests
	summary.TotalErrors = result.TotalErrors
	summary.AvgLatencyMs = result.AvgLatency

	if summary.TotalRequests > 0 {
		summary.ErrorRate = float64(summary.TotalErrors) / float64(summary.TotalRequests)
	}

	// Get unique counts
	var uniqueCounts struct {
		UniqueClients    int64
		UniqueUserAgents int64
	}
	err = s.db.Model(&stats.RequestLog{}).
		Select("COUNT(DISTINCT client_ip) as unique_clients, "+
			"COUNT(DISTINCT user_agent_hash) as unique_user_agents").
		Where("timestamp BETWEEN ? AND ?", from, to).
		Scan(&uniqueCounts).Error
	if err != nil {
		return nil, err
	}
	summary.UniqueClients = uniqueCounts.UniqueClients
	summary.UniqueUserAgents = uniqueCounts.UniqueUserAgents

	// Get latency percentiles
	latency, err := s.GetLatencyPercentiles(from, to, "")
	if err == nil && latency != nil {
		summary.P50LatencyMs = latency.P50Ms
		summary.P95LatencyMs = latency.P95Ms
		summary.P99LatencyMs = latency.P99Ms
	}

	// Get requests by status
	var statusCounts []struct {
		StatusCode int
		Count      int64
	}
	err = s.db.Model(&stats.RequestLog{}).
		Select("status_code, COUNT(*) as count").
		Where("timestamp BETWEEN ? AND ?", from, to).
		Group("status_code").
		Scan(&statusCounts).Error
	if err != nil {
		return nil, err
	}

	summary.RequestsByStatus = make(map[int]int64)
	for _, sc := range statusCounts {
		summary.RequestsByStatus[sc.StatusCode] = sc.Count
	}

	// Get requests by endpoint
	var endpointCounts []struct {
		Endpoint string
		Count    int64
	}
	err = s.db.Model(&stats.RequestLog{}).
		Select("endpoint, COUNT(*) as count").
		Where("timestamp BETWEEN ? AND ?", from, to).
		Group("endpoint").
		Scan(&endpointCounts).Error
	if err != nil {
		return nil, err
	}

	summary.RequestsByEndpoint = make(map[string]int64)
	for _, ec := range endpointCounts {
		summary.RequestsByEndpoint[ec.Endpoint] = ec.Count
	}

	return &summary, nil
}

// GetTopEndpoints returns the top endpoints by request count.
func (s *StatsStorage) GetTopEndpoints(from, to time.Time, limit int) ([]stats.TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []struct {
		Endpoint string
		Count    int64
	}

	err := s.db.Model(&stats.RequestLog{}).
		Select("endpoint, COUNT(*) as count").
		Where("timestamp BETWEEN ? AND ?", from, to).
		Group("endpoint").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.Endpoint, Count: r.Count}
	}
	return entries, nil
}

// GetTopUserAgents returns the top user agents by request count.
func (s *StatsStorage) GetTopUserAgents(from, to time.Time, limit int) ([]stats.TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []struct {
		UserAgent string
		Count     int64
	}

	err := s.db.Model(&stats.RequestLog{}).
		Select("user_agent, COUNT(*) as count").
		Where("timestamp BETWEEN ? AND ? AND user_agent != ''", from, to).
		Group("user_agent").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.UserAgent, Count: r.Count}
	}
	return entries, nil
}

// GetTopClients returns the top client IPs by request count.
func (s *StatsStorage) GetTopClients(from, to time.Time, limit int) ([]stats.TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []struct {
		ClientIP string
		Count    int64
	}

	err := s.db.Model(&stats.RequestLog{}).
		Select("client_ip, COUNT(*) as count").
		Where("timestamp BETWEEN ? AND ? AND client_ip != ''", from, to).
		Group("client_ip").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.ClientIP, Count: r.Count}
	}
	return entries, nil
}

// GetTopCountries returns the top countries by request count.
func (s *StatsStorage) GetTopCountries(from, to time.Time, limit int) ([]stats.TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []struct {
		CountryCode string
		Count       int64
	}

	err := s.db.Model(&stats.RequestLog{}).
		Select("country_code, COUNT(*) as count").
		Where("timestamp BETWEEN ? AND ? AND country_code != ''", from, to).
		Group("country_code").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.CountryCode, Count: r.Count}
	}
	return entries, nil
}

// GetTopQueryParams returns the top query parameter values for an endpoint.
func (s *StatsStorage) GetTopQueryParams(from, to time.Time, endpoint string, limit int) ([]stats.TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	// This requires JSON parsing which varies by database
	// For simplicity, we'll fetch raw data and process in Go
	var logs []stats.RequestLog

	query := s.db.Model(&stats.RequestLog{}).
		Select("query_params").
		Where("timestamp BETWEEN ? AND ? AND query_params IS NOT NULL", from, to)

	if endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}

	// Limit to a reasonable sample to avoid memory issues
	err := query.Limit(10000).Find(&logs).Error
	if err != nil {
		return nil, err
	}

	// Count parameter combinations
	paramCounts := make(map[string]int64)
	for _, log := range logs {
		if log.QueryParams != nil {
			paramStr := string(log.QueryParams)
			paramCounts[paramStr]++
		}
	}

	// Convert to sorted slice
	type paramCount struct {
		params string
		count  int64
	}
	var counts []paramCount
	for p, c := range paramCounts {
		counts = append(counts, paramCount{p, c})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Take top N
	if len(counts) > limit {
		counts = counts[:limit]
	}

	entries := make([]stats.TopEntry, len(counts))
	for i, c := range counts {
		entries[i] = stats.TopEntry{Value: c.params, Count: c.count}
	}
	return entries, nil
}

// GetTimeSeries returns time series data for the given time range.
func (s *StatsStorage) GetTimeSeries(from, to time.Time, endpoint string, interval stats.Interval) ([]stats.TimeSeriesPoint, error) {
	// Build the date truncation based on driver and interval
	var truncExpr string
	switch s.driver {
	case "postgres":
		truncExpr = s.postgresDateTrunc(interval)
	case "mysql":
		truncExpr = s.mysqlDateTrunc(interval)
	default: // sqlite
		truncExpr = s.sqliteDateTrunc(interval)
	}

	var results []struct {
		Bucket       time.Time
		RequestCount int64
		ErrorCount   int64
		AvgLatency   float64
	}

	query := s.db.Model(&stats.RequestLog{}).
		Select(truncExpr+" as bucket, "+
			"COUNT(*) as request_count, "+
			"SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count, "+
			"AVG(duration_ms) as avg_latency").
		Where("timestamp BETWEEN ? AND ?", from, to)

	if endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}

	err := query.Group("bucket").
		Order("bucket ASC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	points := make([]stats.TimeSeriesPoint, len(results))
	for i, r := range results {
		points[i] = stats.TimeSeriesPoint{
			Timestamp:    r.Bucket,
			RequestCount: r.RequestCount,
			ErrorCount:   r.ErrorCount,
			AvgLatencyMs: r.AvgLatency,
		}
	}
	return points, nil
}

// Date truncation helpers for different databases
func (*StatsStorage) postgresDateTrunc(interval stats.Interval) string {
	switch interval {
	case stats.IntervalMinute:
		return "date_trunc('minute', timestamp)"
	case stats.IntervalHour:
		return "date_trunc('hour', timestamp)"
	case stats.IntervalDay:
		return "date_trunc('day', timestamp)"
	case stats.IntervalWeek:
		return "date_trunc('week', timestamp)"
	case stats.IntervalMonth:
		return "date_trunc('month', timestamp)"
	default:
		return "date_trunc('hour', timestamp)"
	}
}

func (*StatsStorage) mysqlDateTrunc(interval stats.Interval) string {
	switch interval {
	case stats.IntervalMinute:
		return "DATE_FORMAT(timestamp, '%Y-%m-%d %H:%i:00')"
	case stats.IntervalHour:
		return "DATE_FORMAT(timestamp, '%Y-%m-%d %H:00:00')"
	case stats.IntervalDay:
		return "DATE(timestamp)"
	case stats.IntervalWeek:
		return "DATE(DATE_SUB(timestamp, INTERVAL WEEKDAY(timestamp) DAY))"
	case stats.IntervalMonth:
		return "DATE_FORMAT(timestamp, '%Y-%m-01')"
	default:
		return "DATE_FORMAT(timestamp, '%Y-%m-%d %H:00:00')"
	}
}

func (*StatsStorage) sqliteDateTrunc(interval stats.Interval) string {
	switch interval {
	case stats.IntervalMinute:
		return "strftime('%Y-%m-%d %H:%M:00', timestamp)"
	case stats.IntervalHour:
		return "strftime('%Y-%m-%d %H:00:00', timestamp)"
	case stats.IntervalDay:
		return "date(timestamp)"
	case stats.IntervalWeek:
		return "date(timestamp, 'weekday 0', '-7 days')"
	case stats.IntervalMonth:
		return "strftime('%Y-%m-01', timestamp)"
	default:
		return "strftime('%Y-%m-%d %H:00:00', timestamp)"
	}
}

// GetLatencyPercentiles calculates latency percentiles for the given time range.
func (s *StatsStorage) GetLatencyPercentiles(from, to time.Time, endpoint string) (*stats.LatencyStats, error) {
	// Fetch all durations and calculate percentiles in Go
	// This is more portable across databases than using DB-specific percentile functions
	var durations []int

	query := s.db.Model(&stats.RequestLog{}).
		Select("duration_ms").
		Where("timestamp BETWEEN ? AND ?", from, to)

	if endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}

	// Limit to reasonable sample
	err := query.Limit(100000).Pluck("duration_ms", &durations).Error
	if err != nil {
		return nil, err
	}

	if len(durations) == 0 {
		return &stats.LatencyStats{}, nil
	}

	// Sort for percentile calculation
	sort.Ints(durations)

	result := &stats.LatencyStats{
		MinMs: durations[0],
		MaxMs: durations[len(durations)-1],
		P50Ms: percentile(durations, 50),
		P75Ms: percentile(durations, 75),
		P90Ms: percentile(durations, 90),
		P95Ms: percentile(durations, 95),
		P99Ms: percentile(durations, 99),
	}

	// Calculate average
	var sum int64
	for _, d := range durations {
		sum += int64(d)
	}
	result.AvgMs = float64(sum) / float64(len(durations))

	return result, nil
}

// percentile calculates the p-th percentile of a sorted slice.
func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// AggregateDailyStats aggregates detailed logs into daily statistics.
func (s *StatsStorage) AggregateDailyStats(date time.Time) error {
	// Normalize date to midnight UTC
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endDate := date.Add(24 * time.Hour)

	// Get unique endpoint/status combinations for the day
	var combinations []struct {
		Endpoint   string
		StatusCode int
	}

	err := s.db.Model(&stats.RequestLog{}).
		Select("DISTINCT endpoint, status_code").
		Where("timestamp >= ? AND timestamp < ?", date, endDate).
		Scan(&combinations).Error
	if err != nil {
		return err
	}

	// For each combination, calculate and store aggregates
	for _, combo := range combinations {
		daily := stats.DailyStats{
			Date:       date,
			Endpoint:   combo.Endpoint,
			StatusCode: combo.StatusCode,
		}

		// Get counts
		var counts struct {
			RequestCount int64
			ErrorCount   int64
		}
		err = s.db.Model(&stats.RequestLog{}).
			Select("COUNT(*) as request_count, "+
				"SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count").
			Where("timestamp >= ? AND timestamp < ? AND endpoint = ? AND status_code = ?",
				date, endDate, combo.Endpoint, combo.StatusCode).
			Scan(&counts).Error
		if err != nil {
			return err
		}
		daily.RequestCount = counts.RequestCount
		daily.ErrorCount = counts.ErrorCount

		// Get latency stats
		latency, err := s.getLatencyForEndpointStatus(date, endDate, combo.Endpoint, combo.StatusCode)
		if err == nil && latency != nil {
			daily.DurationP50Ms = latency.P50Ms
			daily.DurationP95Ms = latency.P95Ms
			daily.DurationP99Ms = latency.P99Ms
			daily.DurationAvgMs = int(latency.AvgMs)
			daily.DurationMinMs = latency.MinMs
			daily.DurationMaxMs = latency.MaxMs
		}

		// Get top user agents
		topUA, _ := s.getTopUserAgentsForDay(date, endDate, combo.Endpoint, 5)
		if topUA != nil {
			daily.TopUserAgents, _ = json.Marshal(topUA)
		}

		// Get top countries
		topCountries, _ := s.getTopCountriesForDay(date, endDate, combo.Endpoint, 5)
		if topCountries != nil {
			daily.TopCountries, _ = json.Marshal(topCountries)
		}

		// Get top client IPs
		topIPs, _ := s.getTopClientsForDay(date, endDate, combo.Endpoint, 5)
		if topIPs != nil {
			daily.TopClientIPs, _ = json.Marshal(topIPs)
		}

		// Upsert the daily stats
		err = s.db.Where(stats.DailyStats{
			Date:       date,
			Endpoint:   combo.Endpoint,
			StatusCode: combo.StatusCode,
		}).Assign(daily).FirstOrCreate(&daily).Error
		if err != nil {
			return err
		}
	}

	return nil
}

// Helper methods for aggregation
func (s *StatsStorage) getLatencyForEndpointStatus(from, to time.Time, endpoint string, statusCode int) (*stats.LatencyStats, error) {
	var durations []int
	err := s.db.Model(&stats.RequestLog{}).
		Select("duration_ms").
		Where("timestamp >= ? AND timestamp < ? AND endpoint = ? AND status_code = ?",
			from, to, endpoint, statusCode).
		Limit(10000).
		Pluck("duration_ms", &durations).Error
	if err != nil || len(durations) == 0 {
		return nil, err
	}

	sort.Ints(durations)
	var sum int64
	for _, d := range durations {
		sum += int64(d)
	}

	return &stats.LatencyStats{
		MinMs: durations[0],
		MaxMs: durations[len(durations)-1],
		P50Ms: percentile(durations, 50),
		P95Ms: percentile(durations, 95),
		P99Ms: percentile(durations, 99),
		AvgMs: float64(sum) / float64(len(durations)),
	}, nil
}

func (s *StatsStorage) getTopUserAgentsForDay(from, to time.Time, endpoint string, limit int) ([]stats.TopEntry, error) {
	var results []struct {
		UserAgent string
		Count     int64
	}
	err := s.db.Model(&stats.RequestLog{}).
		Select("user_agent, COUNT(*) as count").
		Where("timestamp >= ? AND timestamp < ? AND endpoint = ? AND user_agent != ''",
			from, to, endpoint).
		Group("user_agent").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.UserAgent, Count: r.Count}
	}
	return entries, nil
}

func (s *StatsStorage) getTopCountriesForDay(from, to time.Time, endpoint string, limit int) ([]stats.TopEntry, error) {
	var results []struct {
		CountryCode string
		Count       int64
	}
	err := s.db.Model(&stats.RequestLog{}).
		Select("country_code, COUNT(*) as count").
		Where("timestamp >= ? AND timestamp < ? AND endpoint = ? AND country_code != ''",
			from, to, endpoint).
		Group("country_code").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.CountryCode, Count: r.Count}
	}
	return entries, nil
}

func (s *StatsStorage) getTopClientsForDay(from, to time.Time, endpoint string, limit int) ([]stats.TopEntry, error) {
	var results []struct {
		ClientIP string
		Count    int64
	}
	err := s.db.Model(&stats.RequestLog{}).
		Select("client_ip, COUNT(*) as count").
		Where("timestamp >= ? AND timestamp < ? AND endpoint = ? AND client_ip != ''",
			from, to, endpoint).
		Group("client_ip").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	entries := make([]stats.TopEntry, len(results))
	for i, r := range results {
		entries[i] = stats.TopEntry{Value: r.ClientIP, Count: r.Count}
	}
	return entries, nil
}

// GetDailyStats returns aggregated daily statistics for the given time range.
func (s *StatsStorage) GetDailyStats(from, to time.Time) ([]stats.DailyStats, error) {
	var results []stats.DailyStats
	err := s.db.Where("date >= ? AND date <= ?", from, to).
		Order("date DESC").
		Find(&results).Error
	return results, err
}

// PurgeDetailedLogs deletes request logs older than the given time.
func (s *StatsStorage) PurgeDetailedLogs(before time.Time) (int64, error) {
	result := s.db.Where("timestamp < ?", before).Delete(&stats.RequestLog{})
	return result.RowsAffected, result.Error
}

// PurgeAggregatedStats deletes daily stats older than the given time.
func (s *StatsStorage) PurgeAggregatedStats(before time.Time) (int64, error) {
	result := s.db.Where("date < ?", before).Delete(&stats.DailyStats{})
	return result.RowsAffected, result.Error
}

// ExportCSV exports request logs to CSV format.
func (s *StatsStorage) ExportCSV(from, to time.Time, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{
		"timestamp", "endpoint", "method", "status_code", "duration_ms",
		"client_ip", "country_code", "user_agent", "query_params",
		"request_size", "response_size", "error_type",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Stream rows
	rows, err := s.db.Model(&stats.RequestLog{}).
		Where("timestamp BETWEEN ? AND ?", from, to).
		Order("timestamp ASC").
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var log stats.RequestLog
		if err := s.db.ScanRows(rows, &log); err != nil {
			return err
		}

		record := []string{
			log.Timestamp.Format(time.RFC3339),
			log.Endpoint,
			log.Method,
			fmt.Sprintf("%d", log.StatusCode),
			fmt.Sprintf("%d", log.DurationMs),
			log.ClientIP,
			log.CountryCode,
			log.UserAgent,
			string(log.QueryParams),
			fmt.Sprintf("%d", log.RequestSize),
			fmt.Sprintf("%d", log.ResponseSize),
			log.ErrorType,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return rows.Err()
}

// ExportJSON exports request logs to JSON format (newline-delimited JSON).
func (s *StatsStorage) ExportJSON(from, to time.Time, w io.Writer) error {
	encoder := json.NewEncoder(w)

	rows, err := s.db.Model(&stats.RequestLog{}).
		Where("timestamp BETWEEN ? AND ?", from, to).
		Order("timestamp ASC").
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var log stats.RequestLog
		if err := s.db.ScanRows(rows, &log); err != nil {
			return err
		}
		if err := encoder.Encode(log); err != nil {
			return err
		}
	}

	return rows.Err()
}
