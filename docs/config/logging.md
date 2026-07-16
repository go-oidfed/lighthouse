---
icon: material/script-text
---

Under the `logging` config option the logging behavior and locations can be 
configured.

## `access`
<span class="badge badge-purple" title="Value Type">object</span>
<span class="badge badge-green" title="If this option is required or optional">recommended</span>

Under the `access` option the http access log can be configured.

??? file "config.yaml"

    ```yaml
    logging:
        access:
            dir: /var/log/lighthouse
            stderr: true
    ```

The following options are available:

### `dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_ACCESS_DIR`</span>

The `dir` option is used to configure the directory where the log file 
should be stored.
If not set, LightHouse will not log to file.

### `stderr`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_ACCESS_STDERR`</span>

The `stderr` option indicates if LightHouse logs to `stderr`.

## `internal`
The `internal` option is used to configure logging for LightHouse's internal 
logging, i.e. logging related to what LightHouse does.

??? file "config.yaml"

    ```yaml
    logging:
        internal:
            dir: /var/log/lighthouse
            stderr: true
            level: info
            stderr_format: console
            dir_format: json
    ```

### `dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_INTERNAL_DIR`</span>

The `dir` option is used to configure the directory where the log file 
should be stored.
If not set, LightHouse will not log to file.

### `stderr`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_INTERNAL_STDERR`</span>

The `stderr` option indicates if LightHouse logs to `stderr`.

### `level`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">info</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_INTERNAL_LEVEL`</span>

The `level` option sets the minimal log level that should be logged.

!!! tip "Shortcut"
    You can also use `LH_LOG_LEVEL` as a shortcut for `LH_LOGGING_INTERNAL_LEVEL`.

Valid values are:

- `trace`
- `debug`
- `info`
- `warn` 
- `error`
- `fatal`
- `panic`

### `stderr_format`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">console</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_INTERNAL_STDERR_FORMAT`</span>

The `stderr_format` option selects the output format for logs written to
stderr.

Valid values are:

- `console` — human-friendly, colored console output (default)
- `json` — structured JSON output for machine parsing

### `dir_format`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">json</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_INTERNAL_DIR_FORMAT`</span>

The `dir_format` option selects the output format for logs written to file.

Valid values are:

- `json` — structured JSON output for machine parsing (default)
- `console` — human-friendly console output (without ANSI color codes)

## `banner`
<span class="badge badge-purple" title="Value Type">object</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Under the `banner` option, startup banners can be enabled or disabled.
The logo banner is an ANSI art; the version banner is rendered as an
ASCII-art representation of the current Lighthouse version.

??? file "config.yaml"

    ```yaml
    logging:
        banner:
            logo: false
            version: false
    ```

The following options are available:

### `logo`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_BANNER_LOGO`</span>

The `logo` option controls printing of the Lighthouse logo banner on startup.

### `version`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_LOGGING_BANNER_VERSION`</span>

The `version` option controls printing of the version banner on startup.