---
icon: material/api
title: Admin API
---
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Under the `api` config option the Admin API can be configured.

The Admin API provides HTTP endpoints for managing LightHouse configuration, subordinates, 
trust marks, and other aspects of the federation entity at runtime.

## `admin`
<span class="badge badge-purple" title="Value Type">object / mapping</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Configuration for the Admin API.

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
            users_enabled: true
            port: 0
    ```

### `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_ENABLED`</span>

Enables or disables the Admin API. When disabled, no admin endpoints are available.

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
    ```

### `users_enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_USERS_ENABLED`</span>

Enables or disables user management via the Admin API. When enabled, you can create and manage 
admin users through the API endpoints.

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
            users_enabled: true
    ```

### `port`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`0`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_PORT`</span>

The port for the Admin API server. 

- `0` (default) - Serve the Admin API on the main server port alongside federation endpoints
- Any other value - Serve the Admin API on a separate port

Using a separate port can be useful for:

- Network isolation (Admin API only accessible from internal network)
- Different firewall rules for admin vs. federation traffic
- Running admin API on a non-standard port

??? file "config.yaml (same port as main server)"

    ```yaml
    api:
        admin:
            enabled: true
            port: 0
    ```

??? file "config.yaml (separate port)"

    ```yaml
    api:
        admin:
            enabled: true
            port: 8080
    ```

### `actor_source`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`basic_auth`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_ACTOR_SOURCE`</span>

Specifies the source for extracting the actor (user identity) for audit logging in Admin API requests.

Available options:

- `basic_auth` (default) - Extract actor from HTTP Basic Authentication username, use HTTP header as a fallback
- `header` - Extract actor from a custom HTTP header (see `actor_header`), use HTTP Basic Authentication as a fallback

This is useful when running behind a reverse proxy that handles authentication and passes 
the authenticated user in a header.

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
            actor_source: basic_auth
    ```

??? file "config.yaml (header-based authentication)"

    ```yaml
    api:
        admin:
            enabled: true
            actor_source: header
            actor_header: X-Authenticated-User
    ```

### `actor_header`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`X-Actor`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_ACTOR_HEADER`</span>

The HTTP header name to use for extracting the actor when `actor_source` is set to `header` or `both`.

This allows integration with reverse proxies or identity-aware proxies that authenticate 
users and pass their identity in a custom header.

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
            actor_source: header
            actor_header: X-Authenticated-User
    ```

### `password_hashing`
<span class="badge badge-purple" title="Value Type">object / mapping</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Configuration for Argon2id password hashing used for admin user passwords.

!!! note
    These settings affect how admin user passwords are hashed. The defaults provide a good 
    balance of security and performance. Only change these if you have specific requirements.

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
            users_enabled: true
            password_hashing:
                time: 1
                memory_kib: 65536
                parallelism: 4
                key_len: 64
                salt_len: 32
    ```

#### `time`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`1`</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_PASSWORD_HASHING_TIME`</span>

Number of iterations (time cost) for Argon2id. Higher values increase computation time.

#### `memory_kib`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`65536`</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_PASSWORD_HASHING_MEMORY_KIB`</span>

Memory usage in KiB for Argon2id. Default is 64 MiB.

#### `parallelism`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`4`</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_PASSWORD_HASHING_PARALLELISM`</span>

Number of parallel threads for Argon2id.

#### `key_len`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`64`</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_PASSWORD_HASHING_KEY_LEN`</span>

Length of the derived key in bytes.

#### `salt_len`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`32`</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_PASSWORD_HASHING_SALT_LEN`</span>

Length of the random salt in bytes.

### `cors`
<span class="badge badge-purple" title="Value Type">object / mapping</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Configuration for CORS (Cross-Origin Resource Sharing) on the Admin API.
CORS allows web browsers to make requests to the Admin API from different origins.

This is particularly useful for:

- Using the Swagger UI documentation from a different server
- Building admin dashboards hosted on different domains
- Testing the Admin API from development tools

??? file "config.yaml"

    ```yaml
    api:
        admin:
            cors:
                enabled: true
                allow_origins: "*"
                allow_methods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS"
                allow_headers: "Origin,Content-Type,Accept,Authorization"
                allow_credentials: true
                max_age: 3600
    ```

!!! tip "Quick Enable for Swagger UI"
    
    To quickly enable CORS for using Swagger UI from a different origin, 
    you can simply set:
    
    ```yaml
    api:
        admin:
            cors:
                enabled: true
    ```
    
    Or via environment variable:
    
    ```bash
    export LH_API_ADMIN_CORS_ENABLED=true
    ```
    
    The defaults are configured to work with Swagger UI out of the box.

!!! note "Separate from Main Server CORS"
    
    The Admin API has its own CORS configuration, separate from the main 
    server's `server.cors` settings. This allows different policies for 
    federation endpoints vs. admin endpoints.

#### `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_ENABLED`</span>

Enables or disables CORS middleware for the Admin API.

#### `allow_origins`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`*`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_ALLOW_ORIGINS`</span>

Comma-separated list of allowed origins, or `*` to allow all origins.

Examples:

- `*` - Allow all origins
- `https://admin.example.com` - Allow only admin.example.com
- `https://admin.example.com,https://localhost:3000` - Allow multiple origins

#### `allow_methods`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_ALLOW_METHODS`</span>

Comma-separated list of allowed HTTP methods.

#### `allow_headers`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">`Origin,Content-Type,Accept,Authorization`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_ALLOW_HEADERS`</span>

Comma-separated list of allowed request headers. The default includes 
`Authorization` to support HTTP Basic Authentication used by the Admin API.

#### `allow_credentials`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_ALLOW_CREDENTIALS`</span>

Indicates whether the request can include user credentials like cookies 
or HTTP authentication headers. Default is `true` to support HTTP Basic 
Authentication used by the Admin API.

!!! warning
    
    When `allow_credentials` is `true`, `allow_origins` cannot be set to `*` 
    in strict CORS implementations. However, Fiber's CORS middleware handles 
    this by echoing back the requesting origin when credentials are allowed.

#### `expose_headers`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">empty</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_EXPOSE_HEADERS`</span>

Comma-separated list of headers that browsers are allowed to access.

#### `max_age`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">`3600`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_API_ADMIN_CORS_MAX_AGE`</span>

How long (in seconds) browsers should cache preflight request results. 
Default is 3600 (1 hour).

## Complete Example

??? file "config.yaml"

    ```yaml
    api:
        admin:
            enabled: true
            users_enabled: true
            port: 0
            actor_source: header
            actor_header: X-Actor
            password_hashing:
                time: 1
                memory_kib: 65536
                parallelism: 4
                key_len: 64
                salt_len: 32
            cors:
                enabled: true
                allow_origins: "*"
                allow_methods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS"
                allow_headers: "Origin,Content-Type,Accept,Authorization"
                allow_credentials: true
                max_age: 3600
    ```

## Security Considerations

!!! warning "Production Deployments"
    
    In production, consider the following security measures for the Admin API:
    
    - **Network isolation**: Use a separate port and firewall rules to restrict access
    - **TLS**: Always use HTTPS for admin API traffic
    - **Strong passwords**: Use strong, unique passwords for admin users

## Admin API Endpoints

The Admin API provides endpoints for managing:

- **Subordinates** - Add, update, remove subordinate entities
- **Trust Mark Specs** - Configure trust mark issuance specifications
- **Trust Marked Entities** - Manage trust mark eligibility
- **Signing Configuration** - Update signing algorithm and key rotation settings
- **Federation Metadata** - Update entity metadata and authority hints
- **Users** - Manage admin users (when `users_enabled` is true)

For detailed API documentation, see the OpenAPI specification at `/admin/api/v1/docs` when the Admin API is enabled.
