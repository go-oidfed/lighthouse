---
icon: material/wrench
---

# Interactive Setup (lhsetup)

The `lhsetup` command-line tool interactively prompts for all database-managed
configuration options and writes them directly to the database. It is an
alternative to using [`lhmigrate config2db`](../migration/0.22.md) for initial
setup or the [Admin API](../features/admin_api.md) for runtime changes.

## When to Use lhsetup

- **Initial setup** — Configure federation endpoints, trust anchors, signing
  options, metadata, and more after deploying LightHouse for the first time.
- **Reconfiguration** — Change existing DB-managed values without constructing
  Admin API HTTP requests.
- **Inspection** — View all current DB-managed configuration with `lhsetup show`.

## Installation

The `lhsetup` binary is included in LightHouse Docker containers alongside the
`lighthouse`, `lhcli`, and `lhmigrate` binaries. You can also build it from
source:

```bash
go build -o lhsetup ./cmd/lhsetup
```

## Usage

```bash
lhsetup [flags]
lhsetup show [flags]
```

### Commands

| Command  | Description                                                      |
|----------|------------------------------------------------------------------|
| *(none)* | Run the interactive setup wizard through all config sections.    |
| `show`   | Display all current DB-managed configuration values (read-only). |

### Flags

| Flag         | Short | Default | Description                                                                 |
|--------------|-------|---------|-----------------------------------------------------------------------------|
| `--config`   | `-c`  | *(auto)* | Path to the LightHouse config file. Auto-discovered if not specified.      |
| `--db-type`  |       |         | Override database type: `sqlite`, `mysql`, or `postgres`.                  |
| `--db-dsn`   |       |         | Override database DSN (for mysql/postgres).                                |
| `--db-dir`   |       |         | Override data directory (for sqlite).                                       |
| `--db-debug` |       |         | Enable GORM debug logging.                                                  |
| `--only`     |       |         | Comma-separated list of sections to configure (default: all).              |
| `--skip`     |       |         | Comma-separated list of sections to skip.                                   |

### Database Connection

By default, `lhsetup` connects to the database using the same config file and
environment variables as the LightHouse server. Override flags (`--db-type`,
`--db-dsn`, `--db-dir`) take precedence over config file settings.

### Config File Prepopulation

If a config file is provided (via `--config` or auto-discovery), values from
the config file are used to prepopulate prompts. This is useful when migrating
from a config-file-based setup: the tool shows the config file value as a hint
and lets you accept it with Enter.

Existing DB values always take precedence as the default. The config file value
is only shown as a hint when it differs from the current DB value.

## Configurable Sections

`lhsetup` covers all configuration that was formerly in the config file and is
now stored in the database:

| Section                   | Description                                                              |
|---------------------------|--------------------------------------------------------------------------|
| `config_lifetime`         | Entity configuration lifetime                                            |
| `statement_lifetime`      | Subordinate statement lifetime                                           |
| `alg`                     | Signing algorithm                                                        |
| `rsa_key_len`             | RSA key length (only prompted for RSA/PS algorithms)                     |
| `key_rotation`            | Key rotation settings (enabled, interval, overlap, lead time)            |
| `metadata`                | Federation entity metadata (via JSON file)                               |
| `constraints`             | General subordinate statement constraints (via JSON file)                |
| `metadata_policies`       | General metadata policies (via JSON file)                                |
| `metadata_policy_crit`    | Critical metadata policy operators                                       |
| `authority_hints`         | Authority hints (add/remove)                                             |
| `extra_entity_config`     | Extra entity configuration claims (add/remove)                           |
| `trust_marks`             | Published trust marks (add/remove)                                       |
| `trust_mark_specs`        | Trust mark issuance specs (add/remove)                                   |
| `trust_mark_issuers`      | Trust mark issuers per type (add/remove)                                 |
| `trust_mark_owners`       | Trust mark owners per type (add/remove)                                  |
| `trust_anchors`           | Trust anchors (add/remove with JWKS)                                     |
| `endpoints`               | Federation endpoints (path, URL, auth, type-specific config)             |

Use `--only` or `--skip` to select specific sections:

```bash
# Only configure signing options
lhsetup --only=alg,rsa_key_len,key_rotation

# Configure everything except endpoints
lhsetup --skip=endpoints
```

## Running in Docker

The `lhsetup` binary is included in the Docker image. Use `docker exec` to run
it interactively:

```bash
# View current configuration
docker exec -it lighthouse /lhsetup show

# Run the interactive setup wizard
docker exec -it lighthouse /lhsetup

# Configure only signing options
docker exec -it lighthouse /lhsetup --only=alg,key_rotation
```

If your config file is mounted at `/config.yaml` in the container, `lhsetup`
will auto-discover it for database connection settings and prepopulation:

```bash
docker exec -it lighthouse /lhsetup --config=/config.yaml
```

You can also override the database connection directly:

```bash
docker exec -it lighthouse /lhsetup --db-type=sqlite --db-dir=/data
```

## Example Session

```
$ lhsetup --only=config_lifetime,alg

========================================
  Lighthouse Interactive Setup
========================================

=== ENTITY CONFIGURATION LIFETIME ===

  Current: 24h0m0s
Change? [y/n]: y
New lifetime (e.g. 24h, 30m, 600s) [24h0m0s]: 48h
  Set to 48h0m0s

=== SIGNING ALGORITHM ===

  Current: ES512
Change? [y/n]: y
Select signing algorithm:
  1. ES512 (current)
  2. ES256
  3. ES384
  4. ES256K
  5. EdDSA
  6. Ed25519
  7. Ed448
  8. PS512
  9. PS256
  10. PS384
  11. RS512
  12. RS384
  13. RS256
  14. ML-DSA-44
  15. ML-DSA-65
  16. ML-DSA-87
  17. ML-DSA-44-ES256
  18. ML-DSA-65-ES256
  19. ML-DSA-87-ES384
  20. ML-DSA-44-Ed25519
  21. ML-DSA-65-Ed25519
  22. ML-DSA-87-Ed448
Select [1-22] (Enter to keep current): 8
  Set to PS512

========================================
  Setup complete!
========================================
```

## Relationship to Other Tools

| Tool        | Purpose                                                      |
|-------------|--------------------------------------------------------------|
| `lhsetup`   | Interactive setup and reconfiguration of DB-managed options. |
| `lhmigrate` | Non-interactive migration from config file / legacy formats. |
| Admin API   | Runtime management via HTTP; offers the most features.       |
| `lhcli`     | Subordinate, trust mark, and statistics management.          |

`lhsetup` and `lhmigrate config2db` cover overlapping ground: both write
config-file values to the database. Use `lhmigrate config2db` for automated /
non-interactive migrations, and `lhsetup` when you want to review and adjust
values interactively.
