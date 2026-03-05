---
icon: material/database
---
<span class="badge badge-red" title="If this option is required or optional">required</span>

The `storage` option is used to configure how and where data is stored.

## `backend`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">badger</span>
<span class="badge badge-orange" title="If this option is required or optional">recommended</span>

The `backend` option is used to set which storage backend should be used to store data. This defines how data is 
stored. Depending on the chosen backend different further configuration options might be available or not.

In the following the supported storage backend and their available options are detailed.

### `json`
If `backend` is set to `json` the JSON files backend is used. This backend stores data in simple json files (in 
multiple directories). This option is great to see which data is stored, since it is the most human-readable storage 
format supported. It is also great if data is manipulated externally.
Performance-wise other options are better.

??? file "config.yaml"

    ```yaml
    storage:
        backend: json
        data_dir: /path/to/data
    ```

The following configuration options are defined for the `json` backend:

#### `data_dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-red" title="If this option is required or optional">required</span>

The `data_dir` option sets the root directory where data is stored on disk. LightHouse creates subdirectories and 
places the JSON files in those directories.

### `badger`
If `backend` is set to `badger` the [BadgerDB](https://github.com/hypermodeinc/badger) backend is used. BadgerDB is 
an embeddable, persistent key-value database. No external dependencies are needed, as BadgerDB is embedded into 
LightHouse. The data is stored on disk is not suitable to be read or manipulated by humans.

??? file "config.yaml"

    ```yaml
    storage:
        backend: badger
        data_dir: /path/to/data
    ```

The following configuration options are defined for the `badger` backend:

#### `data_dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-red" title="If this option is required or optional">required</span>

The `data_dir` option sets the root directory where the badger data is stored on disk. 

### `gorm`
If `backend` is set to `gorm` the [GORM](https://gorm.io/) ORM mapper is used. GORM is a feature-rich ORM for Go, and this backend supports multiple database engines including SQLite, MySQL, and PostgreSQL. The data is stored in a database which can be accessed using standard database tools.

??? file "config.yaml (SQLite)"

    ```yaml
    storage:
        backend: gorm
        data_dir: /path/to/data
        database:
            driver: sqlite
            debug: false
    ```

??? file "config.yaml (MySQL)"

    ```yaml
    storage:
        backend: gorm
        database:
            driver: mysql
            dsn: "user:pass@tcp(127.0.0.1:3306)/lighthouse?charset=utf8mb4&parseTime=True&loc=Local"
            debug: false
    ```

??? file "config.yaml (PostgreSQL)"

    ```yaml
    storage:
        backend: gorm
        database:
            driver: postgres
            dsn: "host=localhost user=postgres password=postgres dbname=lighthouse port=5432 sslmode=disable TimeZone=UTC"
            debug: false
    ```

The following configuration options are defined for the `gorm` backend:

#### `data_dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-red" title="If this option is required or optional">required for SQLite</span>

The `data_dir` option sets the directory where the SQLite database file (`lighthouse.db`) will be stored. This option is only required when using the SQLite driver.

#### `database.driver`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-orange" title="If this option is required or optional">optional</span>

The `database.driver` option specifies which database driver to use. Supported values are:
- `sqlite` (default): Uses SQLite as the database engine
- `mysql`: Uses MySQL as the database engine
- `postgres`: Uses PostgreSQL as the database engine

#### `database.dsn`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="If this option is required or optional">required for MySQL and PostgreSQL</span>

The `database.dsn` option specifies the Data Source Name (connection string) for the database. This option is required when using MySQL or PostgreSQL drivers.

For MySQL, the DSN format is: `user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local`

For PostgreSQL, the DSN format is: `host=localhost user=postgres password=postgres dbname=lighthouse port=5432 sslmode=disable TimeZone=UTC`

#### `database.debug`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">false</span>
<span class="badge badge-orange" title="If this option is required or optional">optional</span>

The `database.debug` option enables debug logging for the database operations. This is useful for troubleshooting database issues.
