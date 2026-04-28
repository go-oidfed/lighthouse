package stats

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// AggregatorStorage is the interface for aggregation storage operations.
type AggregatorStorage interface {
	AggregateDailyStats(date time.Time) error
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

	log.WithFields(log.Fields{
		"next_run":             next2AM,
		"detailed_retention":   a.detailedRetention,
		"aggregated_retention": a.aggregatedRetention,
	}).Info("stats aggregator started")

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
	log.Info("starting daily stats aggregation")
	start := time.Now()

	// Aggregate yesterday's data
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	yesterday = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)

	if err := a.storage.AggregateDailyStats(yesterday); err != nil {
		log.WithError(err).Error("failed to aggregate daily stats")
	} else {
		log.WithField("date", yesterday.Format("2006-01-02")).Info("daily stats aggregated")
		a.lastAggregation = yesterday
	}

	// Purge old detailed logs
	detailedCutoff := time.Now().UTC().Add(-a.detailedRetention)
	purged, err := a.storage.PurgeDetailedLogs(detailedCutoff)
	if err != nil {
		log.WithError(err).Error("failed to purge detailed logs")
	} else if purged > 0 {
		log.WithFields(log.Fields{
			"purged": purged,
			"before": detailedCutoff,
		}).Info("purged detailed logs")
	}

	// Purge old aggregated stats
	aggregatedCutoff := time.Now().UTC().Add(-a.aggregatedRetention)
	purged, err = a.storage.PurgeAggregatedStats(aggregatedCutoff)
	if err != nil {
		log.WithError(err).Error("failed to purge aggregated stats")
	} else if purged > 0 {
		log.WithFields(log.Fields{
			"purged": purged,
			"before": aggregatedCutoff,
		}).Info("purged aggregated stats")
	}

	log.WithField("duration", time.Since(start)).Info("daily stats aggregation completed")
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
