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

## `username`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Sets the Redis ACL username used to authenticate to the server.
Leave empty when Redis is configured without ACL users.

??? file "config.yaml"

    ```yaml
    cache:
      redis_addr: "localhost:6379"
      username: "app-user"
    ```

## `password`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Sets the password for Redis authentication. Used with or without `username`
depending on your Redis setup.

??? file "config.yaml"

    ```yaml
    cache:
      redis_addr: "localhost:6379"
      password: "s3cr3t-pass"
    ```

## `redis_db`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Selects the Redis logical database index to use. Defaults to `0` if not
set. Common deployments use `0`; choose another index when sharing a
Redis instance with other applications.

??? file "config.yaml"

    ```yaml
    cache:
      redis_addr: "localhost:6379"
      redis_db: 1
    ```
