---
icon: material/database-clock
---

Under the `cache` config option Lighthouse can be configured to use an external cache system.
Currently, only Redis is supported (in additional to in-memory caching).

## `redis_addr`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The `redis_addr` option sets the address of a Redis server to be used as a cache backend.
If set, Lighthouse initializes Redis caching at startup. If not set or empty, no external cache is used and in-memory defaults apply.

Typical formats:

- `hostname:port` (e.g. `localhost:6379`)
- `ip:port` (e.g. `10.0.0.5:6379`)

??? file "config.yaml"

    ```yaml
    cache:
      redis_addr: "localhost:6379"
    ```
