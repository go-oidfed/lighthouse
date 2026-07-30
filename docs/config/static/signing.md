---
icon: material/signature-freehand
---
<span class="badge badge-red" title="If this option is required or optional">required</span>

Under the `signing` config option the key management and signatures are configured.

Some signing options (signing algorithm, RSA key length, key rotation, and key
announcement lead time) are stored in the database and managed via the Admin
API, `lhsetup`, or `lhmigrate config2db`. See
[Database-Managed Signing Options](../db/signing.md) for details.

## `kms`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-red" title="If this option is required or optional">required</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_KMS`</span>

The `kms` option specifies which Key Management System to use for private key storage.

Supported values:

- `filesystem` - Keys stored on the filesystem
- `pkcs11` - Keys stored in a Hardware Security Module (HSM) via PKCS#11
- `db` - Keys stored in the database

??? file "Filesystem KMS (default)"

    ```yaml
    signing:
        kms: filesystem
        filesystem:
            key_dir: /path/to/keys
    ```

??? file "Database-backed KMS"

    ```yaml
    signing:
        kms: db
        pk_backend: db
        auto_generate_keys: true
    ```

!!! info "Migration from Filesystem KMS"
    To migrate existing filesystem-based private keys to the database, use:
    ```bash
    lhmigrate pem-to-db --source /path/to/kms/dir --type federation --db-dsn /path/to/db.sqlite
    ```

## `pk_backend`
<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">db</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PK_BACKEND`</span>

The `pk_backend` option specifies where public keys are stored.

Supported values:

- `db` - Public keys stored in the database (default, recommended)
- `filesystem` - Public keys stored on the filesystem

??? file "config.yaml"

    ```yaml
    signing:
        kms: filesystem
        pk_backend: db
        filesystem:
            key_dir: /path/to/keys
    ```

## `auto_generate_keys`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">true</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_AUTO_GENERATE_KEYS`</span>

When set to `true`, LightHouse will automatically generate signing keys at startup if they don't exist.
When set to `false`, LightHouse will exit with an error if the required private key is not present.

??? file "config.yaml"

    ```yaml
    signing:
        kms: filesystem
        auto_generate_keys: false
        filesystem:
            key_dir: /path/to/keys
    ```

## `filesystem`
<span class="badge badge-purple" title="Value Type">object / mapping</span>
<span class="badge badge-orange" title="If this option is required or optional">required when kms=filesystem or pk_backend=filesystem</span>

Configuration for the filesystem Key Management System.

??? file "config.yaml"

    ```yaml
    signing:
        kms: filesystem
        filesystem:
            key_dir: /var/lib/lighthouse/keys
    ```

### `key_dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-orange" title="If this option is required or optional">required for kms=filesystem</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_FILESYSTEM_KEY_DIR`</span>

The `key_dir` option specifies the path to a directory that contains the private signing key(s).
Keys are stored using the naming convention `<kid>.pem`.

??? file "config.yaml"

    ```yaml
    signing:
        kms: filesystem
        filesystem:
            key_dir: /var/lib/lighthouse/keys
    ```

### `key_file`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_FILESYSTEM_KEY_FILE`</span>

The `key_file` option can be used as an alternative to `key_dir` if only a 
single signing key is used and no key rotation happens. We recommend to use 
`key_dir` instead.

## `pkcs11`
<span class="badge badge-purple" title="Value Type">object / mapping</span>
<span class="badge badge-orange" title="If this option is required or optional">required when kms=pkcs11</span>

Configuration for PKCS#11 Hardware Security Module (HSM) integration.

??? file "config.yaml"

    ```yaml
    signing:
        kms: pkcs11
        pkcs11:
            module_path: /usr/lib/softhsm/libsofthsm2.so
            token_label: lighthouse
            pin: "1234"
            storage_dir: /var/lib/lighthouse/pkcs11
    ```

### `module_path`
<span class="badge badge-purple" title="Value Type">file path</span>
<span class="badge badge-red" title="If this option is required or optional">required</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_MODULE_PATH`</span>

Path to the PKCS#11 module (shared library) provided by your HSM vendor.

Common paths:

- SoftHSM2: `/usr/lib/softhsm/libsofthsm2.so`
- AWS CloudHSM: `/opt/cloudhsm/lib/libcloudhsm_pkcs11.so`
- YubiHSM: `/usr/lib/x86_64-linux-gnu/pkcs11/yubihsm_pkcs11.so`

### `token_label`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-green" title="If this option is required or optional">required (one of token_label, token_serial, or token_slot)</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_TOKEN_LABEL`</span>

Selects the HSM token by its label.

### `token_serial`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-green" title="If this option is required or optional">required (one of token_label, token_serial, or token_slot)</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_TOKEN_SERIAL`</span>

Selects the HSM token by its serial number.

### `token_slot`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-green" title="If this option is required or optional">required (one of token_label, token_serial, or token_slot)</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_TOKEN_SLOT`</span>

Selects the HSM token by its slot number.

### `pin`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="If this option is required or optional">required (unless no_login is true)</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_PIN`</span>

The user PIN for authenticating to the HSM token.

### `storage_dir`
<span class="badge badge-purple" title="Value Type">directory path</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_STORAGE_DIR`</span>

Directory for storing PKCS#11-related metadata.

### `max_sessions`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_MAX_SESSIONS`</span>

Maximum number of concurrent PKCS#11 sessions to open. Must be at least 2 if specified.

### `user_type`
<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_USER_TYPE`</span>

User type for PKCS#11 login. Usually left at default.

### `no_login`
<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">false</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_NO_LOGIN`</span>

Set to `true` for HSM tokens that do not support or require login.

### `label_prefix`
<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_LABEL_PREFIX`</span>

Optional prefix for object labels inside the HSM.

### `load_labels`
<span class="badge badge-purple" title="Value Type">list of strings</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>
<span class="badge badge-cyan" title="Environment Variable">`LH_SIGNING_PKCS11_LOAD_LABELS`</span>

HSM object labels to load into this KMS even if they are not yet present in the public key storage.

For environment variables, use comma-separated values: `LH_SIGNING_PKCS11_LOAD_LABELS="key1,key2"`

## Complete Examples

??? file "Filesystem KMS with database public keys (Recommended)"

    ```yaml
    signing:
        kms: filesystem
        pk_backend: db
        auto_generate_keys: true
        filesystem:
            key_dir: /var/lib/lighthouse/keys
    ```

??? file "PKCS#11 HSM (SoftHSM2)"

    ```yaml
    signing:
        kms: pkcs11
        pk_backend: db
        auto_generate_keys: true
        pkcs11:
            module_path: /usr/lib/softhsm/libsofthsm2.so
            token_label: lighthouse
            pin: "1234"
            storage_dir: /var/lib/lighthouse/pkcs11
    ```
