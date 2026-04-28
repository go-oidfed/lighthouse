package adminapi

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/internal/stats"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// StatsAPI provides REST endpoints for querying statistics.
type StatsAPI struct {
	storage model.StatsStorageBackend
}

// NewStatsAPI creates a new stats API instance.
func NewStatsAPI(storage model.StatsStorageBackend) *StatsAPI {
	return &StatsAPI{storage: storage}
}

// RegisterRoutes registers all stats routes under the given router group.
func (api *StatsAPI) RegisterRoutes(r fiber.Router) {
	r.Get("/summary", api.getSummary)
	r.Get("/top/endpoints", api.getTopEndpoints)
	r.Get("/top/user-agents", api.getTopUserAgents)
	r.Get("/top/clients", api.getTopClients)
	r.Get("/top/countries", api.getTopCountries)
	r.Get("/top/params", api.getTopParams)
	r.Get("/timeseries", api.getTimeSeries)
	r.Get("/latency", api.getLatency)
	r.Get("/daily", api.getDailyStats)
	r.Get("/export", api.export)
}

// parseTimeRange extracts from/to time parameters from the request.
// Defaults to last 24 hours if not specified.
func parseTimeRange(c *fiber.Ctx) (from, to time.Time) {
	now := time.Now().UTC()
	to = now
	from = now.Add(-24 * time.Hour)

	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}

	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			// End of day
			to = t.Add(24*time.Hour - time.Second)
		}
	}

	return from, to
}

// parseLimit extracts the limit parameter, defaulting to 10.
func parseLimit(c *fiber.Ctx) int {
	limit := c.QueryInt("limit", 10)
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

// getSummary returns overall statistics for the given time range.
// GET /stats/summary?from=2024-01-01&to=2024-01-31
func (api *StatsAPI) getSummary(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)

	summary, err := api.storage.GetSummary(from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":    from,
		"to":      to,
		"summary": summary,
	})
}

// getTopEndpoints returns the top endpoints by request count.
// GET /stats/top/endpoints?from=&to=&limit=10
func (api *StatsAPI) getTopEndpoints(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	limit := parseLimit(c)

	entries, err := api.storage.GetTopEndpoints(from, to, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":      from,
		"to":        to,
		"limit":     limit,
		"endpoints": entries,
	})
}

// getTopUserAgents returns the top user agents by request count.
// GET /stats/top/user-agents?from=&to=&limit=10
func (api *StatsAPI) getTopUserAgents(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	limit := parseLimit(c)

	entries, err := api.storage.GetTopUserAgents(from, to, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":        from,
		"to":          to,
		"limit":       limit,
		"user_agents": entries,
	})
}

// getTopClients returns the top client IPs by request count.
// GET /stats/top/clients?from=&to=&limit=10
func (api *StatsAPI) getTopClients(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	limit := parseLimit(c)

	entries, err := api.storage.GetTopClients(from, to, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":    from,
		"to":      to,
		"limit":   limit,
		"clients": entries,
	})
}

// getTopCountries returns the top countries by request count.
// GET /stats/top/countries?from=&to=&limit=10
func (api *StatsAPI) getTopCountries(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	limit := parseLimit(c)

	entries, err := api.storage.GetTopCountries(from, to, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":      from,
		"to":        to,
		"limit":     limit,
		"countries": entries,
	})
}

// getTopParams returns the top query parameters for an endpoint.
// GET /stats/top/params?from=&to=&endpoint=fetch&limit=10
func (api *StatsAPI) getTopParams(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	limit := parseLimit(c)
	endpoint := c.Query("endpoint")

	entries, err := api.storage.GetTopQueryParams(from, to, endpoint, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":     from,
		"to":       to,
		"limit":    limit,
		"endpoint": endpoint,
		"params":   entries,
	})
}

// getTimeSeries returns time series data.
// GET /stats/timeseries?from=&to=&endpoint=&interval=hour
func (api *StatsAPI) getTimeSeries(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	endpoint := c.Query("endpoint")
	interval := stats.ParseInterval(c.Query("interval", "hour"))

	points, err := api.storage.GetTimeSeries(from, to, endpoint, interval)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":       from,
		"to":         to,
		"endpoint":   endpoint,
		"interval":   interval,
		"timeseries": points,
	})
}

// getLatency returns latency percentiles.
// GET /stats/latency?from=&to=&endpoint=
func (api *StatsAPI) getLatency(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	endpoint := c.Query("endpoint")

	latency, err := api.storage.GetLatencyPercentiles(from, to, endpoint)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":     from,
		"to":       to,
		"endpoint": endpoint,
		"latency":  latency,
	})
}

// getDailyStats returns aggregated daily statistics.
// GET /stats/daily?from=&to=
func (api *StatsAPI) getDailyStats(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)

	daily, err := api.storage.GetDailyStats(from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"from":  from,
		"to":    to,
		"daily": daily,
	})
}

// export exports statistics data.
// GET /stats/export?from=&to=&format=csv|json
func (api *StatsAPI) export(c *fiber.Ctx) error {
	from, to := parseTimeRange(c)
	format := c.Query("format", "csv")

	switch format {
	case "json":
		c.Set(fiber.HeaderContentType, "application/json")
		c.Set(fiber.HeaderContentDisposition, "attachment; filename=stats.json")
		return api.storage.ExportJSON(from, to, c.Response().BodyWriter())

	case "csv":
		fallthrough
	default:
		c.Set(fiber.HeaderContentType, "text/csv")
		c.Set(fiber.HeaderContentDisposition, "attachment; filename=stats.csv")
		return api.storage.ExportCSV(from, to, c.Response().BodyWriter())
	}
}
