---
icon: material/walk
---

# Migration to LightHouse 0.20.0

This page covers how to migrate to LightHouse >= 0.20.0.
LightHouse 0.20.0 is a major release with significant changes, including 
key management and storage backends.

!!! warning
    You cannot directly upgrade from LightHouse <0.20.0 to 0.20.0.

    You MUST migrate your deployment.

We still try to make it as easy as possible to migrate your deployment.

## Understanding the Changes in LightHouse 0.20.0
The main addition in LightHouse 0.20.0 is an http admin API. This admin API 
allows admins of LightHouse to manage almost all aspects of LightHouse via 
this API. This has several implications. The following is a list of changes 
that are relevant:

- LightHouse now has an admin API, which allows management of LightHouse via 
  HTTP requests.
- The storage backend (database) is now driven by a SQL database.
    - Different databases are supported: SQLite, MySQL, and PostgreSQL.
    - To keep existing data, you must migrate your database to the new backend.
- The key management has been changed.
    - LightHouse supports private keys on the filesystem or in an HSM.
        - Existing keys can be migrated to the new filesystem KMS.
    - LightHouse can manage public keys on the filesystem or in the database 
      (default is database).
- The configuration file is not backwards compatible with previous versions.
    - Some options might have been renamed or removed.
    - You must migrate your configuration file to the latest format.
    - Several of the options can now be configured via the admin API, but 
      have been removed from the configuration file.
        - There is a migration tool to read a config file and update 
          LightHouse via the admin API.
    - Most of the options can now be configured via environment variables.

## Build the tool

From the repository root:

```bash
go build -o lhmigrate ./cmd/lhmigrate
```

## Signing keys

The key migrations live under the `keys` command (alias: `signing`). There are two subcommands:

- `public`: Migrate legacy public key storage (JWKS + rotation history) to the new filesystem public store.
- `kms`: Migrate legacy private key files (`<type>_<alg>.pem`) to the filesystem KMS and align the public keys.

### Key type identifiers

Use `-type` to choose the key group. For federation signing keys, `-type federation` (default) is typically used.

### Public key migration

Migrate legacy JWKS and rotation history to either filesystem storage (default) or the database-backed public key storage.

Filesystem destination (default):

```bash
./lhmigrate keys public -src <legacy_dir> -dst <dest_dir> -type <typeID>
```

Database destination:

```bash
# SQLite
./lhmigrate keys public -src <legacy_dir> -dst </path/to/sqlite_dir_or_db> -type <typeID> --dest-db sqlite

# MySQL
./lhmigrate keys public -src <legacy_dir> -dst <ignored> -type <typeID> \
  --dest-db mysql --dest-dsn 'user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True'

# PostgreSQL
./lhmigrate keys public -src <legacy_dir> -dst <ignored> -type <typeID> \
  --dest-db postgres --dest-dsn 'host=localhost user=gorm password=gorm dbname=gorm port=9920'
```

Flags:

- `-src`: Path to legacy public key storage directory (required)
- `-dst`: Destination for filesystem store, or SQLite file/dir (default: same as `-src`)
- `-type`: Key type identifier (default: `federation`)
- `--dest-db`: Destination database type (`sqlite|mysql|postgres`). If omitted, filesystem destination is used.
- `--dest-dsn`: DSN for MySQL/Postgres. Ignored for SQLite.
- `--db-debug`: Enable GORM debug logging.
- `-v`: Verbose CLI logging

Examples:

```bash
# Filesystem migration
./lhmigrate keys public \
  -src /var/lib/lighthouse/legacy-keys \
  -dst /var/lib/lighthouse/keys \
  -type federation

# SQLite DB migration (uses /var/lib/lighthouse/lighthouse.db if -dst is a directory)
./lhmigrate keys public \
  -src /var/lib/lighthouse/legacy-keys \
  -dst /var/lib/lighthouse \
  -type federation \
  --dest-db sqlite

# MySQL DB migration
./lhmigrate keys public \
  -src /var/lib/lighthouse/legacy-keys \
  -dst ignored \
  -type federation \
  --dest-db mysql \
  --dest-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

# PostgreSQL DB migration
./lhmigrate keys public \
  -src /var/lib/lighthouse/legacy-keys \
  -dst ignored \
  -type federation \
  --dest-db postgres \
  --dest-dsn 'host=localhost user=lh password=secret dbname=lighthouse port=9920'
```

### KMS (private key) migration

Expected legacy layout: one PEM per algorithm in `-src`, named `<type>_<alg>.pem` (e.g., `federation_ES256.pem`).

```bash
./lhmigrate keys kms -src <legacy_dir> -dst <dest_dir> -type <typeID> -algs <list> [options]
```

Flags:

- `-src`: Path to legacy key files directory (required)
- `-dst`: Destination directory for filesystem KMS and public store (default: same as `-src`)
- `-type`: Key type identifier (default: `federation`)
- `-algs`: Comma‑separated list of algorithms (e.g., `ES256,RS256`) (required)
- `-default`: Default algorithm to mark active after migration (optional)
- `-generate-missing`: Generate missing keys in destination if not present (optional)
- `-rsa-len`: RSA key length when generating (default: `4096`)
- `-v`: Verbose logging

Examples:

```bash
# Migrate ES256 and RS256 keys
./lhmigrate keys kms \
  -src /var/lib/lighthouse/legacy-keys \
  -dst /var/lib/lighthouse/keys \
  -type federation \
  -algs ES256,RS256

# Migrate ES256 and set it as default
./lhmigrate keys kms \
  -src /var/lib/lighthouse/legacy-keys \
  -dst /var/lib/lighthouse/keys \
  -type federation \
  -algs ES256 \
  -default ES256

# Generate missing keys in destination
./lhmigrate keys kms \
  -src /var/lib/lighthouse/legacy-keys \
  -dst /var/lib/lighthouse/keys \
  -type federation \
  -algs RS256 \
  -generate-missing -rsa-len 4096
```

## Data (DB) migration

The `db` subcommand migrates legacy storage (JSON file or BadgerDB) to the new GORM‑based storage backends.

### Data migration sections

The following data is migrated:

- **subordinates** - Subordinate entities and their JWKS, metadata, policies, and constraints
- **trust_marked_entities** - Trust mark subject eligibility status (active, blocked, pending)

### CLI Usage

```bash
./lhmigrate db \
  --source-type <json|badger> \
  --source-dir /path/to/source \
  --dest-type <sqlite|mysql|postgres> \
  [--dest-dir /path/to/sqlite] \
  [--dest-dsn "dsn for mysql/postgres"] \
  [--force] \
  [--dry-run] \
  [--only=<sections>] \
  [--skip=<sections>] \
  [-v]
```

### Flags

- `--source-type`: Source storage type (`json` or `badger`) - **required**
- `--source-dir`: Path to legacy data directory - **required**
- `--dest-type`: Destination database type (`sqlite`, `mysql`, or `postgres`) - default: `sqlite`
- `--dest-dir`: Destination data directory (for SQLite)
- `--dest-dsn`: Destination DSN (for MySQL/PostgreSQL)
- `--db-debug`: Enable GORM debug logging
- `--force`: Overwrite existing records
- `--dry-run`: Preview only, don't make changes
- `--only`: Comma-separated list of sections to migrate (default: all)
- `--skip`: Comma-separated list of sections to skip
- `-v`: Verbose logging

### Examples

```bash
# Migrate JSON file storage to SQLite
./lhmigrate db \
  --source-type=json \
  --source-dir=/var/lib/lighthouse/legacy \
  --dest-type=sqlite \
  --dest-dir=/var/lib/lighthouse

# Migrate BadgerDB to PostgreSQL
./lhmigrate db \
  --source-type=badger \
  --source-dir=/var/lib/lighthouse/badger \
  --dest-type=postgres \
  --dest-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse'

# Dry run to preview migration
./lhmigrate db \
  --source-type=json \
  --source-dir=/var/lib/lighthouse/legacy \
  --dest-dir=/var/lib/lighthouse \
  --dry-run -v

# Migrate only subordinates, skip trust marked entities
./lhmigrate db \
  --source-type=json \
  --source-dir=/var/lib/lighthouse/legacy \
  --dest-dir=/var/lib/lighthouse \
  --only=subordinates
```

### Important Notes

- Trust marked entities require trust mark specs to exist in the database first. Run `lhmigrate config2db` before `lhmigrate db` if you have trust mark specs in your config file.
- Per-subordinate `MetadataPolicyCrit` is no longer supported. A warning will be logged if this field is present in legacy data; consider migrating to the global setting via `config2db`.
- Existing records are skipped by default. Use `--force` to overwrite.

## Config migration

The `config` subcommand transforms legacy configuration files to the new format compatible with LightHouse 0.20.0+.

### Transformations applied

| Old Config | New Config | Notes |
|------------|------------|-------|
| `storage.backend` (json\|badger) | `storage.driver` (sqlite) | Legacy backends no longer supported |
| `signing.automatic_key_rollover` | `signing.key_rotation` | Renamed |
| `federation_data.entity_id` | `entity_id` | Moved to top level |

Deprecated fields are preserved with comments indicating they should be migrated to the database.

### Fields moved to database

The following fields are now managed in the database via the Admin API or `lhmigrate config2db`:

| Config Path | Migration Section | Description |
|-------------|-------------------|-------------|
| `signing.alg` | `alg` | Signing algorithm (e.g., ES256, RS256) |
| `signing.rsa_key_len` | `rsa_key_len` | RSA key length (e.g., 2048, 4096) |
| `signing.key_rotation` | `key_rotation` | Key rotation settings (enabled, interval, overlap) |
| `federation_data.authority_hints` | `authority_hints` | List of authority hint entity IDs |
| `federation_data.federation_entity_metadata` | `metadata` | Federation entity metadata (name, contacts, etc.) |
| `federation_data.constraints` | `constraints` | Subordinate statement constraints |
| `federation_data.metadata_policy_crit` | `metadata_policy_crit` | Critical metadata policy operators |
| `federation_data.metadata_policy_file` | `metadata_policies` | Metadata policies (loaded from JSON file) |
| `federation_data.configuration_lifetime` | `config_lifetime` | Entity configuration JWT lifetime |
| `endpoints.fetch.statement_lifetime` | `statement_lifetime` | Subordinate statement JWT lifetime |
| `federation_data.trust_mark_issuers` | `trust_mark_issuers` | Allowed trust mark issuers per type |
| `federation_data.trust_mark_owners` | `trust_mark_owners` | Trust mark owners per type |
| `endpoints.trust_mark.trust_mark_specs` | `trust_mark_specs` | Trust mark issuance specifications |

### Fields NOT migrated

The following configuration fields are **not** migrated by `config2db`:

| Config Path | Reason |
|-------------|--------|
| `federation_data.crit` | The `crit` attribute was used to mark critical claims in subordinate entity statements. This functionality has been replaced: additional claims can now be added to entity statements via the Admin API (`POST /admin/api/v1/subordinates/{id}/additional-claims`), and each claim can be individually marked as critical. Since the old config only specified which claims were critical but not the claim values themselves, there is nothing to migrate. |

### Fields that remain in config

The following fields remain in the configuration file:

| Config Path | Description |
|-------------|-------------|
| `entity_id` | The entity identifier (URI) - **required** |
| `server.*` | Server settings (port, TLS, trusted proxies) |
| `storage.*` | Storage driver configuration (sqlite, mysql, postgres) |
| `signing.kms` | Key management system (filesystem, pkcs11) |
| `signing.pk_backend` | Public key storage backend (filesystem, db) |
| `signing.auto_generate_keys` | Auto-generate missing keys |
| `signing.filesystem.*` | Filesystem KMS settings |
| `signing.pkcs11.*` | PKCS#11 HSM settings |
| `endpoints.*` | Endpoint paths and settings (except `statement_lifetime`) |
| `api.*` | Admin API settings |
| `stats.*` | Statistics collection settings |
| `logging.*` | Logging configuration |
| `cache.*` | Caching configuration |

!!! note "federation_data section deprecated"
    
    The entire `federation_data` section is deprecated. All its options are either 
    moved to top-level config (`entity_id`) or managed in the database. See 
    [Federation Data](config/federation_data.md) for migration details.

### CLI Usage

```bash
./lhmigrate config \
  --in <config.yaml> \
  [--out <updated.yaml>] \
  [--run-config2db] \
  [--db-driver <sqlite|mysql|postgres>] \
  [--db-dir <path>] \
  [--db-dsn <dsn>] \
  [--force] \
  [--dry-run] \
  [-v]
```

### Flags

- `--in`: Path to existing configuration file - **required**
- `--out`: Path to write updated configuration (default: stdout)
- `--run-config2db`: Also run config2db migration after transformation
- `--db-driver`: Database driver for config2db (`sqlite`, `mysql`, `postgres`)
- `--db-dir`: Data directory for config2db (for SQLite)
- `--db-dsn`: Database DSN for config2db (for MySQL/PostgreSQL)
- `--db-debug`: Enable GORM debug logging for config2db
- `--force`: Force overwrite in config2db
- `--dry-run`: Preview only, don't make changes
- `-v`: Verbose logging

### Examples

```bash
# Transform config and output to stdout
./lhmigrate config --in=old-config.yaml

# Transform and write to new file
./lhmigrate config --in=old-config.yaml --out=new-config.yaml

# Transform and also migrate values to database
./lhmigrate config \
  --in=old-config.yaml \
  --out=new-config.yaml \
  --run-config2db \
  --db-dir=/var/lib/lighthouse

# Dry run to preview changes
./lhmigrate config --in=old-config.yaml --dry-run -v
```

## Config to Database migration (config2db)

The `config2db` subcommand migrates configuration file values directly to the database without modifying the config file.

### Sections

- `alg` - Signing algorithm
- `rsa_key_len` - RSA key length
- `key_rotation` - Key rotation configuration
- `constraints` - Subordinate statement constraints
- `metadata_crit` - Metadata policy crit operators
- `metadata_policies` - Metadata policies
- `config_lifetime` - Entity configuration lifetime
- `statement_lifetime` - Subordinate statement lifetime
- `authority_hints` - Authority hints
- `metadata` - Federation entity metadata
- `trust_mark_specs` - Trust mark specifications
- `trust_mark_issuers` - Trust mark issuers
- `trust_mark_owners` - Trust mark owners

### CLI Usage

```bash
./lhmigrate config2db \
  --config=<config.yaml> \
  [--db-driver <sqlite|mysql|postgres>] \
  [--db-dir <path>] \
  [--db-dsn <dsn>] \
  [--only=<sections>] \
  [--skip=<sections>] \
  [--force] \
  [--dry-run] \
  [-v]
```

### Examples

```bash
# Migrate all config values to SQLite
./lhmigrate config2db --config=config.yaml --db-dir=/var/lib/lighthouse

# Migrate only signing options
./lhmigrate config2db \
  --config=config.yaml \
  --db-dir=/var/lib/lighthouse \
  --only=alg,rsa_key_len,key_rotation

# Dry run with verbose output
./lhmigrate config2db --config=config.yaml --db-dir=/var/lib/lighthouse --dry-run -v
```

## After migration

- Make sure Lighthouse points to the migrated key/data locations in your deployment.
- Back up your migrated directories and/or database.
- Validate signatures and application behavior in your environment.

## Troubleshooting

- Use `-v` to enable verbose logging for key migrations.
- Check file permissions and paths for both source and destination.
- Verify algorithm names (`ES256`, `RS256`, etc.) are correct for KMS migration.
