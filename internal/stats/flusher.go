package stats

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// Flusher handles periodic flushing of the ring buffer to the database.
type Flusher struct {
	buffer   *RingBuffer
	storage  StorageBackend
	interval time.Duration

	// Metrics
	totalFlushed  int64
	totalDropped  int64
	lastFlushTime time.Time
	lastFlushSize int
}

// StorageBackend is the interface that storage must implement for the flusher.
type StorageBackend interface {
	InsertBatch(entries []*RequestLog) error
}

// NewFlusher creates a new flusher instance.
func NewFlusher(buffer *RingBuffer, storage StorageBackend, interval time.Duration) *Flusher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Flusher{
		buffer:   buffer,
		storage:  storage,
		interval: interval,
	}
}

// Run starts the flusher loop. It flushes on:
// - Regular interval (FlushInterval)
// - Buffer threshold reached (via NotifyThreshold channel)
// - Context cancellation (final flush)
//
// This method blocks until the context is cancelled.
func (f *Flusher) Run(ctx context.Context) error {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", f.interval).
		Float64("threshold", f.buffer.threshold).
		Int("capacity", f.buffer.capacity).
		Msg("stats flusher started")

	for {
		select {
		case <-ctx.Done():
			// Final flush before exit
			log.Info().Msg("stats flusher shutting down, performing final flush")
			f.flush()
			return ctx.Err()

		case <-ticker.C:
			f.flush()

		case <-f.buffer.NotifyThreshold:
			// Buffer reached threshold, flush immediately
			f.flush()
		}
	}
}

// flush drains the buffer and inserts entries into the database.
func (f *Flusher) flush() {
	entries := f.buffer.Drain()
	if len(entries) == 0 {
		return
	}

	start := time.Now()
	err := f.storage.InsertBatch(entries)
	duration := time.Since(start)

	if err != nil {
		f.totalDropped += int64(len(entries))
		log.Error().Err(err).
			Int("count", len(entries)).
			Dur("duration", duration).
			Msg("failed to flush stats to database, entries dropped")
		return
	}

	f.totalFlushed += int64(len(entries))
	f.lastFlushTime = time.Now()
	f.lastFlushSize = len(entries)

	log.Debug().
		Int("count", len(entries)).
		Dur("duration", duration).
		Msg("stats flushed to database")
}

// Stats returns flusher statistics.
func (f *Flusher) Stats() FlusherStats {
	return FlusherStats{
		TotalFlushed:  f.totalFlushed,
		TotalDropped:  f.totalDropped,
		LastFlushTime: f.lastFlushTime,
		LastFlushSize: f.lastFlushSize,
		BufferSize:    f.buffer.Size(),
		BufferFill:    f.buffer.FillPercentage(),
	}
}

// FlusherStats holds flusher operational statistics.
type FlusherStats struct {
	TotalFlushed  int64     `json:"total_flushed"`
	TotalDropped  int64     `json:"total_dropped"`
	LastFlushTime time.Time `json:"last_flush_time"`
	LastFlushSize int       `json:"last_flush_size"`
	BufferSize    int       `json:"buffer_size"`
	BufferFill    float64   `json:"buffer_fill"`
}
