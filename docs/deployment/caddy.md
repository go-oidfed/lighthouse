---
title: Caddy & Docker
description: Deployment of LightHouse with Docker and Caddy reverse proxy.
icon: simple/docker
---

# Caddy & Docker Deployment

This guide covers deploying LightHouse using Docker with a Caddy reverse proxy.



=== ":simple-sqlite: SQLite"

    ## Project Layout

    ```tree
    caddy/
        Caddyfile
        config/
        data/
    docker-compose.yaml
    lighthouse/
        config.yaml
        data/
            keys/
    ```

    ## Configuration Files

    === ":material-file-code: `docker-compose.yaml`"

        ```yaml
        services:
          caddy:
            image: caddy:latest
            restart: unless-stopped
            ports:
              - "80:80"
              - "443:443"
            volumes:
              - ./caddy/Caddyfile:/etc/caddy/Caddyfile
              - ./caddy/data:/data
              - ./caddy/config:/config

          lighthouse:
            image: oidfed/lighthouse:latest
            restart: unless-stopped
            volumes:
              - ./lighthouse/config.yaml:/config.yaml:ro
              - ./lighthouse/data:/data
        ```

    === ":material-file-code: `caddy/Caddyfile`"

        ```caddy
        lighthouse.example.com {
            reverse_proxy lighthouse:7672
        }
        ```

        For separate admin API access (recommended for production):

        ```caddy
        # Public federation endpoints
        lighthouse.example.com {
            reverse_proxy lighthouse:7672
        }

        # Admin API (restrict access via firewall or Caddy matchers)
        admin.lighthouse.example.com {
            reverse_proxy lighthouse:7673
        }
        ```

    === ":material-file-code: `lighthouse/config.yaml`"

        ```yaml
        server:
          port: 7672

        # Entity identifier - CHANGE THIS to your domain
        entity_id: "https://lighthouse.example.com"

        # Signing configuration
        signing:
          kms: filesystem
          pk_backend: db
          auto_generate_keys: true
          filesystem:
            key_dir: "/data/keys"

        # Storage configuration
        storage:
          driver: sqlite
          data_dir: "/data"

        # Admin API
        api:
          admin:
            enabled: true
            users_enabled: true
        ```

        Federation endpoints, trust anchors, metadata, and other
        DB-managed options are configured with `lhsetup` or the Admin API.

        For more configuration options, see [Configuration](../config/index.md).

=== ":simple-postgresql: PostgreSQL"

    ## Project Layout

    ```tree
    caddy/
        Caddyfile
        config/
        data/
    docker-compose.yaml
    lighthouse/
        config.yaml
        data/
            keys/
    postgres/
        data/
    ```

    ## Configuration Files

    === ":material-file-code: `docker-compose.yaml`"

        ```yaml
        services:
          caddy:
            image: caddy:latest
            restart: unless-stopped
            ports:
              - "80:80"
              - "443:443"
            volumes:
              - ./caddy/Caddyfile:/etc/caddy/Caddyfile
              - ./caddy/data:/data
              - ./caddy/config:/config

          postgres:
            image: postgres:16-alpine
            restart: unless-stopped
            environment:
              POSTGRES_USER: lighthouse
              POSTGRES_PASSWORD: changeme  # Change this!
              POSTGRES_DB: lighthouse
            volumes:
              - ./postgres/data:/var/lib/postgresql/data
            healthcheck:
              test: ["CMD-SHELL", "pg_isready -U lighthouse"]
              interval: 5s
              timeout: 5s
              retries: 5

          lighthouse:
            image: oidfed/lighthouse:latest
            restart: unless-stopped
            depends_on:
              postgres:
                condition: service_healthy
            environment:
              LH_STORAGE_DSN: "host=postgres user=lighthouse password=changeme dbname=lighthouse sslmode=disable"
            volumes:
              - ./lighthouse/config.yaml:/config.yaml:ro
              - ./lighthouse/data:/data
        ```

    === ":material-file-code: `caddy/Caddyfile`"

        ```caddy
        lighthouse.example.com {
            reverse_proxy lighthouse:7672
        }
        ```

        For separate admin API access (recommended for production):

        ```caddy
        # Public federation endpoints
        lighthouse.example.com {
            reverse_proxy lighthouse:7672
        }

        # Admin API (restrict access via firewall or Caddy matchers)
        admin.lighthouse.example.com {
            reverse_proxy lighthouse:7673
        }
        ```

    === ":material-file-code: `lighthouse/config.yaml`"

        ```yaml
        server:
          port: 7672

        # Entity identifier - CHANGE THIS to your domain
        entity_id: "https://lighthouse.example.com"

        # Signing configuration
        signing:
          kms: filesystem
          pk_backend: db
          auto_generate_keys: true
          filesystem:
            key_dir: "/data/keys"

        # Storage configuration
        storage:
          driver: postgres
          # DSN set via LH_STORAGE_DSN environment variable in docker-compose.yaml

        # Admin API
        api:
          admin:
            enabled: true
            users_enabled: true
            # Separate port for admin API (optional)
            # port: 7673

        # Statistics (optional, recommended for production)
        stats:
          enabled: true
          retention:
            detailed_days: 90
            aggregated_days: 365
        ```

        Federation endpoints, trust anchors, metadata, and other
        DB-managed options are configured with `lhsetup` or the Admin API.

        For more configuration options, see [Configuration](../config/index.md).

!!! tip "Environment Variables"

    Configuration can also be passed via environment variables:

    ```yaml
    environment:
      LH_STORAGE_DSN: "host=postgres user=lighthouse password=${DB_PASSWORD} dbname=lighthouse"
      LH_ENTITY_ID: "https://lighthouse.example.com"
    ```

    See [Configuration](../config/index.md#environment-variables) for details.

## Initial Setup

After starting the containers with `docker compose up -d`, configure your 
federation entity using `lhsetup` or the Admin API.

### Option A: Interactive Setup with lhsetup (Recommended)

Run the interactive setup wizard inside the container:

```bash
# View current configuration (all defaults on a fresh database)
docker exec -it lighthouse /lhsetup show

# Run the interactive setup wizard
docker exec -it lighthouse /lhsetup
```

The wizard will prompt you for all DB-managed configuration: federation
endpoints, trust anchors, signing options, metadata, authority hints, trust
marks, and more. Press Enter to keep the current value at each prompt.

See the [lhsetup documentation](lhsetup.md) for details on available sections
and flags.

### Option B: Admin API

Alternatively, configure your entity via HTTP requests:

#### 1. Create an Admin User

```bash
# Via API (basic auth disabled initially if no users exist)
curl -X POST https://lighthouse.example.com/api/v1/admin/users \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-secure-password"}'
```

!!! note "Authentication Behavior"
    When no users exist, the Admin API does not require authentication. This allows you to create the first admin user. Once at least one user exists, all API requests require HTTP Basic Authentication.

#### 2. Configure Federation Metadata

```bash
curl -X PUT https://lighthouse.example.com/api/v1/admin/entity-configuration/metadata/federation_entity \
  -u admin:your-secure-password \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Example Organization",
    "homepage_uri": "https://example.com",
    "contacts": ["admin@example.com"]
  }'
```

#### 3. Set Authority Hints (if not a Trust Anchor)

```bash
curl -X POST https://lighthouse.example.com/api/v1/admin/entity-configuration/authority-hints \
  -u admin:your-secure-password \
  -H "Content-Type: application/json" \
  -d '{"entity_id": "https://trust-anchor.example.org"}'
```

#### 4. Configure Trust Mark Issuance (optional)

```bash
curl -X POST https://lighthouse.example.com/api/v1/admin/trust-marks/issuance-spec \
  -u admin:your-secure-password \
  -H "Content-Type: application/json" \
  -d '{
    "trust_mark_type": "https://lighthouse.example.com/trustmarks/member",
    "lifetime": "8760h"
  }'
```

## Verification

Check that LightHouse is running correctly:

```bash
# Fetch entity configuration
curl https://lighthouse.example.com/.well-known/openid-federation

# Check Admin API
curl https://lighthouse.example.com/api/v1/admin/entity-configuration \
  -u admin:your-secure-password
```


## Next Steps

- [Configuration Reference](../config/index.md) - Full configuration options
- [Interactive Setup](lhsetup.md) - Bootstrap DB-managed config with `lhsetup`
- [Admin API](../features/admin_api.md) - Manage your federation via REST API
- [CLI Tool](lhcli.md) - Command-line management with `lhcli`
- [Migration Guide](../migration/index.md) - Upgrading from older versions
