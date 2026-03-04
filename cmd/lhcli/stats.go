package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
	"github.com/go-oidfed/lighthouse/internal/stats"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

var statsStorage model.StatsStorageBackend

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View and manage statistics",
	Long:  `View and manage statistics for federation endpoints`,
}

var statsSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show statistics summary",
	Long:  `Show an overall summary of request statistics`,
	RunE:  showStatsSummary,
}

var statsTopCmd = &cobra.Command{
	Use:   "top",
	Short: "Show top statistics",
	Long:  `Show top endpoints, user agents, clients, etc.`,
}

var statsTopEndpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Show top endpoints by request count",
	RunE:  showTopEndpoints,
}

var statsTopUserAgentsCmd = &cobra.Command{
	Use:   "user-agents",
	Short: "Show top user agents by request count",
	RunE:  showTopUserAgents,
}

var statsTopClientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "Show top clients by request count",
	RunE:  showTopClients,
}

var statsTopCountriesCmd = &cobra.Command{
	Use:   "countries",
	Short: "Show top countries by request count",
	RunE:  showTopCountries,
}

var statsTimeseriesCmd = &cobra.Command{
	Use:   "timeseries",
	Short: "Show time series data",
	Long:  `Show time series data for request counts`,
	RunE:  showTimeseries,
}

var statsLatencyCmd = &cobra.Command{
	Use:   "latency",
	Short: "Show latency percentiles",
	RunE:  showLatency,
}

var statsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export statistics to file",
	Long:  `Export statistics data to CSV or JSON format`,
	RunE:  exportStats,
}

var statsPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge old statistics data",
	Long:  `Delete statistics data older than the retention period`,
	RunE:  purgeStats,
}

var statsAggregateCmd = &cobra.Command{
	Use:   "aggregate",
	Short: "Run daily aggregation manually",
	Long:  `Aggregate detailed logs into daily statistics for a specific date`,
	RunE:  aggregateStats,
}

// Flags
var (
	statsFromDate     string
	statsToDate       string
	statsLimit        int
	statsEndpoint     string
	statsInterval     string
	statsFormat       string
	statsOutput       string
	statsPurgeDryRun  bool
	statsAggregateDay string
)

func init() {
	// Global flags for stats commands
	statsCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "the config file to use")
	statsCmd.PersistentFlags().StringVar(&statsFromDate, "from", "", "start date (YYYY-MM-DD or RFC3339)")
	statsCmd.PersistentFlags().StringVar(&statsToDate, "to", "", "end date (YYYY-MM-DD or RFC3339)")

	// Limit flag for top commands
	statsTopCmd.PersistentFlags().IntVar(&statsLimit, "limit", 10, "number of results to show")

	// Timeseries flags
	statsTimeseriesCmd.Flags().StringVar(&statsEndpoint, "endpoint", "", "filter by endpoint")
	statsTimeseriesCmd.Flags().StringVar(&statsInterval, "interval", "hour", "time interval (minute, hour, day, week, month)")

	// Latency flags
	statsLatencyCmd.Flags().StringVar(&statsEndpoint, "endpoint", "", "filter by endpoint")

	// Export flags
	statsExportCmd.Flags().StringVar(&statsFormat, "format", "csv", "export format (csv or json)")
	statsExportCmd.Flags().StringVarP(&statsOutput, "output", "o", "", "output file (default: stdout)")

	// Purge flags
	statsPurgeCmd.Flags().BoolVar(&statsPurgeDryRun, "dry-run", false, "show what would be purged without deleting")

	// Aggregate flags
	statsAggregateCmd.Flags().StringVar(&statsAggregateDay, "date", "", "date to aggregate (YYYY-MM-DD, default: yesterday)")

	// Build command tree
	statsTopCmd.AddCommand(statsTopEndpointsCmd)
	statsTopCmd.AddCommand(statsTopUserAgentsCmd)
	statsTopCmd.AddCommand(statsTopClientsCmd)
	statsTopCmd.AddCommand(statsTopCountriesCmd)

	statsCmd.AddCommand(statsSummaryCmd)
	statsCmd.AddCommand(statsTopCmd)
	statsCmd.AddCommand(statsTimeseriesCmd)
	statsCmd.AddCommand(statsLatencyCmd)
	statsCmd.AddCommand(statsExportCmd)
	statsCmd.AddCommand(statsPurgeCmd)
	statsCmd.AddCommand(statsAggregateCmd)

	rootCmd.AddCommand(statsCmd)
}

func loadStatsStorage() error {
	if err := loadConfig(); err != nil {
		return err
	}

	c := config.Get()
	if !c.Stats.Enabled {
		return errors.New("statistics collection is not enabled in configuration")
	}

	backs, err := storage.LoadStorageBackends(
		storage.Config{
			Driver:  c.Storage.Driver,
			DSN:     c.Storage.DSN,
			DataDir: c.Storage.DataDir,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to load storage backends")
	}

	statsStorage = backs.Stats
	if statsStorage == nil {
		return errors.New("stats storage backend not available")
	}

	return nil
}

func parseStatsTimeRange() (from, to time.Time, err error) {
	now := time.Now().UTC()
	to = now
	from = now.Add(-24 * time.Hour)

	if statsFromDate != "" {
		from, err = parseDate(statsFromDate)
		if err != nil {
			return from, to, errors.Wrap(err, "invalid --from date")
		}
	}

	if statsToDate != "" {
		to, err = parseDate(statsToDate)
		if err != nil {
			return from, to, errors.Wrap(err, "invalid --to date")
		}
		// If only date (no time), set to end of day
		if len(statsToDate) == 10 {
			to = to.Add(24*time.Hour - time.Second)
		}
	}

	return from, to, nil
}

func parseDate(s string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, errors.Errorf("cannot parse date: %s", s)
}

func showStatsSummary(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	summary, err := statsStorage.GetSummary(from, to)
	if err != nil {
		return errors.Wrap(err, "failed to get summary")
	}

	fmt.Printf("Statistics Summary (%s to %s)\n", from.Format("2006-01-02"), to.Format("2006-01-02"))
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total Requests:      %d\n", summary.TotalRequests)
	fmt.Printf("Total Errors:        %d\n", summary.TotalErrors)
	fmt.Printf("Error Rate:          %.2f%%\n", summary.ErrorRate*100)
	fmt.Printf("Avg Latency:         %.2f ms\n", summary.AvgLatencyMs)
	fmt.Printf("P50 Latency:         %d ms\n", summary.P50LatencyMs)
	fmt.Printf("P95 Latency:         %d ms\n", summary.P95LatencyMs)
	fmt.Printf("P99 Latency:         %d ms\n", summary.P99LatencyMs)
	fmt.Printf("Unique Clients:      %d\n", summary.UniqueClients)
	fmt.Printf("Unique User Agents:  %d\n", summary.UniqueUserAgents)

	if len(summary.RequestsByEndpoint) > 0 {
		fmt.Println("\nRequests by Endpoint:")
		for endpoint, count := range summary.RequestsByEndpoint {
			fmt.Printf("  %-20s %d\n", endpoint, count)
		}
	}

	if len(summary.RequestsByStatus) > 0 {
		fmt.Println("\nRequests by Status:")
		for status, count := range summary.RequestsByStatus {
			fmt.Printf("  %-10d %d\n", status, count)
		}
	}

	return nil
}

func showTopEndpoints(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	entries, err := statsStorage.GetTopEndpoints(from, to, statsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top endpoints")
	}

	fmt.Printf("Top %d Endpoints (%s to %s)\n", statsLimit, from.Format("2006-01-02"), to.Format("2006-01-02"))
	printTopEntries(entries)
	return nil
}

func showTopUserAgents(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	entries, err := statsStorage.GetTopUserAgents(from, to, statsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top user agents")
	}

	fmt.Printf("Top %d User Agents (%s to %s)\n", statsLimit, from.Format("2006-01-02"), to.Format("2006-01-02"))
	printTopEntries(entries)
	return nil
}

func showTopClients(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	entries, err := statsStorage.GetTopClients(from, to, statsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top clients")
	}

	fmt.Printf("Top %d Clients (%s to %s)\n", statsLimit, from.Format("2006-01-02"), to.Format("2006-01-02"))
	printTopEntries(entries)
	return nil
}

func showTopCountries(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	entries, err := statsStorage.GetTopCountries(from, to, statsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top countries")
	}

	fmt.Printf("Top %d Countries (%s to %s)\n", statsLimit, from.Format("2006-01-02"), to.Format("2006-01-02"))
	printTopEntries(entries)
	return nil
}

func printTopEntries(entries []stats.TopEntry) {
	if len(entries) == 0 {
		fmt.Println("No data available")
		return
	}

	maxLen := 0
	for _, e := range entries {
		if len(e.Value) > maxLen {
			maxLen = len(e.Value)
		}
	}
	if maxLen > 60 {
		maxLen = 60
	}

	for i, e := range entries {
		value := e.Value
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		fmt.Printf("%3d. %-*s %d\n", i+1, maxLen, value, e.Count)
	}
}

func showTimeseries(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	interval := stats.ParseInterval(statsInterval)
	points, err := statsStorage.GetTimeSeries(from, to, statsEndpoint, interval)
	if err != nil {
		return errors.Wrap(err, "failed to get time series")
	}

	fmt.Printf("Time Series (%s to %s, interval: %s)\n", from.Format("2006-01-02"), to.Format("2006-01-02"), statsInterval)
	fmt.Printf("%-25s %10s %8s %12s\n", "Timestamp", "Requests", "Errors", "Avg Latency")
	fmt.Println(strings.Repeat("-", 60))

	for _, p := range points {
		fmt.Printf("%-25s %10d %8d %10.2f ms\n",
			p.Timestamp.Format("2006-01-02 15:04"),
			p.RequestCount,
			p.ErrorCount,
			p.AvgLatencyMs,
		)
	}

	return nil
}

func showLatency(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	latency, err := statsStorage.GetLatencyPercentiles(from, to, statsEndpoint)
	if err != nil {
		return errors.Wrap(err, "failed to get latency stats")
	}

	fmt.Printf("Latency Percentiles (%s to %s)\n", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if statsEndpoint != "" {
		fmt.Printf("Endpoint: %s\n", statsEndpoint)
	}
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("P50:  %d ms\n", latency.P50Ms)
	fmt.Printf("P75:  %d ms\n", latency.P75Ms)
	fmt.Printf("P90:  %d ms\n", latency.P90Ms)
	fmt.Printf("P95:  %d ms\n", latency.P95Ms)
	fmt.Printf("P99:  %d ms\n", latency.P99Ms)
	fmt.Printf("Avg:  %.2f ms\n", latency.AvgMs)
	fmt.Printf("Min:  %d ms\n", latency.MinMs)
	fmt.Printf("Max:  %d ms\n", latency.MaxMs)

	return nil
}

func exportStats(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	from, to, err := parseStatsTimeRange()
	if err != nil {
		return err
	}

	var w = os.Stdout
	if statsOutput != "" {
		f, err := os.Create(statsOutput)
		if err != nil {
			return errors.Wrap(err, "failed to create output file")
		}
		defer f.Close()
		w = f
	}

	switch statsFormat {
	case "json":
		if err := statsStorage.ExportJSON(from, to, w); err != nil {
			return errors.Wrap(err, "failed to export JSON")
		}
	case "csv":
		fallthrough
	default:
		if err := statsStorage.ExportCSV(from, to, w); err != nil {
			return errors.Wrap(err, "failed to export CSV")
		}
	}

	if statsOutput != "" {
		fmt.Printf("Exported to %s\n", statsOutput)
	}

	return nil
}

func purgeStats(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	c := config.Get()
	detailedCutoff := time.Now().UTC().Add(-c.Stats.DetailedRetention())
	aggregatedCutoff := time.Now().UTC().Add(-c.Stats.AggregatedRetention())

	if statsPurgeDryRun {
		fmt.Println("Dry run mode - no data will be deleted")
		fmt.Printf("Would purge detailed logs before: %s\n", detailedCutoff.Format("2006-01-02"))
		fmt.Printf("Would purge aggregated stats before: %s\n", aggregatedCutoff.Format("2006-01-02"))
		return nil
	}

	detailed, err := statsStorage.PurgeDetailedLogs(detailedCutoff)
	if err != nil {
		return errors.Wrap(err, "failed to purge detailed logs")
	}

	aggregated, err := statsStorage.PurgeAggregatedStats(aggregatedCutoff)
	if err != nil {
		return errors.Wrap(err, "failed to purge aggregated stats")
	}

	fmt.Printf("Purged %d detailed log entries\n", detailed)
	fmt.Printf("Purged %d aggregated stat entries\n", aggregated)

	return nil
}

func aggregateStats(cmd *cobra.Command, args []string) error {
	if err := loadStatsStorage(); err != nil {
		return err
	}

	var date time.Time
	if statsAggregateDay != "" {
		var err error
		date, err = parseDate(statsAggregateDay)
		if err != nil {
			return err
		}
	} else {
		// Default to yesterday
		date = time.Now().UTC().Add(-24 * time.Hour)
	}
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	fmt.Printf("Aggregating stats for %s...\n", date.Format("2006-01-02"))

	// We need to create an aggregator to run the aggregation
	// Since we just need to call AggregateDailyStats, we can call it directly
	if agg, ok := statsStorage.(interface{ AggregateDailyStats(time.Time) error }); ok {
		if err := agg.AggregateDailyStats(date); err != nil {
			return errors.Wrap(err, "failed to aggregate stats")
		}
	} else {
		return errors.New("stats storage does not support aggregation")
	}

	fmt.Println("Aggregation completed successfully")
	return nil
}
