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

The `db` subcommand will migrate legacy storage (e.g., JSON/Badger) to the new GORM‑based storage backends. This is not implemented yet.

Planned CLI (subject to change):

```bash
./lhmigrate db \
  --source-type <json|badger> \
  --source-dir /path/to/source \
  --dest-type <sqlite|mysql|postgres> \
  [--dest-dir /path/to/sqlite] \
  [--dest-dsn "dsn for mysql/postgres"] \
  [--dry-run] [-v]
```

Status: Not implemented yet; running this subcommand prints a clear message and exits.

## Config migration

The `config` subcommand will assist with migrating or updating configuration files to the latest format. This is not implemented yet.

Planned CLI (subject to change):

```bash
./lhmigrate config --in config.yaml [--out updated.yaml] [-v]
```

Status: Not implemented yet; running this subcommand prints a clear message and exits.

## After migration

- Make sure Lighthouse points to the migrated key/data locations in your deployment.
- Back up your migrated directories and/or database.
- Validate signatures and application behavior in your environment.

## Troubleshooting

- Use `-v` to enable verbose logging for key migrations.
- Check file permissions and paths for both source and destination.
- Verify algorithm names (`ES256`, `RS256`, etc.) are correct for KMS migration.
