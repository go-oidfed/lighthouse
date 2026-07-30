---
icon: material/signature-freehand
---

# Database-Managed Signing Options

The following signing options are stored in the database and managed via the
[Admin API](../../features/admin_api.md), [`lhsetup`](../../deployment/lhsetup.md),
or [`lhmigrate config2db`](../../migration/0.22.md). They are **not** set in the
config file.

The static signing options (KMS, public key backend, filesystem/PKCS#11
configuration) remain in the [config file](../static/signing.md).

## Signing Algorithm (`alg`)

<span class="badge badge-purple" title="Value Type">enum</span>
<span class="badge badge-blue" title="Default Value">ES512</span>

The signing algorithm to use for Entity Configurations, Entity Statements, and
Trust Marks.

### Supported Values

**ECDSA**

- `ES256`, `ES384`, `ES512`

**EdDSA (RFC 9864)**

- `Ed25519` (curve-specific, recommended)
- `Ed448`
- `EdDSA` (deprecated polymorphic form; resolves to Ed25519 — prefer `Ed25519`)

**secp256k1**

- `ES256K`

**RSA**

- `RS256`, `RS384`, `RS512` (RSASSA-PKCS1-v1_5)
- `PS256`, `PS384`, `PS512` (RSASSA-PSS)

**ML-DSA (FIPS 204, post-quantum)**

- `ML-DSA-44`, `ML-DSA-65`, `ML-DSA-87`

**Composite (PQC-hybrid) signatures**

- `ML-DSA-44-ES256`, `ML-DSA-65-ES256`, `ML-DSA-87-ES384` (ML-DSA + ECDSA)
- `ML-DSA-44-Ed25519`, `ML-DSA-65-Ed25519`, `ML-DSA-87-Ed448` (ML-DSA + EdDSA)

### Management

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/signing/alg` |
| lhsetup | `lhsetup --only=alg` |
| config2db | `lhmigrate config2db --only=alg` |

## RSA Key Length (`rsa_key_len`)

<span class="badge badge-purple" title="Value Type">integer</span>
<span class="badge badge-blue" title="Default Value">2048</span>

The RSA key length (in bits) used when generating RSA-based signing keys. Only
relevant when the signing algorithm is RSA-based (`RS*` or `PS*`).

### Management

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/signing/rsa-key-len` |
| lhsetup | `lhsetup --only=rsa_key_len` |
| config2db | `lhmigrate config2db --only=rsa_key_len` |

## Key Rotation (`key_rotation`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Configuration for automatic signing key rotation. When enabled, LightHouse
generates new signing keys according to the configured interval and publishes
both current and next public keys.

### `enabled`

<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>

Enables automatic key rotation.

### `interval`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">~1 week</span>

The interval at which keys are rotated. This defines the lifetime of each key.

!!! note
    The interval should not be smaller than the lifetime of Entity
    Configurations, Entity Statements, Trust Marks, or other JWTs signed with
    the federation key.

### `overlap`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">1 hour</span>

The overlap period between the current and next key. During this window,
LightHouse transitions to using the new key while the old key's public key is
still published.

### `key_announcement_lead_time`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">`max(5 × entity configuration lifetime, 24h)`</span>

The **key announcement lead time** controls how far in advance a new key is
published in the JWKS before it becomes the active signing key. This gives
clients time to fetch the updated entity configuration and cache the new key
before the old key is retired.

The effective lead time is resolved as follows:

1. If [`key_announcement_lead_time_ec_multiplier`](#key_announcement_lead_time_ec_multiplier) `> 0`: multiplier × entity configuration lifetime.
2. If `key_announcement_lead_time` `> 0`: the fixed duration.
3. Default: `max(5 × entity configuration lifetime, 24h)`.

The result is clamped to a minimum of the entity configuration lifetime; if the
configured value is shorter, the EC lifetime is used instead and a warning is
logged.

!!! info

      In the past the key announcement lead time was set to one entity
      configuration lifetime. This is only enough if all clients always fetch
      the entity configuration as soon as it expires — an unrealistic
      assumption. Set the lead time to a larger value in production.

### `key_announcement_lead_time_ec_multiplier`

<span class="badge badge-purple" title="Value Type">float</span>
<span class="badge badge-blue" title="Default Value">`0`</span>

Multiplier for the entity configuration lifetime to compute the announcement
lead time. Takes precedence over
[`key_announcement_lead_time`](#key_announcement_lead_time) if set (`> 0`).

### Management

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT/PATCH /api/v1/admin/keys/rotation` |
| lhsetup | `lhsetup --only=key_rotation` |
| config2db | `lhmigrate config2db --only=key_rotation` |
