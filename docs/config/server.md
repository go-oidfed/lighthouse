---
icon: material/server-network
---

Under the `server` config option the (http) server can be configured.

## `ip_listen`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">0.0.0.0</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_IP_LISTEN`</span>

The `ip_listen` config option is used to set the network address to which to bind to.
If omitted `0.0.0.0` is used.

??? file "config.yaml"

    ```yaml
    server:
        ip_listen: 127.0.0.1
    ```

## `port`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">7672</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_PORT`</span>

The `port` config option is used to set the port at which LightHouse starts 
the webserver and listens for incoming requests.
Will only be used if `tls` is not used.
If `tls` is enabled port `443` will be used (and optionally port `80`).

??? file "config.yaml"

    ```yaml
    server:
        port: 4242
    ```

## `prefork`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_PREFORK`</span>

The `prefork` option enables multiple processes listening on the same port.
When enabled, LightHouse spawns multiple child processes to distribute 
incoming connections across CPU cores for improved performance.

??? file "config.yaml"

    ```yaml
    server:
        prefork: true
    ```

!!! warning "Requirements and Recommendations"

    **We recommend to not enable prefork mode.**

    If you still want to use prefork mode, consider the following:

    **Strongly recommended: Use Redis for caching**
    
    In prefork mode, each child process has its own in-memory caches 
    (eligibility cache, issued trust mark cache, etc.). This means cache 
    invalidations via the Admin API only affect the process that receives 
    the request. To ensure cache consistency across all processes, it is 
    **strongly recommended** to configure Redis for caching:

    ```yaml
    cache:
        redis_addr: "localhost:6379"
    ```

    **Database recommendations**
    
    - **SQLite**: Not recommended with prefork. Multiple processes writing 
      to SQLite may cause write conflicts.
    - **MySQL/PostgreSQL**: Recommended for production deployments with 
      prefork enabled.

    **Background tasks**
    
    Background tasks like the proactive resolver and periodic entity 
    collector run only in the parent process to avoid duplicate work.

    **Admin API**
    
    The Admin API server runs only in the parent process when prefork is 
    enabled.

!!! note "Running in Docker"

    When using prefork with Docker, ensure the application is started with 
    a shell. Use `CMD ./lighthouse` or `CMD ["sh", "-c", "/lighthouse"]` 
    instead of `CMD ["/lighthouse"]` as prefork mode sets environment 
    variables.

## `tls`

Under the `tls` config option settings related to `tls` can be configured.
It is unlikely that one enables `tls` since a reverse proxy will be used in 
most cases.

If `tls` is enabled port `443` will be used.

??? file "config.yaml"

    ```yaml
    server:
        tls:
            enabled: true
            redirect_http: true
            cert: /path/to/cert
            key: /path/to/key
    ```

### `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_TLS_ENABLED`</span>

If set to `false` `tls` will be disabled. Otherwise, it will automatically be 
enabled, if `cert` and `key` are set.

### `redirect_http`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_TLS_REDIRECT_HTTP`</span>

The `redirect_http` option determines if port `80` should be redirected to 
port `443` or not.

### `cert`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-green" title="If this option is required or optional">required for TLS</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_TLS_CERT`</span>

The `cert` option is set to the tls `cert` file.

### `key`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-green" title="If this option is required or optional">required for TLS</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_TLS_KEY`</span>

The `key` option is set to the tls `key` file.

## `trusted_proxies`
<span class="badge badge-purple" title="Value Type">list of strings</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_TRUSTED_PROXIES`</span>

The `trusted_proxies` option is used to configure a list of trusted proxies
by IP address or network range (CIDR notation).

If LightHouse runs behind some sort of proxy, like a load 
balancer, then certain header information may be sent to LightHouse using 
special `X-Forwarded-*` headers or the Forwarded header.
For example, to forward the client's real IP address.

If set, such header information is only used when the request comes via one 
of the trusted proxies. If unset, the information is always read from the 
headers, which might be spoofed.

??? file "config.yaml"

    ```yaml
    server:
        trusted_proxies:
            - "10.0.0.0/8"
            - "172.16.0.0/12"
            - "192.168.0.0/16"
            - "fc00::/7"
    ```

```bash
# Environment variable (comma-separated)
export LH_SERVER_TRUSTED_PROXIES="10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
```

## `forwarded_ip_header`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`X-Forwarded-For`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_FORWARDED_IP_HEADER`</span>

The `forwarded_ip_header` option specifies which HTTP header to use for getting the client's real IP address when behind
a proxy.

??? file "config.yaml"

    ```yaml
    server:
        forwarded_ip_header: X-Real-IP
    ```

## `cors`
<span class="badge badge-purple" title="Value Type">object / mapping</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Configuration for CORS (Cross-Origin Resource Sharing) on the main server.
CORS allows web browsers to make requests to the API from different origins.

This is useful when:

- Hosting API documentation (like Swagger UI) on a different domain
- Building web applications that consume the federation endpoints
- Allowing third-party integrations

??? file "config.yaml"

    ```yaml
    server:
        cors:
            enabled: true
            allow_origins: "*"
            allow_methods: "GET,POST,HEAD,PUT,DELETE,PATCH"
            allow_headers: "Origin,Content-Type,Accept"
            allow_credentials: false
            max_age: 3600
    ```

!!! note "Admin API CORS"
    
    The Admin API has its own separate CORS configuration under `api.admin.cors`.
    This allows you to have different CORS policies for federation endpoints 
    and admin endpoints.

### `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_ENABLED`</span>

Enables or disables CORS middleware for the main server. When disabled, no 
CORS headers are sent and cross-origin requests from browsers will be blocked.

### `allow_origins`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`*`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_ALLOW_ORIGINS`</span>

Comma-separated list of allowed origins, or `*` to allow all origins.

Examples:

- `*` - Allow all origins
- `https://example.com` - Allow only example.com
- `https://example.com,https://app.example.com` - Allow multiple specific origins

??? file "config.yaml"

    ```yaml
    server:
        cors:
            enabled: true
            allow_origins: "https://example.com,https://app.example.com"
    ```

### `allow_methods`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`GET,POST,HEAD,PUT,DELETE,PATCH`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_ALLOW_METHODS`</span>

Comma-separated list of allowed HTTP methods.

### `allow_headers`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">empty (uses Fiber defaults)</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_ALLOW_HEADERS`</span>

Comma-separated list of allowed request headers. If empty, the Fiber CORS 
middleware uses sensible defaults.

### `allow_credentials`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_ALLOW_CREDENTIALS`</span>

Indicates whether the request can include user credentials like cookies, 
HTTP authentication, or client-side SSL certificates.

!!! warning
    
    When `allow_credentials` is `true`, `allow_origins` cannot be set to `*`. 
    You must specify explicit origins.

### `expose_headers`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">empty</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_EXPOSE_HEADERS`</span>

Comma-separated list of headers that browsers are allowed to access.

### `max_age`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`0`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SERVER_CORS_MAX_AGE`</span>

How long (in seconds) browsers should cache preflight request results. 
A value of `0` means no caching.
