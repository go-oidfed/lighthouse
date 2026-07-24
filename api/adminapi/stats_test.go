package adminapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	istats "github.com/go-oidfed/lighthouse/internal/stats"
	"github.com/go-oidfed/lighthouse/storage/model"
)

type mockStatsStorageBackend struct {
	insertBatchFn          func([]*istats.RequestLog) error
	getSummaryFn           func(time.Time, time.Time) (*istats.Summary, error)
	getTopEndpointsFn      func(time.Time, time.Time, int) ([]istats.TopEntry, error)
	getTopUserAgentsFn     func(time.Time, time.Time, int) ([]istats.TopEntry, error)
	getTopClientsFn        func(time.Time, time.Time, int) ([]istats.TopEntry, error)
	getTopCountriesFn      func(time.Time, time.Time, int) ([]istats.TopEntry, error)
	getTopQueryParamsFn    func(time.Time, time.Time, string, int) ([]istats.TopEntry, error)
	getTimeSeriesFn        func(time.Time, time.Time, string, istats.Interval) ([]istats.TimeSeriesPoint, error)
	getLatencyFn           func(time.Time, time.Time, string) (*istats.LatencyStats, error)
	aggregateDailyStatsFn  func(time.Time) error
	hasDailyStatsForDateFn func(time.Time) (bool, error)
	getDailyStatsFn        func(time.Time, time.Time) ([]istats.DailyStats, error)
	purgeDetailedLogsFn    func(time.Time) (int64, error)
	purgeAggregatedStatsFn func(time.Time) (int64, error)
	exportCSVFn            func(time.Time, time.Time, io.Writer) error
	exportJSONFn           func(time.Time, time.Time, io.Writer) error
}

func (m *mockStatsStorageBackend) InsertBatch(entries []*istats.RequestLog) error {
	if m.insertBatchFn != nil {
		return m.insertBatchFn(entries)
	}
	return nil
}

func (m *mockStatsStorageBackend) GetSummary(from, to time.Time) (*istats.Summary, error) {
	return m.getSummaryFn(from, to)
}

func (m *mockStatsStorageBackend) GetTopEndpoints(from, to time.Time, limit int) ([]istats.TopEntry, error) {
	return m.getTopEndpointsFn(from, to, limit)
}

func (m *mockStatsStorageBackend) GetTopUserAgents(from, to time.Time, limit int) ([]istats.TopEntry, error) {
	return m.getTopUserAgentsFn(from, to, limit)
}

func (m *mockStatsStorageBackend) GetTopClients(from, to time.Time, limit int) ([]istats.TopEntry, error) {
	return m.getTopClientsFn(from, to, limit)
}

func (m *mockStatsStorageBackend) GetTopCountries(from, to time.Time, limit int) ([]istats.TopEntry, error) {
	return m.getTopCountriesFn(from, to, limit)
}

func (m *mockStatsStorageBackend) GetTopQueryParams(from, to time.Time, endpoint string, limit int) ([]istats.TopEntry, error) {
	return m.getTopQueryParamsFn(from, to, endpoint, limit)
}

func (m *mockStatsStorageBackend) GetTimeSeries(from, to time.Time, endpoint string, interval istats.Interval) ([]istats.TimeSeriesPoint, error) {
	return m.getTimeSeriesFn(from, to, endpoint, interval)
}

func (m *mockStatsStorageBackend) GetLatencyPercentiles(from, to time.Time, endpoint string) (*istats.LatencyStats, error) {
	return m.getLatencyFn(from, to, endpoint)
}

func (m *mockStatsStorageBackend) AggregateDailyStats(date time.Time) error {
	if m.aggregateDailyStatsFn != nil {
		return m.aggregateDailyStatsFn(date)
	}
	return nil
}

func (m *mockStatsStorageBackend) HasDailyStatsForDate(date time.Time) (bool, error) {
	if m.hasDailyStatsForDateFn != nil {
		return m.hasDailyStatsForDateFn(date)
	}
	return false, nil
}

func (m *mockStatsStorageBackend) GetDailyStats(from, to time.Time) ([]istats.DailyStats, error) {
	return m.getDailyStatsFn(from, to)
}

func (m *mockStatsStorageBackend) PurgeDetailedLogs(before time.Time) (int64, error) {
	if m.purgeDetailedLogsFn != nil {
		return m.purgeDetailedLogsFn(before)
	}
	return 0, nil
}

func (m *mockStatsStorageBackend) PurgeAggregatedStats(before time.Time) (int64, error) {
	if m.purgeAggregatedStatsFn != nil {
		return m.purgeAggregatedStatsFn(before)
	}
	return 0, nil
}

func (m *mockStatsStorageBackend) ExportCSV(from, to time.Time, w io.Writer) error {
	return m.exportCSVFn(from, to, w)
}

func (m *mockStatsStorageBackend) ExportJSON(from, to time.Time, w io.Writer) error {
	return m.exportJSONFn(from, to, w)
}

func setupStatsTestApp(t *testing.T, store model.StatsStorageBackend) *fiber.App {
	t.Helper()
	app := fiber.New()
	NewStatsAPI(store).RegisterRoutes(app.Group("/stats"))
	return app
}

func TestStatsAPISummary(t *testing.T) {
	t.Parallel()

	t.Run("UsesExplicitDateRange", func(t *testing.T) {
		t.Parallel()

		var gotFrom time.Time
		var gotTo time.Time
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getSummaryFn: func(from, to time.Time) (*istats.Summary, error) {
				gotFrom = from
				gotTo = to
				return &istats.Summary{TotalRequests: 42}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/summary?from=2026-01-01&to=2026-01-31", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		wantFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
		if !gotFrom.Equal(wantFrom) || !gotTo.Equal(wantTo) {
			t.Fatalf("unexpected time range: got %s to %s, want %s to %s", gotFrom, gotTo, wantFrom, wantTo)
		}
		if !strings.Contains(string(body), `"total_requests":42`) {
			t.Fatalf("expected response to contain summary payload, got %s", string(body))
		}
	})

	t.Run("DefaultsToLast24Hours", func(t *testing.T) {
		t.Parallel()

		var gotFrom time.Time
		var gotTo time.Time
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getSummaryFn: func(from, to time.Time) (*istats.Summary, error) {
				gotFrom = from
				gotTo = to
				return &istats.Summary{}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/summary", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)

		diff := gotTo.Sub(gotFrom)
		if diff < 23*time.Hour+59*time.Minute || diff > 24*time.Hour+time.Minute {
			t.Fatalf("expected default range to be about 24h, got %s", diff)
		}
	})

	t.Run("StoreError", func(t *testing.T) {
		t.Parallel()

		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getSummaryFn: func(_, _ time.Time) (*istats.Summary, error) {
				return nil, io.ErrUnexpectedEOF
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/summary", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusInternalServerError)
	})
}

func TestStatsAPITopEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("CapsLimitAndParsesRFC3339", func(t *testing.T) {
		t.Parallel()

		var gotFrom time.Time
		var gotTo time.Time
		var gotLimit int
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTopEndpointsFn: func(from, to time.Time, limit int) ([]istats.TopEntry, error) {
				gotFrom = from
				gotTo = to
				gotLimit = limit
				return []istats.TopEntry{{Value: "fetch", Count: 9}}, nil
			},
		})

		req := httptest.NewRequest(
			http.MethodGet,
			"/stats/top/endpoints?from=2026-03-05T12:00:00Z&to=2026-03-05T13:00:00Z&limit=500",
			http.NoBody,
		)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)

		if gotLimit != 100 {
			t.Fatalf("expected capped limit 100, got %d", gotLimit)
		}
		if !gotFrom.Equal(time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)) || !gotTo.Equal(time.Date(2026, 3, 5, 13, 0, 0, 0, time.UTC)) {
			t.Fatalf("unexpected RFC3339 time range: got %s to %s", gotFrom, gotTo)
		}
		if !strings.Contains(string(body), `"value":"fetch"`) {
			t.Fatalf("expected response to contain endpoint entry, got %s", string(body))
		}
	})

	t.Run("DefaultsLimitWhenNonPositive", func(t *testing.T) {
		t.Parallel()

		var gotLimit int
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTopEndpointsFn: func(_, _ time.Time, limit int) ([]istats.TopEntry, error) {
				gotLimit = limit
				return []istats.TopEntry{}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/top/endpoints?limit=-3", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)
		if gotLimit != 10 {
			t.Fatalf("expected default limit 10, got %d", gotLimit)
		}
	})
}

func TestStatsAPITopBreakdowns(t *testing.T) {
	t.Parallel()

	t.Run("UserAgents", func(t *testing.T) {
		t.Parallel()

		var gotLimit int
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTopUserAgentsFn: func(_, _ time.Time, limit int) ([]istats.TopEntry, error) {
				gotLimit = limit
				return []istats.TopEntry{{Value: "curl/8.0", Count: 5}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/top/user-agents?limit=5", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if gotLimit != 5 {
			t.Fatalf("expected limit 5, got %d", gotLimit)
		}
		if !strings.Contains(string(body), `"curl/8.0"`) {
			t.Fatalf("expected user-agent entry in body, got %s", string(body))
		}
	})

	t.Run("Clients", func(t *testing.T) {
		t.Parallel()

		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTopClientsFn: func(_, _ time.Time, _ int) ([]istats.TopEntry, error) {
				return []istats.TopEntry{{Value: "127.0.0.1", Count: 4}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/top/clients", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if !strings.Contains(string(body), `"127.0.0.1"`) {
			t.Fatalf("expected client entry in body, got %s", string(body))
		}
	})

	t.Run("Countries", func(t *testing.T) {
		t.Parallel()

		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTopCountriesFn: func(_, _ time.Time, _ int) ([]istats.TopEntry, error) {
				return []istats.TopEntry{{Value: "SE", Count: 3}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/top/countries", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if !strings.Contains(string(body), `"SE"`) {
			t.Fatalf("expected country entry in body, got %s", string(body))
		}
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		var gotEndpoint string
		var gotLimit int
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTopQueryParamsFn: func(_, _ time.Time, endpoint string, limit int) ([]istats.TopEntry, error) {
				gotEndpoint = endpoint
				gotLimit = limit
				return []istats.TopEntry{{Value: "sub=https://rp.example", Count: 7}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/top/params?endpoint=fetch&limit=7", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if gotEndpoint != "fetch" || gotLimit != 7 {
			t.Fatalf("unexpected query params call: endpoint=%q limit=%d", gotEndpoint, gotLimit)
		}
		if !strings.Contains(string(body), `"sub=https://rp.example"`) {
			t.Fatalf("expected params entry in body, got %s", string(body))
		}
	})
}

func TestStatsAPITimeSeriesLatencyAndDaily(t *testing.T) {
	t.Parallel()

	t.Run("TimeSeriesParsesInterval", func(t *testing.T) {
		t.Parallel()

		var gotEndpoint string
		var gotInterval istats.Interval
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTimeSeriesFn: func(_, _ time.Time, endpoint string, interval istats.Interval) ([]istats.TimeSeriesPoint, error) {
				gotEndpoint = endpoint
				gotInterval = interval
				return []istats.TimeSeriesPoint{{Timestamp: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC), RequestCount: 12}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/timeseries?endpoint=fetch&interval=day", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if gotEndpoint != "fetch" || gotInterval != istats.IntervalDay {
			t.Fatalf("unexpected timeseries call: endpoint=%q interval=%q", gotEndpoint, gotInterval)
		}
		if !strings.Contains(string(body), `"request_count":12`) {
			t.Fatalf("expected timeseries point in body, got %s", string(body))
		}
	})

	t.Run("TimeSeriesDefaultsInvalidIntervalToHour", func(t *testing.T) {
		t.Parallel()

		var gotInterval istats.Interval
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getTimeSeriesFn: func(_, _ time.Time, _ string, interval istats.Interval) ([]istats.TimeSeriesPoint, error) {
				gotInterval = interval
				return []istats.TimeSeriesPoint{}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/timeseries?interval=bogus", http.NoBody)
		resp, bodyBytes := doRequest(t, app, req)
		requireStatus(t, resp, bodyBytes, http.StatusOK)
		if gotInterval != istats.IntervalHour {
			t.Fatalf("expected default interval %q, got %q", istats.IntervalHour, gotInterval)
		}
	})

	t.Run("Latency", func(t *testing.T) {
		t.Parallel()

		var gotEndpoint string
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getLatencyFn: func(_, _ time.Time, endpoint string) (*istats.LatencyStats, error) {
				gotEndpoint = endpoint
				return &istats.LatencyStats{P95Ms: 120, AvgMs: 45.5}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/latency?endpoint=resolve", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if gotEndpoint != "resolve" {
			t.Fatalf("expected latency endpoint %q, got %q", "resolve", gotEndpoint)
		}
		if !strings.Contains(string(body), `"p95_ms":120`) {
			t.Fatalf("expected latency payload in body, got %s", string(body))
		}
	})

	t.Run("DailyStats", func(t *testing.T) {
		t.Parallel()

		var gotFrom time.Time
		var gotTo time.Time
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			getDailyStatsFn: func(from, to time.Time) ([]istats.DailyStats, error) {
				gotFrom = from
				gotTo = to
				return []istats.DailyStats{{RequestCount: 55, Endpoint: "fetch"}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/daily?from=2026-04-01&to=2026-04-03", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if !gotFrom.Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) || !gotTo.Equal(time.Date(2026, 4, 3, 23, 59, 59, 0, time.UTC)) {
			t.Fatalf("unexpected daily time range: got %s to %s", gotFrom, gotTo)
		}
		if !strings.Contains(string(body), `"request_count":55`) {
			t.Fatalf("expected daily payload in body, got %s", string(body))
		}
	})
}

func TestStatsAPIExport(t *testing.T) {
	t.Parallel()

	t.Run("JSONExport", func(t *testing.T) {
		t.Parallel()

		var jsonCalls int
		var csvCalls int
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			exportCSVFn: func(_, _ time.Time, _ io.Writer) error {
				csvCalls++
				return nil
			},
			exportJSONFn: func(_, _ time.Time, w io.Writer) error {
				jsonCalls++
				_, err := io.WriteString(w, `{"ok":true}`)
				return err
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/export?format=json", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if jsonCalls != 1 || csvCalls != 0 {
			t.Fatalf("expected json export once and csv never, got json=%d csv=%d", jsonCalls, csvCalls)
		}
		if !strings.HasPrefix(resp.Header.Get(fiber.HeaderContentType), "application/json") {
			t.Fatalf("unexpected content type %q", resp.Header.Get(fiber.HeaderContentType))
		}
		if resp.Header.Get(fiber.HeaderContentDisposition) != "attachment; filename=stats.json" {
			t.Fatalf("unexpected content disposition %q", resp.Header.Get(fiber.HeaderContentDisposition))
		}
		if string(body) != `{"ok":true}` {
			t.Fatalf("unexpected export body %s", string(body))
		}
	})

	t.Run("UnknownFormatFallsBackToCSV", func(t *testing.T) {
		t.Parallel()

		var jsonCalls int
		var csvCalls int
		app := setupStatsTestApp(t, &mockStatsStorageBackend{
			exportCSVFn: func(_, _ time.Time, w io.Writer) error {
				csvCalls++
				_, err := io.WriteString(w, "endpoint,count\nfetch,9\n")
				return err
			},
			exportJSONFn: func(_, _ time.Time, _ io.Writer) error {
				jsonCalls++
				return nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/stats/export?format=xml", http.NoBody)
		resp, body := doRequest(t, app, req)
		requireStatus(t, resp, body, http.StatusOK)
		if csvCalls != 1 || jsonCalls != 0 {
			t.Fatalf("expected csv export once and json never, got csv=%d json=%d", csvCalls, jsonCalls)
		}
		if !strings.HasPrefix(resp.Header.Get(fiber.HeaderContentType), "text/csv") {
			t.Fatalf("unexpected content type %q", resp.Header.Get(fiber.HeaderContentType))
		}
		if resp.Header.Get(fiber.HeaderContentDisposition) != "attachment; filename=stats.csv" {
			t.Fatalf("unexpected content disposition %q", resp.Header.Get(fiber.HeaderContentDisposition))
		}
		if string(body) != "endpoint,count\nfetch,9\n" {
			t.Fatalf("unexpected export body %s", string(body))
		}
	})
}
