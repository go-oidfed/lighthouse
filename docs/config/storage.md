---
icon: material/database
---
<span class="badge badge-red" title="If this option is required or optional">required</span>

The `storage` option is used to configure how and where data is stored.

LightHouse uses SQL databases for data storage. SQLite, MySQL, and PostgreSQL are supported.

## `driver`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">sqlite</span>
<span class="badge badge-red" title="If this option is required or optional">required</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STORAGE_DRIVER`</span>

The `driver` option specifies which database driver to use.

Supported values:

- `sqlite` - SQLite database (default, file-based)
- `mysql` - MySQL database
- `postgres` - PostgreSQL database

??? file "config.yaml (SQLite)"

    ```yaml
    storage:
        driver: sqlite
        data_dir: /path/to/data
    ```

??? file "config.yaml (MySQL)"

    ```yaml
    storage:
        driver: mysql
        dsn: "user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True&loc=Local"
    ```

??? file "config.yaml (PostgreSQL)"

    ```yaml
    storage:
        driver: postgres
        dsn: "host=localhost user=postgres password=postgres dbname=lighthouse port=5432 sslmode=disable TimeZone=UTC"
    ```

## `data_dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-red" title="If this option is required or optional">required for SQLite</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STORAGE_DATA_DIR`</span>

The `data_dir` option sets the directory where the SQLite database file (`lighthouse.db`) will be stored.

This option is only required when using the `sqlite` driver.

??? file "config.yaml"

    ```yaml
    storage:
        driver: sqlite
        data_dir: /var/lib/lighthouse
    ```

## `dsn`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="If this option is required or optional">required for MySQL and PostgreSQL</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STORAGE_DSN`</span>

The `dsn` option specifies the Data Source Name (connection string) for the database.

This option is required when using MySQL or PostgreSQL drivers.

### MySQL DSN Format

```
user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
```

??? file "config.yaml"

    ```yaml
    storage:
        driver: mysql
        dsn: "lighthouse:secret@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True&loc=Local"
    ```

### PostgreSQL DSN Format

```
host=hostname user=username password=password dbname=database port=5432 sslmode=disable TimeZone=UTC
```

??? file "config.yaml"

    ```yaml
    storage:
        driver: postgres
        dsn: "host=localhost user=lighthouse password=secret dbname=lighthouse port=5432 sslmode=disable"
    ```

### DSN Components (Alternative)

Instead of providing a full `dsn` string, you can specify individual connection components:

| Option | Description | Environment Variable |
|--------|-------------|---------------------|
| `user` | Database username | `LH_STORAGE_USER` |
| `password` | Database password | `LH_STORAGE_PASSWORD` |
| `host` | Database host (default: `localhost`) | `LH_STORAGE_HOST` |
| `port` | Database port | `LH_STORAGE_PORT` |
| `db` | Database name (default: `lighthouse`) | `LH_STORAGE_DB` |

!!! tip "Sensitive Data"
    Use `LH_STORAGE_PASSWORD` environment variable to avoid storing database passwords in config files.

??? file "config.yaml"

    ```yaml
    storage:
        driver: postgres
        user: lighthouse
        password: secret
        host: db.example.com
        port: 5432
        db: lighthouse
    ```

## `debug`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">false</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_STORAGE_DEBUG`</span>

The `debug` option enables debug logging for database operations. This is useful for troubleshooting database issues.

??? file "config.yaml"

    ```yaml
    storage:
        driver: sqlite
        data_dir: /var/lib/lighthouse
        debug: true
    ```

## Complete Examples

??? file "SQLite (Recommended for development)"

    ```yaml
    storage:
        driver: sqlite
        data_dir: /var/lib/lighthouse
    ```

??? file "PostgreSQL (Recommended for production)"

    ```yaml
    storage:
        driver: postgres
        dsn: "host=db.example.com user=lighthouse password=secret dbname=lighthouse port=5432 sslmode=require"
    ```

??? file "MySQL"

    ```yaml
    storage:
        driver: mysql
        dsn: "lighthouse:secret@tcp(db.example.com:3306)/lighthouse?charset=utf8mb4&parseTime=True&loc=Local"
    ```

## Legacy Backends (Deprecated)

!!! warning "Deprecated"
    The `backend` option with values `json` and `badger` is **deprecated** and no longer supported.
    
    If you are upgrading from an older version of LightHouse, use the `lhmigrate db` command to 
    migrate your data to the new SQL-based storage. See the [Migration Guide](../migration.md) for details.
