---
icon: material/chart-line
---

# Statistics & Analytics

LightHouse includes a comprehensive statistics collection system that 
captures detailed metrics about federation endpoint usage. This enables 
monitoring, analysis, and reporting on how your federation is being used.

## Overview

The statistics system captures:

- **Request metrics**: Endpoint, method, status code, response time
- **Client information**: IP address, User-Agent, country (optional)
- **Request details**: Query parameters, request/response sizes
- **Error tracking**: Error types and frequencies

Statistics can be accessed via:

- **REST API** - JSON endpoints under `/api/v1/admin/stats/`
- **CLI** - [`lhcli stats`](../deployment/lhcli.md#statistics) commands
- **Export** - CSV or JSON file export

## Enabling Statistics

Statistics collection is disabled by default. Enable it in your 
configuration:

```yaml
stats:
    enabled: true
```

See [Statistics Configuration](../config/stats.md) for all options.

## REST API

All statistics endpoints are under `/api/v1/admin/stats/` and require 
authentication (same as other admin endpoints).

### Common Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `from` | date | 24 hours ago | Start of time range (RFC3339 or YYYY-MM-DD) |
| `to` | date | now | End of time range (RFC3339 or YYYY-MM-DD) |
| `limit` | integer | 10 | Number of results for top-N queries (max 100) |
| `endpoint` | string | all | Filter by specific endpoint |
| `interval` | string | hour | Time bucket size for time series |

### Endpoints

#### GET /stats/summary

Returns an overall summary of request statistics.

**Response:**
```json
{
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z",
    "summary": {
        "total_requests": 1234567,
        "total_errors": 1234,
        "error_rate": 0.001,
        "avg_latency_ms": 45.2,
        "p50_latency_ms": 32,
        "p95_latency_ms": 120,
        "p99_latency_ms": 250,
        "unique_clients": 5678,
        "unique_user_agents": 42,
        "requests_by_status": {
            "200": 1233333,
            "404": 1000,
            "500": 234
        },
        "requests_by_endpoint": {
            "well-known": 500000,
            "fetch": 400000,
            "resolve": 334567
        }
    }
}
```

#### GET /stats/top/endpoints

Returns top endpoints by request count.

**Response:**
```json
{
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z",
    "limit": 10,
    "endpoints": [
        {"value": "well-known", "count": 500000},
        {"value": "fetch", "count": 400000},
        {"value": "resolve", "count": 334567}
    ]
}
```

#### GET /stats/top/user-agents

Returns top User-Agent strings by request count.

#### GET /stats/top/clients

Returns top client IP addresses by request count.

#### GET /stats/top/countries

Returns top countries by request count (requires GeoIP).

#### GET /stats/top/params

Returns top query parameter combinations.

**Query Parameters:**

- `endpoint` - Filter by specific endpoint (recommended)

#### GET /stats/timeseries

Returns time series data for request counts.

**Query Parameters:**

- `interval` - Time bucket: `minute`, `hour`, `day`, `week`, `month`
- `endpoint` - Filter by specific endpoint

**Response:**
```json
{
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-02T00:00:00Z",
    "endpoint": "",
    "interval": "hour",
    "timeseries": [
        {
            "timestamp": "2024-01-01T00:00:00Z",
            "request_count": 1234,
            "error_count": 5,
            "avg_latency_ms": 42.5
        },
        {
            "timestamp": "2024-01-01T01:00:00Z",
            "request_count": 1456,
            "error_count": 3,
            "avg_latency_ms": 38.2
        }
    ]
}
```

#### GET /stats/latency

Returns latency percentile statistics.

**Response:**
```json
{
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z",
    "endpoint": "",
    "latency": {
        "p50_ms": 32,
        "p75_ms": 55,
        "p90_ms": 95,
        "p95_ms": 120,
        "p99_ms": 250,
        "avg_ms": 45.2,
        "min_ms": 5,
        "max_ms": 2500
    }
}
```

#### GET /stats/daily

Returns daily aggregated statistics.

#### GET /stats/export

Exports raw request log data.

**Query Parameters:**

- `format` - Export format: `csv` (default) or `json`

Returns a file download with the exported data.

## CLI Commands

The `lhcli stats` command provides access to statistics from the command line.

```bash
# View summary
lhcli stats summary --from 2024-01-01 --to 2024-01-31

# Top endpoints
lhcli stats top endpoints --limit 20

# Time series data
lhcli stats timeseries --interval hour --endpoint fetch

# Export to file
lhcli stats export --format csv --output stats.csv
```

For complete CLI documentation including all commands, flags, and examples, 
see the [lhcli stats reference](../deployment/lhcli.md#statistics).

## Data Retention

Statistics uses a two-tier retention system:

### Detailed Logs

Individual request records are stored in `federation_request_logs`. These 
contain full details including client IP, User-Agent, and query parameters.

- Default retention: **90 days**
- Configurable via `stats.retention.detailed_days`

### Aggregated Statistics

Daily summaries are stored in `federation_daily_stats`. These contain 
aggregated counts, percentiles, and top-N lists.

- Default retention: **365 days**
- Configurable via `stats.retention.aggregated_days`
- Generated automatically at 2 AM UTC daily

!!! info "Automatic Aggregation"
    LightHouse automatically aggregates detailed logs into daily statistics 
    and purges old data based on retention settings. This runs daily at 
    2 AM UTC.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Request                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Stats Middleware (non-blocking)                 │
│  Captures: timing, IP, User-Agent, params, status           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Ring Buffer (in-memory)                         │
│  - Capacity: configurable (default 10,000)                   │
│  - Non-blocking writes                                       │
│  - Overflow: oldest entries dropped                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Background Flusher                              │
│  - Flushes every 5 seconds (configurable)                    │
│  - Batch inserts for efficiency                              │
│  - PostgreSQL: Uses COPY for max performance                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Database                                        │
│  ┌─────────────────────┐    ┌─────────────────────┐        │
│  │ federation_request  │    │ federation_daily    │        │
│  │ _logs (detailed)    │───▶│ _stats (aggregated) │        │
│  │ 90 day retention    │    │ 365 day retention   │        │
│  └─────────────────────┘    └─────────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## Performance Impact

The statistics system is designed for minimal performance impact:

- **Zero latency overhead**: Uses non-blocking buffer writes
- **Efficient storage**: Batch inserts reduce database load
- **Memory bounded**: Ring buffer prevents unbounded memory growth
- **Background processing**: All database writes happen asynchronously

## Database Compatibility

Statistics works with all supported databases:

| Database | Bulk Insert | JSON Queries | Recommended For |
|----------|-------------|--------------|-----------------|
| PostgreSQL | COPY (fastest) | Full JSONB support | High volume (>100K/day) |
| MySQL | Batch INSERT | JSON_EXTRACT | Medium volume |
| SQLite | Batch INSERT | Limited | Development/testing |

## Visualization

While LightHouse doesn't include built-in dashboards, the statistics data 
can be easily visualized using external tools:

### Grafana

Export data via the API and import into Grafana for real-time dashboards.

### Metabase / Apache Superset

Connect directly to the LightHouse database to create interactive reports.

### Custom Dashboards

Use the REST API to build custom dashboards with any frontend framework.

### Spreadsheets

Export to CSV and import into Excel, Google Sheets, or similar tools for 
ad-hoc analysis.
