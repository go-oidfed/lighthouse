---
icon: material/file-cog
---

# Config
LightHouse is configured through a configuration file named `config.yaml` and/or 
environment variables.

## Configuration Sources

Configuration values are loaded in the following order (later sources override earlier ones):

1. **Default values** - Built-in defaults
2. **YAML config file** - Values from `config.yaml`
3. **Environment variables** - Override any value from the config file

## Config File Location

LightHouse will search for the config file at startup at different locations, the 
first file that is found will be used. Supported locations are:

- `config.yaml`
- `config/config.yaml`
- `/config/config.yaml`
- `/lighthouse/config/config.yaml`
- `/lighthouse/config.yaml`
- `/data/config/config.yaml`
- `/data/config.yaml`
- `/etc/lighthouse/config.yaml`

You can also specify the config file path via the `LH_CONFIG_FILE` environment variable:

```bash
export LH_CONFIG_FILE=/path/to/config.yaml
```

## Environment Variables

All configuration options can be set via environment variables with the `LH_` prefix.
Environment variables **override** values from the YAML config file.

### Naming Convention

Environment variable names are derived from the YAML path by:

1. Adding the `LH_` prefix
2. Converting to UPPERCASE
3. Replacing dots/nesting with underscores

For example:

| YAML Path | Environment Variable |
|-----------|---------------------|
| `entity_id` | `LH_ENTITY_ID` |
| `server.port` | `LH_SERVER_PORT` |
| `server.tls.enabled` | `LH_SERVER_TLS_ENABLED` |
| `storage.dsn` | `LH_STORAGE_DSN` |
| `logging.internal.level` | `LH_LOGGING_INTERNAL_LEVEL` |

### Special Environment Variables

| Variable | Description |
|----------|-------------|
| `LH_CONFIG_FILE` | Path to the configuration file |
| `LH_LOG_LEVEL` | Shortcut for `LH_LOGGING_INTERNAL_LEVEL` |

### Array/List Values

For configuration options that accept lists (e.g., `trusted_proxies`), use comma-separated values:

```bash
export LH_SERVER_TRUSTED_PROXIES="10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
```

### Duration Values

Duration values accept Go duration strings:

```bash
export LH_CACHE_MAX_LIFETIME="6h"
```

### YAML-Only Options

Some complex configuration options can only be set via YAML (not environment variables):

- `signing.filesystem` - Filesystem KMS configuration
- `signing.pkcs11` - PKCS#11 HSM configuration

### Example: Docker/Kubernetes Deployment

Environment variables are particularly useful for container deployments:

```yaml
# docker-compose.yml
services:
  lighthouse:
    image: lighthouse:latest
    environment:
      - LH_ENTITY_ID=https://ta.example.com
      - LH_SERVER_PORT=8080
      - LH_STORAGE_DRIVER=postgres
      - LH_STORAGE_DSN=host=db user=lighthouse password=secret dbname=lighthouse
      - LH_LOG_LEVEL=INFO
    volumes:
      - ./keys:/keys
```

## Example Config File
The following is an example `config.yaml` file:

??? file "config.yaml"

    ```yaml
    server:
        port: 7672

    entity_id: "https://ta.example.lh"

    signing:
        kms: filesystem
        pk_backend: db
        auto_generate_keys: true
        filesystem:
            key_dir: "/keys"

    storage:
        driver: sqlite
        data_dir: "/data"

    api:
        admin:
            enabled: true
            users_enabled: true
    ```

!!! info "DB-Managed Configuration"

    Federation endpoints, trust anchors, metadata, authority hints, trust marks,
    signing algorithm, key rotation, and other runtime options are stored in the
    database. See [Database-Managed Configuration](db/index.md) for the full
    reference. Use [`lhsetup`](../deployment/lhsetup.md) for interactive setup,
    [`lhmigrate config2db`](../migration/0.22.md) for non-interactive migration
    from a config file, or the [Admin API](../features/admin_api.md) for runtime
    management.

## `entity_id`
<span class="badge badge-purple" title="Value Type">URI</span>
<span class="badge badge-red" title="If this option is required or optional">required</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_ENTITY_ID`</span>

The `entity_id` option sets the Entity Identifier for this federation entity. This is a URI that 
uniquely identifies your entity within the federation and is used as the `iss` and `sub` claims 
in the Entity Configuration JWT.

The entity ID is typically the base URL where your federation endpoints are served.

??? file "config.yaml"

    ```yaml
    entity_id: "https://ta.example.com"
    ```

!!! warning "Required"
    
    This option is **required**. LightHouse will fail to start if `entity_id` is not specified.

!!! note "Moved from federation_data"
    
    This option was previously located at `federation_data.entity_id`. It has been moved to 
    a top-level configuration option.     Use [`lhmigrate config`](../migration/index.md#config-migration) 
    to automatically update your config file.

## Configuration Sections

LightHouse configuration is split into two categories:

### Static Configuration (Config File & Environment Variables)

These options are set in `config.yaml` and/or environment variables. They
require a restart to change.

<div class="grid cards" markdown>

- [:material-server-network: Server](static/server.md)
- [:material-script-text: Logging](static/logging.md)
- [:material-database: Storage](static/storage.md)
- [:material-database-clock: Cache](static/cache.md)
- [:material-signature-freehand: Signing](static/signing.md)
- [:material-api: Admin API](static/api.md)
- [:material-chart-line: Statistics](static/stats.md)

</div>

### Database-Managed Configuration

These options are stored in the database and can be changed at runtime via the
Admin API, `lhsetup`, or `lhmigrate config2db` — no restart required.

<div class="grid cards" markdown>

- [:material-database-cog: Database-Managed Config](db/index.md)
- [:material-signature-freehand: Signing](db/signing.md)
- [:material-openid: Entity Configuration](db/entity-configuration.md)
- [:material-shield-check-outline: Trust Marks](db/trust-marks.md)
- [:material-shield-check: Trust Anchors](db/trust-anchors.md)
- [:material-routes: Federation Endpoints](db/federation-endpoints.md)
- [:material-account-group: Subordinates](db/subordinates.md)

</div>

## :fontawesome-solid-stopwatch: Time Duration Configuration Options
Some configuration option take a duration, e.g. the lifetime of entity 
statements or the entity configuration.

There are different options how to pass a duration in the config file:

- **Number**: If only a number is given, this is the number of seconds.
- **String**: The duration can also be given as a string which supports 
  different units.

For a duration string the following units are supported and multiple units 
can be used in a single string:

| Symbol | Unit        | Comment            |
|--------|-------------|--------------------|
| `y`    | Year        | = 365 days         |
| `w`    | Week        | = 7 days           |
| `d`    | Day         | = 24 hours         |
| `h`    | Hour        |                    |
| `m`    | Minute      |                    |
| `s`    | Second      |                    |
| `ms`   | Millisecond | SHOULD NOT be used |
| `µs`   | Microsecond | SHOULD NOT be used |
| `ns`   | Nanosecond  | SHOULD NOT be used |


!!! Example "Examples"
    ```
    1y
    2w6d
    20d
    1h30m
    ```
