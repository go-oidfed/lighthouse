---
icon: material/chart-line
title: Statistics
---

Under the `stats` config option, statistics collection for federation 
endpoints can be configured. When enabled, LightHouse captures detailed 
metrics about requests to federation endpoints including timing, client 
information, and query parameters.

## `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_ENABLED`</span>

The `enabled` option controls whether statistics collection is active.

??? file "config.yaml"

    ```yaml
    stats:
        enabled: true
    ```

## `buffer`
<span class="badge badge-purple" title="Value Type">object</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The `buffer` option configures the in-memory ring buffer used for 
non-blocking request capture. The buffer holds request data temporarily 
before flushing to the database.

??? file "config.yaml"

    ```yaml
    stats:
        enabled: true
        buffer:
            size: 10000
            flush_interval: 5s
            flush_threshold: 0.8
    ```

### `size`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`10000`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_BUFFER_SIZE`</span>

The maximum number of request entries to hold in the ring buffer. If the 
buffer fills up before flushing, older entries are overwritten.

For high-traffic deployments, increase this value to reduce the chance of 
data loss during traffic spikes.

### `flush_interval`
<span class="badge badge-purple" title="Value Type">duration</span>
<span class="badge badge-blue" title="Default Value">`5s`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_BUFFER_FLUSH_INTERVAL`</span>

How often the buffer is flushed to the database. Shorter intervals reduce 
the risk of data loss but increase database write frequency.

### `flush_threshold`
<span class="badge badge-purple" title="Value Type">float (0.0-1.0)</span>
<span class="badge badge-blue" title="Default Value">`0.8`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_BUFFER_FLUSH_THRESHOLD`</span>

Triggers an immediate flush when the buffer reaches this percentage of 
capacity. This prevents data loss during sudden traffic spikes.

## `capture`
<span class="badge badge-purple" title="Value Type">object</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The `capture` option controls what data is collected from each request.

??? file "config.yaml"

    ```yaml
    stats:
        enabled: true
        capture:
            client_ip: true
            user_agent: true
            query_params: true
            geo_ip:
                enabled: false
                database_path: /path/to/GeoLite2-Country.mmdb
    ```

### `client_ip`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_CAPTURE_CLIENT_IP`</span>

Records the client's IP address. When behind a reverse proxy, ensure 
[`server.forwarded_ip_header`](server.md) is configured correctly.

### `user_agent`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_CAPTURE_USER_AGENT`</span>

Records the `User-Agent` header from requests.

### `query_params`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_CAPTURE_QUERY_PARAMS`</span>

Records URL query parameters as JSON. This is useful for analyzing which 
entities are being fetched or resolved most frequently.

### `geo_ip`
<span class="badge badge-purple" title="Value Type">object</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

GeoIP lookup enables country detection from client IP addresses.

#### `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_CAPTURE_GEO_IP_ENABLED`</span>

Enables GeoIP country lookup.

#### `database_path`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-red" title="Required when geo_ip.enabled is true">required if enabled</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_CAPTURE_GEO_IP_DATABASE_PATH`</span>

Path to a MaxMind GeoLite2-Country or GeoIP2-Country database file (`.mmdb`).

!!! info "Obtaining GeoIP Database"
    The GeoLite2-Country database is free but requires registration at 
    [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data).
    Download the `.mmdb` file and specify its path here.

## `retention`
<span class="badge badge-purple" title="Value Type">object</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The `retention` option defines how long statistics data is kept.

??? file "config.yaml"

    ```yaml
    stats:
        enabled: true
        retention:
            detailed_days: 90
            aggregated_days: 365
    ```

### `detailed_days`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`90`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_RETENTION_DETAILED_DAYS`</span>

Number of days to keep individual request logs. After this period, detailed 
logs are deleted but daily aggregates are preserved.

### `aggregated_days`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`365`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_RETENTION_AGGREGATED_DAYS`</span>

Number of days to keep daily aggregated statistics. This enables long-term 
trend analysis with minimal storage requirements.

## `endpoints`
<span class="badge badge-purple" title="Value Type">array of strings</span>
<span class="badge badge-blue" title="Default Value">empty (all federation endpoints)</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STATS_ENDPOINTS`</span>

List of endpoint paths to track. If empty or not specified, all federation 
endpoints are tracked (excluding the admin API).

For environment variables, use comma-separated values: `LH_STATS_ENDPOINTS="/.well-known/openid-federation,/fetch,/resolve"`

??? file "config.yaml"

    ```yaml
    stats:
        enabled: true
        endpoints:
            - /.well-known/openid-federation
            - /fetch
            - /resolve
    ```

## Complete Example

??? file "config.yaml"

    ```yaml
    stats:
        enabled: true
        
        buffer:
            size: 10000
            flush_interval: 5s
            flush_threshold: 0.8
        
        capture:
            client_ip: true
            user_agent: true
            query_params: true
            geo_ip:
                enabled: true
                database_path: /data/GeoLite2-Country.mmdb
        
        retention:
            detailed_days: 90
            aggregated_days: 365
        
        endpoints: []  # Track all federation endpoints
    ```

## Database Considerations

Statistics data is stored in two tables:

- `federation_request_logs` - Individual request records (detailed)
- `federation_daily_stats` - Aggregated daily statistics (compact)

### Storage Estimates

| Traffic Level | Requests/Day | Daily Storage | Yearly Storage (Detailed) | Yearly Storage (Aggregated) |
|---------------|--------------|---------------|---------------------------|----------------------------|
| Low           | 10,000       | ~5 MB         | ~1.8 GB                   | ~50 MB                     |
| Medium        | 100,000      | ~50 MB        | ~18 GB                    | ~500 MB                    |
| High          | 1,000,000    | ~500 MB       | ~180 GB                   | ~5 GB                      |

!!! tip "PostgreSQL Recommended"
    For high-volume deployments (>100,000 requests/day), PostgreSQL is 
    recommended for its superior performance with bulk inserts and 
    analytical queries.
