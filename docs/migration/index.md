---
icon: material/walk
---

# Migration to LightHouse 0.20.0

This page covers how to migrate to LightHouse >= 0.20.0.
LightHouse 0.20.0 is a major release with significant changes, including 
key management and storage backends.

!!! danger "Breaking Changes"
    You cannot directly upgrade from LightHouse <0.20.0 to 0.20.0.

    **You MUST migrate your deployment.**

We still try to make it as easy as possible to migrate your deployment.

!!! warning "Backup Before Migration"
    Before starting the migration process, create backups of:
    
    - Your configuration file (`config.yaml`)
    - Your data directory (e.g., `/var/lib/lighthouse`)
    - Your signing keys directory
    - Your "database"
    
    This ensures you can recover if anything goes wrong during migration.

!!! info "Understanding the Changes in LightHouse 0.20.0"

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
            - There is a migration tool to read a config file and apply it to the 
              database.
        - Most of the options can now be configured via environment variables.

## Quickstart

This quickstart covers the complete migration workflow. Run these commands in order.

!!! tip "Using Docker"
    If you're using the Docker image instead of building the tool locally, replace 
    `./lhmigrate` with the Docker command. Make sure to mount all required directories (and connect to the 
    db-container network if applicable):
    
    ```bash
    docker run --rm \
      -v ./:/data \
      --network db-net
      --entrypoint /lhmigrate
      go-oidfed/lighthouse \
      all \
      --config=/data/config.yaml \
      # ... other flags
    ```

Select your **previous storage backend** (JSON or BadgerDB) and your **target database** (SQLite, MySQL, or PostgreSQL):

=== "JSON to SQLite"
    === "All in one"
        ```bash
        # Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # Run all migration steps in one command
        ./lhmigrate all \
          --config=/etc/lighthouse/config.yaml \
          --keys-source=/var/lib/lighthouse/legacy-keys \
          --keys-dest=/var/lib/lighthouse/keys \
          --data-source=/var/lib/lighthouse/legacy \
          --source-type=json \
          --db-type=sqlite \
          --db-dir=/var/lib/lighthouse \
          --algs=ES256,RS256 \
          --pks-type=db
        ```

        This single command runs all migration steps in sequence:

        1. **config** - Transform legacy config file to new format
        2. **keys-public** - Migrate public keys (JWKS + history) to database
        3. **keys-kms** - Migrate private keys to filesystem KMS
        4. **config2db** - Migrate config values to database (with `--update-config`)
        5. **db** - Migrate legacy storage data (subordinates, trust marks)
        6. **config-cleanup** - Remove empty/leftover values from config file

        !!! tip "Dry Run"
            Add `--dry-run -v` to preview all changes without applying them:
            ```bash
            ./lhmigrate all \
              --config=/etc/lighthouse/config.yaml \
              --keys-source=/var/lib/lighthouse/legacy-keys \
              --keys-dest=/var/lib/lighthouse/keys \
              --data-source=/var/lib/lighthouse/legacy \
              --source-type=json \
              --db-type=sqlite \
              --db-dir=/var/lib/lighthouse \
              --algs=ES256,RS256 \
              --pks-type=db \
              --dry-run -v
            ```

        !!! tip "Skip Steps"
            Use `--skip` to skip specific steps:
            ```bash
            # Skip key migrations if you don't have legacy keys
            ./lhmigrate all \
              --config=/etc/lighthouse/config.yaml \
              --data-source=/var/lib/lighthouse/legacy \
              --source-type=json \
              --db-type=sqlite \
              --db-dir=/var/lib/lighthouse \
              --skip=keys-public,keys-kms
            ```

    === "Detailed"
        ```bash
        # 1. Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # 2. Migrate signing keys
        # 2a. Migrate public keys (JWKS + rotation history) to database
        ./lhmigrate keys public \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse \
          --type federation \
          --db-type sqlite

        # 2b. Migrate private keys to filesystem KMS (with DB-based public key storage)
        ./lhmigrate keys kms \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse/keys \
          --type federation \
          --algs ES256,RS256 \
          --pks-type db --db-type sqlite

        # 3. Transform configuration file to new format (with correct DB settings)
        ./lhmigrate config \
          --source=/etc/lighthouse/config.yaml \
          --dest=/etc/lighthouse/config-new.yaml \
          --db-type=sqlite \
          --db-dir=/var/lib/lighthouse

        # 4. Migrate configuration values to database and clean up config file
        # IMPORTANT: Run this BEFORE migrating data (step 5)
        # Trust mark specs must exist in the DB before trust marked entities can be migrated
        ./lhmigrate config2db \
          --config=/etc/lighthouse/config-new.yaml \
          --db-dir=/var/lib/lighthouse \
          --update-config

        # 5. Migrate data (subordinates, trust marked entities)
        ./lhmigrate db \
          --source-type=json \
          --source=/var/lib/lighthouse/legacy \
          --db-type=sqlite \
          --dest=/var/lib/lighthouse

        # 6. Update your deployment to use the new config and data locations
        mv /etc/lighthouse/config-new.yaml /etc/lighthouse/config.yaml
        ```

        !!! tip "Dry Run"
            Add `--dry-run -v` (or `-n -v`) to any command to preview changes without applying them.

        !!! tip "Combined Config Migration"
            Steps 3 and 4 can be combined into a single command:
            ```bash
            ./lhmigrate config \
              --source=/etc/lighthouse/config.yaml \
              --dest=/etc/lighthouse/config-new.yaml \
              --db-type=sqlite \
              --db-dir=/var/lib/lighthouse \
              --run-config2db
            ```

=== "BadgerDB to SQLite"
    === "All in one"
        ```bash
        # Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # Run all migration steps in one command
        ./lhmigrate all \
          --config=/etc/lighthouse/config.yaml \
          --keys-source=/var/lib/lighthouse/legacy-keys \
          --keys-dest=/var/lib/lighthouse/keys \
          --data-source=/var/lib/lighthouse/badger \
          --source-type=badger \
          --db-type=sqlite \
          --db-dir=/var/lib/lighthouse \
          --algs=ES256,RS256 \
          --pks-type=db
        ```

        This single command runs all migration steps in sequence:

        1. **config** - Transform legacy config file to new format
        2. **keys-public** - Migrate public keys (JWKS + history) to database
        3. **keys-kms** - Migrate private keys to filesystem KMS
        4. **config2db** - Migrate config values to database (with `--update-config`)
        5. **db** - Migrate legacy storage data (subordinates, trust marks)
        6. **config-cleanup** - Remove empty/leftover values from config file

        !!! tip "Dry Run"
            Add `--dry-run -v` to preview all changes without applying them.

        !!! tip "Skip Steps"
            Use `--skip` to skip specific steps (e.g., `--skip=keys-public,keys-kms`).

    === "Detailed"
        ```bash
        # 1. Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # 2. Migrate signing keys
        # 2a. Migrate public keys (JWKS + rotation history) to database
        ./lhmigrate keys public \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse \
          --type federation \
          --db-type sqlite

        # 2b. Migrate private keys to filesystem KMS (with DB-based public key storage)
        ./lhmigrate keys kms \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse/keys \
          --type federation \
          --algs ES256,RS256 \
          --pks-type db --db-type sqlite

        # 3. Transform configuration file to new format (with correct DB settings)
        ./lhmigrate config \
          --source=/etc/lighthouse/config.yaml \
          --dest=/etc/lighthouse/config-new.yaml \
          --db-type=sqlite \
          --db-dir=/var/lib/lighthouse

        # 4. Migrate configuration values to database and clean up config file
        # IMPORTANT: Run this BEFORE migrating data (step 5)
        # Trust mark specs must exist in the DB before trust marked entities can be migrated
        ./lhmigrate config2db \
          --config=/etc/lighthouse/config-new.yaml \
          --db-dir=/var/lib/lighthouse \
          --update-config

        # 5. Migrate data (subordinates, trust marked entities)
        ./lhmigrate db \
          --source-type=badger \
          --source=/var/lib/lighthouse/badger \
          --db-type=sqlite \
          --dest=/var/lib/lighthouse

        # 6. Update your deployment to use the new config and data locations
        mv /etc/lighthouse/config-new.yaml /etc/lighthouse/config.yaml
        ```

        !!! tip "Dry Run"
            Add `--dry-run -v` (or `-n -v`) to any command to preview changes without applying them.

        !!! tip "Combined Config Migration"
            Steps 3 and 4 can be combined into a single command:
            ```bash
            ./lhmigrate config \
              --source=/etc/lighthouse/config.yaml \
              --dest=/etc/lighthouse/config-new.yaml \
              --db-type=sqlite \
              --db-dir=/var/lib/lighthouse \
              --run-config2db
            ```

=== "JSON to MySQL"
    === "All in one"
        ```bash
        # Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # Run all migration steps in one command
        ./lhmigrate all \
          --config=/etc/lighthouse/config.yaml \
          --keys-source=/var/lib/lighthouse/legacy-keys \
          --keys-dest=/var/lib/lighthouse/keys \
          --data-source=/var/lib/lighthouse/legacy \
          --source-type=json \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True' \
          --algs=ES256,RS256 \
          --pks-type=db
        ```

        This single command runs all migration steps in sequence:

        1. **config** - Transform legacy config file to new format
        2. **keys-public** - Migrate public keys (JWKS + history) to database
        3. **keys-kms** - Migrate private keys to filesystem KMS
        4. **config2db** - Migrate config values to database (with `--update-config`)
        5. **db** - Migrate legacy storage data (subordinates, trust marks)
        6. **config-cleanup** - Remove empty/leftover values from config file

        !!! tip "Dry Run"
            Add `--dry-run -v` to preview all changes without applying them.

        !!! tip "Skip Steps"
            Use `--skip` to skip specific steps (e.g., `--skip=keys-public,keys-kms`).

    === "Detailed"
        ```bash
        # 1. Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # 2. Migrate signing keys
        # 2a. Migrate public keys (JWKS + rotation history) to database
        ./lhmigrate keys public \
          --source /var/lib/lighthouse/legacy-keys \
          --type federation \
          --db-type mysql \
          --db-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 2b. Migrate private keys to filesystem KMS (with DB-based public key storage)
        ./lhmigrate keys kms \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse/keys \
          --type federation \
          --algs ES256,RS256 \
          --pks-type db --db-type mysql \
          --db-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 3. Transform configuration file to new format (with correct DB settings)
        ./lhmigrate config \
          --source=/etc/lighthouse/config.yaml \
          --dest=/etc/lighthouse/config-new.yaml \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 4. Migrate configuration values to database and clean up config file
        # IMPORTANT: Run this BEFORE migrating data (step 5)
        # Trust mark specs must exist in the DB before trust marked entities can be migrated
        ./lhmigrate config2db \
          --config=/etc/lighthouse/config-new.yaml \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True' \
          --update-config

        # 5. Migrate data (subordinates, trust marked entities)
        ./lhmigrate db \
          --source-type=json \
          --source=/var/lib/lighthouse/legacy \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 6. Update your deployment to use the new config and data locations
        mv /etc/lighthouse/config-new.yaml /etc/lighthouse/config.yaml
        ```

        !!! tip "Dry Run"
            Add `--dry-run -v` (or `-n -v`) to any command to preview changes without applying them.

        !!! tip "Combined Config Migration"
            Steps 3 and 4 can be combined into a single command:
            ```bash
            ./lhmigrate config \
              --source=/etc/lighthouse/config.yaml \
              --dest=/etc/lighthouse/config-new.yaml \
              --db-type=mysql \
              --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True' \
              --run-config2db
            ```

=== "BadgerDB to MySQL"
    === "All in one"
        ```bash
        # Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # Run all migration steps in one command
        ./lhmigrate all \
          --config=/etc/lighthouse/config.yaml \
          --keys-source=/var/lib/lighthouse/legacy-keys \
          --keys-dest=/var/lib/lighthouse/keys \
          --data-source=/var/lib/lighthouse/badger \
          --source-type=badger \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True' \
          --algs=ES256,RS256 \
          --pks-type=db
        ```

        This single command runs all migration steps in sequence:

        1. **config** - Transform legacy config file to new format
        2. **keys-public** - Migrate public keys (JWKS + history) to database
        3. **keys-kms** - Migrate private keys to filesystem KMS
        4. **config2db** - Migrate config values to database (with `--update-config`)
        5. **db** - Migrate legacy storage data (subordinates, trust marks)
        6. **config-cleanup** - Remove empty/leftover values from config file

        !!! tip "Dry Run"
            Add `--dry-run -v` to preview all changes without applying them.

        !!! tip "Skip Steps"
            Use `--skip` to skip specific steps (e.g., `--skip=keys-public,keys-kms`).

    === "Detailed"
        ```bash
        # 1. Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # 2. Migrate signing keys
        # 2a. Migrate public keys (JWKS + rotation history) to database
        ./lhmigrate keys public \
          --source /var/lib/lighthouse/legacy-keys \
          --type federation \
          --db-type mysql \
          --db-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 2b. Migrate private keys to filesystem KMS (with DB-based public key storage)
        ./lhmigrate keys kms \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse/keys \
          --type federation \
          --algs ES256,RS256 \
          --pks-type db --db-type mysql \
          --db-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 3. Transform configuration file to new format (with correct DB settings)
        ./lhmigrate config \
          --source=/etc/lighthouse/config.yaml \
          --dest=/etc/lighthouse/config-new.yaml \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 4. Migrate configuration values to database and clean up config file
        # IMPORTANT: Run this BEFORE migrating data (step 5)
        # Trust mark specs must exist in the DB before trust marked entities can be migrated
        ./lhmigrate config2db \
          --config=/etc/lighthouse/config-new.yaml \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True' \
          --update-config

        # 5. Migrate data (subordinates, trust marked entities)
        ./lhmigrate db \
          --source-type=badger \
          --source=/var/lib/lighthouse/badger \
          --db-type=mysql \
          --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

        # 6. Update your deployment to use the new config and data locations
        mv /etc/lighthouse/config-new.yaml /etc/lighthouse/config.yaml
        ```

        !!! tip "Dry Run"
            Add `--dry-run -v` (or `-n -v`) to any command to preview changes without applying them.

        !!! tip "Combined Config Migration"
            Steps 3 and 4 can be combined into a single command:
            ```bash
            ./lhmigrate config \
              --source=/etc/lighthouse/config.yaml \
              --dest=/etc/lighthouse/config-new.yaml \
              --db-type=mysql \
              --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True' \
              --run-config2db
            ```

=== "JSON to PostgreSQL"
    === "All in one"
        ```bash
        # Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # Run all migration steps in one command
        ./lhmigrate all \
          --config=/etc/lighthouse/config.yaml \
          --keys-source=/var/lib/lighthouse/legacy-keys \
          --keys-dest=/var/lib/lighthouse/keys \
          --data-source=/var/lib/lighthouse/legacy \
          --source-type=json \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432' \
          --algs=ES256,RS256 \
          --pks-type=db
        ```

        This single command runs all migration steps in sequence:

        1. **config** - Transform legacy config file to new format
        2. **keys-public** - Migrate public keys (JWKS + history) to database
        3. **keys-kms** - Migrate private keys to filesystem KMS
        4. **config2db** - Migrate config values to database (with `--update-config`)
        5. **db** - Migrate legacy storage data (subordinates, trust marks)
        6. **config-cleanup** - Remove empty/leftover values from config file

        !!! tip "Dry Run"
            Add `--dry-run -v` to preview all changes without applying them.

        !!! tip "Skip Steps"
            Use `--skip` to skip specific steps (e.g., `--skip=keys-public,keys-kms`).

    === "Detailed"
        ```bash
        # 1. Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # 2. Migrate signing keys
        # 2a. Migrate public keys (JWKS + rotation history) to database
        ./lhmigrate keys public \
          --source /var/lib/lighthouse/legacy-keys \
          --type federation \
          --db-type postgres \
          --db-dsn 'host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 2b. Migrate private keys to filesystem KMS (with DB-based public key storage)
        ./lhmigrate keys kms \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse/keys \
          --type federation \
          --algs ES256,RS256 \
          --pks-type db --db-type postgres \
          --db-dsn 'host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 3. Transform configuration file to new format (with correct DB settings)
        ./lhmigrate config \
          --source=/etc/lighthouse/config.yaml \
          --dest=/etc/lighthouse/config-new.yaml \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 4. Migrate configuration values to database and clean up config file
        # IMPORTANT: Run this BEFORE migrating data (step 5)
        # Trust mark specs must exist in the DB before trust marked entities can be migrated
        ./lhmigrate config2db \
          --config=/etc/lighthouse/config-new.yaml \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432' \
          --update-config

        # 5. Migrate data (subordinates, trust marked entities)
        ./lhmigrate db \
          --source-type=json \
          --source=/var/lib/lighthouse/legacy \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 6. Update your deployment to use the new config and data locations
        mv /etc/lighthouse/config-new.yaml /etc/lighthouse/config.yaml
        ```

        !!! tip "Dry Run"
            Add `--dry-run -v` (or `-n -v`) to any command to preview changes without applying them.

        !!! tip "Combined Config Migration"
            Steps 3 and 4 can be combined into a single command:
            ```bash
            ./lhmigrate config \
              --source=/etc/lighthouse/config.yaml \
              --dest=/etc/lighthouse/config-new.yaml \
              --db-type=postgres \
              --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432' \
              --run-config2db
            ```

=== "BadgerDB to PostgreSQL"
    === "All in one"
        ```bash
        # Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # Run all migration steps in one command
        ./lhmigrate all \
          --config=/etc/lighthouse/config.yaml \
          --keys-source=/var/lib/lighthouse/legacy-keys \
          --keys-dest=/var/lib/lighthouse/keys \
          --data-source=/var/lib/lighthouse/badger \
          --source-type=badger \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432' \
          --algs=ES256,RS256 \
          --pks-type=db
        ```

        This single command runs all migration steps in sequence:

        1. **config** - Transform legacy config file to new format
        2. **keys-public** - Migrate public keys (JWKS + history) to database
        3. **keys-kms** - Migrate private keys to filesystem KMS
        4. **config2db** - Migrate config values to database (with `--update-config`)
        5. **db** - Migrate legacy storage data (subordinates, trust marks)
        6. **config-cleanup** - Remove empty/leftover values from config file

        !!! tip "Dry Run"
            Add `--dry-run -v` to preview all changes without applying them.

        !!! tip "Skip Steps"
            Use `--skip` to skip specific steps (e.g., `--skip=keys-public,keys-kms`).

    === "Detailed"
        ```bash
        # 1. Build the migration tool (or use the docker image)
        go build -o lhmigrate ./cmd/lhmigrate

        # 2. Migrate signing keys
        # 2a. Migrate public keys (JWKS + rotation history) to database
        ./lhmigrate keys public \
          --source /var/lib/lighthouse/legacy-keys \
          --type federation \
          --db-type postgres \
          --db-dsn 'host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 2b. Migrate private keys to filesystem KMS (with DB-based public key storage)
        ./lhmigrate keys kms \
          --source /var/lib/lighthouse/legacy-keys \
          --dest /var/lib/lighthouse/keys \
          --type federation \
          --algs ES256,RS256 \
          --pks-type db --db-type postgres \
          --db-dsn 'host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 3. Transform configuration file to new format (with correct DB settings)
        ./lhmigrate config \
          --source=/etc/lighthouse/config.yaml \
          --dest=/etc/lighthouse/config-new.yaml \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 4. Migrate configuration values to database and clean up config file
        # IMPORTANT: Run this BEFORE migrating data (step 5)
        # Trust mark specs must exist in the DB before trust marked entities can be migrated
        ./lhmigrate config2db \
          --config=/etc/lighthouse/config-new.yaml \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432' \
          --update-config

        # 5. Migrate data (subordinates, trust marked entities)
        ./lhmigrate db \
          --source-type=badger \
          --source=/var/lib/lighthouse/badger \
          --db-type=postgres \
          --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

        # 6. Update your deployment to use the new config and data locations
        mv /etc/lighthouse/config-new.yaml /etc/lighthouse/config.yaml
        ```

        !!! tip "Dry Run"
            Add `--dry-run -v` (or `-n -v`) to any command to preview changes without applying them.

        !!! tip "Combined Config Migration"
            Steps 3 and 4 can be combined into a single command:
            ```bash
            ./lhmigrate config \
              --source=/etc/lighthouse/config.yaml \
              --dest=/etc/lighthouse/config-new.yaml \
              --db-type=postgres \
              --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse port=5432' \
              --run-config2db
            ```


## Build the tool

From the repository root:

```bash
go build -o lhmigrate ./cmd/lhmigrate
```

The `lhmigrate` tool is also already included in the docker image.

## Signing keys

The key migrations live under the `keys` command (alias: `signing`). There are two subcommands:

- `public`: Migrate legacy public key storage (JWKS + rotation history) to the new filesystem public store.
- `kms`: Migrate legacy private key files (`<type>_<alg>.pem`) to the filesystem KMS and align the public keys.

### Key type identifiers

Use `--type` (or `-t`) to choose the key group. For federation signing keys, `--type federation` (default) is typically used.

### Public key migration

Migrate legacy JWKS and rotation history to either filesystem storage (default) or the database-backed public key storage.

Filesystem destination (default):

```bash
./lhmigrate keys public --source <legacy_dir> --dest <dest_dir> --type <typeID>
```

Database destination:

```bash
# SQLite
./lhmigrate keys public --source <legacy_dir> --dest </path/to/sqlite_dir_or_db> --type <typeID> --db-type sqlite

# MySQL
./lhmigrate keys public --source <legacy_dir> --type <typeID> \
  --db-type mysql --db-dsn 'user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True'

# PostgreSQL
./lhmigrate keys public --source <legacy_dir> --type <typeID> \
  --db-type postgres --db-dsn 'host=localhost user=gorm password=gorm dbname=gorm port=9920'
```

Flags:

- `--source`, `-s`: Path to legacy public key storage directory (required)
- `--dest`, `-d`: Destination for filesystem store, or SQLite file/dir (default: same as `--source`)
- `--type`, `-t`: Key type identifier (default: `federation`)
- `--db-type`: Destination database type (`sqlite|mysql|postgres`). If omitted, filesystem destination is used.
- `--db-dsn`: DSN for MySQL/Postgres. Ignored for SQLite.
- `--db-debug`: Enable GORM debug logging.
- `--verbose`, `-v`: Verbose CLI logging

Examples:

```bash
# Filesystem migration
./lhmigrate keys public \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation

# SQLite DB migration (uses /var/lib/lighthouse/lighthouse.db if --dest is a directory)
./lhmigrate keys public \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse \
  --type federation \
  --db-type sqlite

# MySQL DB migration
./lhmigrate keys public \
  --source /var/lib/lighthouse/legacy-keys \
  --type federation \
  --db-type mysql \
  --db-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

# PostgreSQL DB migration
./lhmigrate keys public \
  --source /var/lib/lighthouse/legacy-keys \
  --type federation \
  --db-type postgres \
  --db-dsn 'host=localhost user=lh password=secret dbname=lighthouse port=9920'
```

### KMS (private key) migration

Expected legacy layout: one PEM per algorithm in `--source`, named `<type>_<alg>.pem` (e.g., `federation_ES256.pem`).

```bash
./lhmigrate keys kms --source <legacy_dir> --dest <dest_dir> --type <typeID> --algs <list> --pks-type <fs|db> [options]
```

Flags:

- `--source`, `-s`: Path to legacy key files directory (required)
- `--dest`, `-d`: Destination directory for filesystem KMS (and public storage if `--pks-type=fs`)
- `--type`, `-t`: Key type identifier (default: `federation`)
- `--algs`, `-a`: Comma‑separated list of algorithms (e.g., `ES256,RS256`) (required)
- `--pks-type`: Public key storage type: `fs` (filesystem) or `db` (database) (required)
- `--db-type`: Database type for public key storage: `sqlite`, `mysql`, or `postgres` (required when `--pks-type=db`)
- `--db-dsn`: Database DSN for MySQL/PostgreSQL (ignored for SQLite)
- `--db-debug`: Enable GORM debug logging for DB operations
- `--default`: Default algorithm to mark active after migration (optional)
- `--generate-missing`, `-g`: Generate missing keys in destination if not present (optional)
- `--rsa-len`: RSA key length when generating (default: `4096`)
- `--verbose`, `-v`: Verbose logging

Examples:

```bash
# Migrate ES256 and RS256 keys with filesystem public key storage
./lhmigrate keys kms \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation \
  --algs ES256,RS256 \
  --pks-type fs

# Migrate keys with SQLite-based public key storage
./lhmigrate keys kms \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation \
  --algs ES256,RS256 \
  --pks-type db --db-type sqlite

# Migrate keys with MySQL-based public key storage
./lhmigrate keys kms \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation \
  --algs ES256,RS256 \
  --pks-type db --db-type mysql \
  --db-dsn 'user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

# Migrate keys with PostgreSQL-based public key storage
./lhmigrate keys kms \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation \
  --algs ES256,RS256 \
  --pks-type db --db-type postgres \
  --db-dsn 'host=localhost user=lighthouse password=secret dbname=lighthouse port=5432'

# Migrate ES256, set it as default, with DB storage
./lhmigrate keys kms \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation \
  --algs ES256 \
  --default ES256 \
  --pks-type db --db-type sqlite

# Generate missing keys in destination with filesystem storage
./lhmigrate keys kms \
  --source /var/lib/lighthouse/legacy-keys \
  --dest /var/lib/lighthouse/keys \
  --type federation \
  --algs RS256 \
  --generate-missing --rsa-len 4096 \
  --pks-type fs
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
  --source /path/to/source \
  --db-type <sqlite|mysql|postgres> \
  [--dest /path/to/sqlite] \
  [--db-dsn "dsn for mysql/postgres"] \
  [--force] \
  [--dry-run] \
  [--only=<sections>] \
  [--skip=<sections>] \
  [-v]
```

### Flags

- `--source-type`: Source storage type (`json` or `badger`) - **required**
- `--source`, `-s`: Path to legacy data directory - **required**
- `--db-type`: Destination database type (`sqlite`, `mysql`, or `postgres`) - default: `sqlite`
- `--dest`, `-d`: Destination data directory (for SQLite)
- `--db-dsn`: Destination DSN (for MySQL/PostgreSQL)
- `--db-debug`: Enable GORM debug logging
- `--force`, `-f`: Overwrite existing records
- `--dry-run`, `-n`: Preview only, don't make changes
- `--only`: Comma-separated list of sections to migrate (default: all)
- `--skip`: Comma-separated list of sections to skip
- `--verbose`, `-v`: Verbose logging

### Examples

```bash
# Migrate JSON file storage to SQLite
./lhmigrate db \
  --source-type=json \
  --source=/var/lib/lighthouse/legacy \
  --db-type=sqlite \
  --dest=/var/lib/lighthouse

# Migrate BadgerDB to PostgreSQL
./lhmigrate db \
  --source-type=badger \
  --source=/var/lib/lighthouse/badger \
  --db-type=postgres \
  --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse'

# Dry run to preview migration
./lhmigrate db \
  --source-type=json \
  --source=/var/lib/lighthouse/legacy \
  --dest=/var/lib/lighthouse \
  --dry-run -v

# Migrate only subordinates, skip trust marked entities
./lhmigrate db \
  --source-type=json \
  --source=/var/lib/lighthouse/legacy \
  --dest=/var/lib/lighthouse \
  --only=subordinates
```

### Important Notes

- Trust marked entities require trust mark specs to exist in the database first. Run `lhmigrate config2db` before `lhmigrate db` if you have trust mark specs in your config file.
- Per-subordinate `MetadataPolicyCrit` is no longer supported. A warning will be logged if this field is present in legacy data; consider migrating to the global setting via `config2db`.
- Existing records are skipped by default. Use `--force` to overwrite.

## Config migration

The `config` subcommand transforms legacy configuration files to the new format compatible with LightHouse 0.20.0+.

The `--db-type`, `--db-dsn`, and `--db-dir` options configure the `storage` section in the output config file, ensuring it has the correct database driver and connection settings. These options are also passed to `config2db` if `--run-config2db` is set.

### Transformations applied

| Old Config | New Config | Notes |
|------------|------------|-------|
| `storage.backend` (json\|badger) | `storage.driver` | Set from `--db-type` (default: sqlite) |
| `storage.dsn` | `storage.dsn` | Set from `--db-dsn` (for mysql/postgres) |
| `storage.data_dir` | `storage.data_dir` | Set from `--db-dir` (for sqlite) |
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
| `federation_data.extra_entity_configuration_data` | `extra_entity_config` | Extra claims for entity configuration |
| `endpoints.fetch.statement_lifetime` | `statement_lifetime` | Subordinate statement JWT lifetime |
| `federation_data.trust_mark_issuers` | `trust_mark_issuers` | Allowed trust mark issuers per type |
| `federation_data.trust_mark_owners` | `trust_mark_owners` | Trust mark owners per type |
| `endpoints.trust_mark.trust_mark_specs` | `trust_mark_specs` | Trust mark issuance specifications |

### Fields NOT migrated

The following configuration fields are **not** migrated by `config2db`:

| Config Path | Reason |
|-------------|--------|
| `federation_data.crit` | The `crit` attribute was used to mark critical claims in subordinate entity statements. This functionality has been replaced: additional claims can now be added to entity statements via the Admin API (`POST /admin/api/v1/subordinates/{id}/additional-claims`), and each claim can be individually marked as critical. Since the old config only specified which claims were critical but not the claim values themselves, there is nothing to migrate. |
| `federation_data.trust_anchors` | This field was not used at runtime and is therefore not migrated. |

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
  --source <config.yaml> \
  [--dest <updated.yaml>] \
  [--run-config2db] \
  [--db-type <sqlite|mysql|postgres>] \
  [--db-dir <path>] \
  [--db-dsn <dsn>] \
  [--force] \
  [--dry-run] \
  [-v]
```

### Flags

- `--source`, `-s`: Path to existing configuration file - **required**
- `--dest`, `-d`: Path to write updated configuration (default: stdout)
- `--db-type`: Database type (`sqlite`, `mysql`, `postgres`) - configures `storage.driver` in output (default: `sqlite`)
- `--db-dir`: Data directory - configures `storage.data_dir` in output (for SQLite)
- `--db-dsn`: Database DSN - configures `storage.dsn` in output (for MySQL/PostgreSQL)
- `--run-config2db`: Also run config2db migration after transformation
- `--db-debug`: Enable GORM debug logging for config2db
- `--force`, `-f`: Force overwrite in config2db
- `--dry-run`, `-n`: Preview only, don't make changes
- `--verbose`, `-v`: Verbose logging

### Examples

```bash
# Transform config for SQLite
./lhmigrate config \
  --source=old-config.yaml \
  --dest=new-config.yaml \
  --db-type=sqlite \
  --db-dir=/var/lib/lighthouse

# Transform config for PostgreSQL
./lhmigrate config \
  --source=old-config.yaml \
  --dest=new-config.yaml \
  --db-type=postgres \
  --db-dsn='host=localhost user=lighthouse password=secret dbname=lighthouse'

# Transform config for MySQL
./lhmigrate config \
  --source=old-config.yaml \
  --dest=new-config.yaml \
  --db-type=mysql \
  --db-dsn='user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True'

# Transform and also migrate values to database (combined)
./lhmigrate config \
  --source=old-config.yaml \
  --dest=new-config.yaml \
  --db-type=sqlite \
  --db-dir=/var/lib/lighthouse \
  --run-config2db

# Dry run to preview changes
./lhmigrate config --source=old-config.yaml --db-dir=/var/lib/lighthouse --dry-run -v
```

## Config to Database migration (config2db)

The `config2db` subcommand migrates configuration file values directly to the database. Optionally, it can also remove the migrated options from the config file using the `--update-config` flag.

### Sections

- `alg` - Signing algorithm
- `rsa_key_len` - RSA key length
- `key_rotation` - Key rotation configuration
- `constraints` - Subordinate statement constraints
- `metadata_crit` - Metadata policy crit operators
- `metadata_policies` - Metadata policies
- `config_lifetime` - Entity configuration lifetime
- `extra_entity_config` - Extra entity configuration claims
- `statement_lifetime` - Subordinate statement lifetime
- `authority_hints` - Authority hints
- `metadata` - Federation entity metadata
- `trust_mark_specs` - Trust mark specifications (for issuance)
- `trust_marks` - Entity configuration trust marks (published in own entity configuration)
- `trust_mark_issuers` - Trust mark issuers
- `trust_mark_owners` - Trust mark owners

### CLI Usage

```bash
./lhmigrate config2db \
  --config=<config.yaml> \
  [--db-type <sqlite|mysql|postgres>] \
  [--db-dir <path>] \
  [--db-dsn <dsn>] \
  [--only=<sections>] \
  [--skip=<sections>] \
  [--update-config] \
  [--force] \
  [--dry-run] \
  [-v]
```

### Flags

- `--config`, `-c`: Path to config file to migrate - **required**
- `--db-type`: Database type (`sqlite`, `mysql`, `postgres`) - default: `sqlite`
- `--db-dir`: Data directory (for SQLite)
- `--db-dsn`: Database DSN (for MySQL/PostgreSQL)
- `--db-debug`: Enable GORM debug logging
- `--only`: Comma-separated list of sections to migrate (default: all)
- `--skip`: Comma-separated list of sections to skip
- `--update-config`: Remove successfully migrated options from the config file
- `--validate`: Validate config values before migration (default: true)
- `--force`, `-f`: Overwrite existing values in DB
- `--dry-run`, `-n`: Show what would be written without actually writing
- `--verbose`, `-v`: Verbose logging

### Examples

```bash
# Migrate all config values to SQLite
./lhmigrate config2db --config=config.yaml --db-dir=/var/lib/lighthouse

# Migrate and remove migrated options from config file
./lhmigrate config2db \
  --config=config.yaml \
  --db-dir=/var/lib/lighthouse \
  --update-config

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
