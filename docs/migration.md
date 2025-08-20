---
icon: material/database-sync
---

# Storage Migration

This document describes how to migrate data from one storage backend to another in Lighthouse.

## Migration Tool

Lighthouse includes a migration tool called `lhmigrate` that can migrate data from JSON or Badger storage to the new GORM-based storage. The tool is designed to be simple to use and provides a way to migrate all your data without losing any information.

### Building the Migration Tool

To build the migration tool, run the following command from the root of the Lighthouse repository:

```bash
go build -o lhmigrate ./cmd/lhmigrate
```

### Using the Migration Tool

The migration tool supports migrating from JSON or Badger storage to any of the supported GORM database backends (SQLite, MySQL, or PostgreSQL).

#### Basic Usage

```bash
./lhmigrate migrate --source-type=json --source-dir=/path/to/source/data --dest-type=sqlite --dest-dir=/path/to/destination/data
```

#### Command-line Options

- `--source-type`: The source storage type (json or badger)
- `--source-dir`: The source data directory
- `--dest-type`: The destination database type (sqlite, mysql, or postgres)
- `--dest-dir`: The destination data directory (for sqlite)
- `--dest-dsn`: The destination DSN (for mysql and postgres)
- `--verbose`: Enable verbose logging
- `--dry-run`: Perform a dry run without writing to destination

#### Migration Examples

##### Migrating from JSON to SQLite

```bash
./lhmigrate migrate --source-type=json --source-dir=/path/to/json/data --dest-type=sqlite --dest-dir=/path/to/sqlite/data
```

##### Migrating from Badger to MySQL

```bash
./lhmigrate migrate --source-type=badger --source-dir=/path/to/badger/data --dest-type=mysql --dest-dsn="user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True&loc=Local"
```

##### Migrating from JSON to PostgreSQL

```bash
./lhmigrate migrate --source-type=json --source-dir=/path/to/json/data --dest-type=postgres --dest-dsn="host=localhost user=postgres password=postgres dbname=lighthouse port=5432 sslmode=disable TimeZone=UTC"
```

##### Performing a Dry Run

To test the migration without actually writing to the destination, use the `--dry-run` flag:

```bash
./lhmigrate migrate --source-type=json --source-dir=/path/to/json/data --dest-type=sqlite --dest-dir=/path/to/sqlite/data --dry-run
```

### Migration Process

The migration tool performs the following steps:

1. Loads the source storage (JSON or Badger)
2. Initializes the destination storage (SQLite, MySQL, or PostgreSQL)
3. Migrates all subordinate entities (active, blocked, and pending)
4. Migrates all trust marked entities (active, blocked, and pending)

The tool will display progress information as it migrates the data, showing how many entities are being migrated and their status.

### After Migration

After migrating your data, you should update your Lighthouse configuration to use the new storage backend. For example, if you migrated to SQLite, your configuration would look like:

```yaml
storage:
    backend: gorm
    data_dir: /path/to/sqlite/data
    database:
        driver: sqlite
        debug: false
```

Or if you migrated to MySQL:

```yaml
storage:
    backend: gorm
    database:
        driver: mysql
        dsn: "user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True&loc=Local"
        debug: false
```

## Troubleshooting

If you encounter any issues during migration, try the following:

1. Use the `--verbose` flag to get more detailed logging
2. Perform a dry run first to check for any potential issues
3. Make sure you have backup copies of your data before migration
4. Check that the destination database is accessible and has the correct permissions