---
icon: material/server-network
---

Under the `server` config option the (http) server can be configured.

## `port`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">7672</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The `port` config option is used to set the port at which LightHouse starts 
the webserver and listens for incoming requests.
Will only be used if `tls` is not used.
If `tls` is enabled port `443` will be used (and optionally port `80`).

??? file "config.yaml"

    ```yaml
    server:
        port: 4242
    ```

## `tls`

Under the `tls` config option settings related to `tls` can be configured.
It is unlikely that one enables `tls` since a reverse proxy will be used in 
most cases.

If `tls` is enabled port `443` will be used.

??? file "config.yaml"

    ```yaml
    server:
        tls:
            enabled: true
            redirect_http: true
            cert: /path/to/cert
            key: /path/to/key
    ```

### `enabled`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

If set to `false` `tls` will be disabled. Otherwise, it will automatically be 
enabled, if `cert` and `key` are set.

### `redirect_http`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`true`</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The `redirect_http` option determines if port `80` should be redirected to 
port `443` or not.

### `cert`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-green" title="If this option is required or optional">required for TLS</span>

The `cert` option is set to the tls `cert` file.

### `key`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-green" title="If this option is required or optional">required for TLS</span>

The `key` option is set to the tls `key` file.
