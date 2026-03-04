package stats

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

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

// Collector manages statistics collection for federation endpoints.
type Collector struct {
	config  Config
	buffer  *RingBuffer
	flusher *Flusher
	geoIP   GeoIPProvider
	storage StorageBackend

	trackedEndpoints map[string]bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCollector creates a new statistics collector.
func NewCollector(cfg Config, storage StorageBackend) (*Collector, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Create ring buffer
	buffer := NewRingBuffer(cfg.BufferSize, cfg.FlushThreshold)

	// Create GeoIP lookup if enabled
	var geoIP GeoIPProvider = NoOpGeoIPLookup{}
	if cfg.GeoIPEnabled && cfg.GeoIPDBPath != "" {
		var err error
		geoIP, err = NewGeoIPLookup(cfg.GeoIPDBPath)
		if err != nil {
			log.WithError(err).Warn("failed to initialize GeoIP lookup, country detection disabled")
			geoIP = NoOpGeoIPLookup{}
		} else {
			log.WithField("path", cfg.GeoIPDBPath).Info("GeoIP lookup initialized")
		}
	}

	// Build tracked endpoints map
	trackedEndpoints := BuildTrackedEndpoints(cfg.Endpoints)

	// Create flusher
	flusher := NewFlusher(buffer, storage, cfg.FlushInterval)

	ctx, cancel := context.WithCancel(context.Background())

	return &Collector{
		config:           cfg,
		buffer:           buffer,
		flusher:          flusher,
		geoIP:            geoIP,
		storage:          storage,
		trackedEndpoints: trackedEndpoints,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// Middleware returns a Fiber middleware handler for collecting statistics.
func (c *Collector) Middleware() fiber.Handler {
	if c == nil {
		return func(ctx *fiber.Ctx) error {
			return ctx.Next()
		}
	}

	return Middleware(MiddlewareConfig{
		CaptureClientIP:    c.config.CaptureClientIP,
		CaptureUserAgent:   c.config.CaptureUserAgent,
		CaptureQueryParams: c.config.CaptureQueryParams,
		GeoIP:              c.geoIP,
		TrackedEndpoints:   c.trackedEndpoints,
		Buffer:             c.buffer,
	})
}

// Start begins the background flushing goroutine.
// This method is non-blocking and returns immediately.
func (c *Collector) Start() {
	if c == nil {
		return
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.flusher.Run(c.ctx); err != nil && err != context.Canceled {
			log.WithError(err).Error("stats flusher exited with error")
		}
	}()

	log.Info("stats collector started")
}

// Stop gracefully shuts down the collector.
// It signals the flusher to stop and waits for the final flush to complete.
func (c *Collector) Stop() error {
	if c == nil {
		return nil
	}

	log.Info("stopping stats collector")
	c.cancel()
	c.wg.Wait()

	// Close GeoIP database if applicable
	if closer, ok := c.geoIP.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			log.WithError(err).Warn("failed to close GeoIP database")
		}
	}

	log.Info("stats collector stopped")
	return nil
}

// Record manually records a request log entry.
// This is useful for testing or custom integrations.
func (c *Collector) Record(entry *RequestLog) {
	if c == nil || c.buffer == nil {
		return
	}
	c.buffer.Write(entry)
}

// BufferStats returns current buffer statistics.
func (c *Collector) BufferStats() BufferStats {
	if c == nil || c.buffer == nil {
		return BufferStats{}
	}
	return BufferStats{
		Size:           c.buffer.Size(),
		Capacity:       c.buffer.Capacity(),
		FillPercentage: c.buffer.FillPercentage(),
	}
}

// FlusherStats returns current flusher statistics.
func (c *Collector) FlusherStats() FlusherStats {
	if c == nil || c.flusher == nil {
		return FlusherStats{}
	}
	return c.flusher.Stats()
}

// BufferStats holds buffer operational statistics.
type BufferStats struct {
	Size           int     `json:"size"`
	Capacity       int     `json:"capacity"`
	FillPercentage float64 `json:"fill_percentage"`
}
