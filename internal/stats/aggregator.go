package stats

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// AggregatorStorage is the interface for aggregation storage operations.
type AggregatorStorage interface {
	AggregateDailyStats(date time.Time) error
	HasDailyStatsForDate(date time.Time) (bool, error)
	PurgeDetailedLogs(before time.Time) (int64, error)
	PurgeAggregatedStats(before time.Time) (int64, error)
}

// Aggregator handles daily aggregation and data retention.
type Aggregator struct {
	storage             AggregatorStorage
	detailedRetention   time.Duration
	aggregatedRetention time.Duration

	// lastAggregation tracks the last date we aggregated
	lastAggregation time.Time
}

// NewAggregator creates a new aggregator instance.
func NewAggregator(storage AggregatorStorage, detailedRetention, aggregatedRetention time.Duration) *Aggregator {
	return &Aggregator{
		storage:             storage,
		detailedRetention:   detailedRetention,
		aggregatedRetention: aggregatedRetention,
	}
}

// Run starts the aggregation loop. It runs once per day at 2 AM UTC.
// This method blocks until the context is cancelled.
func (a *Aggregator) Run(ctx context.Context) error {
	// Calculate time until next 2 AM UTC
	now := time.Now().UTC()
	next2AM := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
	if now.After(next2AM) {
		next2AM = next2AM.Add(24 * time.Hour)
	}
	waitDuration := next2AM.Sub(now)

	log.Info().
		Time("next_run", next2AM).
		Dur("detailed_retention", a.detailedRetention).
		Dur("aggregated_retention", a.aggregatedRetention).
		Msg("stats aggregator started")

	// Backfill any missing days within the detailed-log retention window so
	// /stats/daily is populated immediately on startup rather than only after
	// the next 2 AM tick. Only days whose detailed logs are still retained are
	// considered, so already-purged data is never overwritten.
	a.Backfill(ctx)

	// Wait until first run time
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitDuration):
	}

	// Run daily at 2 AM
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run immediately on first tick
	a.runAggregation()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			a.runAggregation()
		}
	}
}

// runAggregation performs the daily aggregation and purge tasks.
func (a *Aggregator) runAggregation() {
	log.Info().Msg("starting daily stats aggregation")
	start := time.Now()

	// Aggregate yesterday's data
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	yesterday = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)

	if err := a.storage.AggregateDailyStats(yesterday); err != nil {
		log.Error().Err(err).Msg("failed to aggregate daily stats")
	} else {
		log.Info().Str("date", yesterday.Format("2006-01-02")).Msg("daily stats aggregated")
		a.lastAggregation = yesterday
	}

	// Purge old detailed logs
	detailedCutoff := time.Now().UTC().Add(-a.detailedRetention)
	purged, err := a.storage.PurgeDetailedLogs(detailedCutoff)
	if err != nil {
		log.Error().Err(err).Msg("failed to purge detailed logs")
	} else if purged > 0 {
		log.Info().Int64("purged", purged).Time("before", detailedCutoff).Msg("purged detailed logs")
	}

	// Purge old aggregated stats
	aggregatedCutoff := time.Now().UTC().Add(-a.aggregatedRetention)
	purged, err = a.storage.PurgeAggregatedStats(aggregatedCutoff)
	if err != nil {
		log.Error().Err(err).Msg("failed to purge aggregated stats")
	} else if purged > 0 {
		log.Info().Int64("purged", purged).Time("before", aggregatedCutoff).Msg("purged aggregated stats")
	}

	log.Info().Dur("duration", time.Since(start)).Msg("daily stats aggregation completed")
}

// Backfill aggregates every day from (now - detailedRetention) up to yesterday
// that does not already have daily stats rows. It is safe to run repeatedly:
// days with existing rows are skipped, and only days whose detailed logs are
// still retained are considered, so already-purged data is never overwritten.
func (a *Aggregator) Backfill(ctx context.Context) {
	now := time.Now().UTC()
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	horizon := yesterday.Add(-a.detailedRetention)

	backfilled := 0
	for d := horizon; d.Before(yesterday); d = d.Add(24 * time.Hour) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		exists, err := a.storage.HasDailyStatsForDate(d)
		if err != nil {
			log.Warn().Err(err).
				Str("date", d.Format("2006-01-02")).
				Msg("failed to check existing daily stats during backfill")
			continue
		}
		if exists {
			continue
		}

		if err := a.storage.AggregateDailyStats(d); err != nil {
			log.Warn().Err(err).
				Str("date", d.Format("2006-01-02")).
				Msg("failed to backfill daily stats")
			continue
		}
		// Only count/log days that actually produced rows; days with no
		// detailed logs aggregate to nothing and would otherwise spam the
		// log on every startup.
		created, err := a.storage.HasDailyStatsForDate(d)
		if err != nil || !created {
			continue
		}
		backfilled++
		log.Info().Str("date", d.Format("2006-01-02")).Msg("backfilled daily stats")
	}

	if backfilled > 0 {
		log.Info().Int("days_backfilled", backfilled).Msg("daily stats backfill completed")
	}
}

// RunOnce performs a single aggregation for the specified date.
// This is useful for CLI commands or manual aggregation.
func (a *Aggregator) RunOnce(date time.Time) error {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	return a.storage.AggregateDailyStats(date)
}

// Purge manually purges data older than the retention periods.
func (a *Aggregator) Purge() (detailed int64, aggregated int64, err error) {
	detailedCutoff := time.Now().UTC().Add(-a.detailedRetention)
	detailed, err = a.storage.PurgeDetailedLogs(detailedCutoff)
	if err != nil {
		return
	}

	aggregatedCutoff := time.Now().UTC().Add(-a.aggregatedRetention)
	aggregated, err = a.storage.PurgeAggregatedStats(aggregatedCutoff)
	return
}

// LastAggregation returns the date of the last successful aggregation.
func (a *Aggregator) LastAggregation() time.Time {
	return a.lastAggregation
}
