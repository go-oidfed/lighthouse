---
icon: material/shield-check-outline
---

# Trust Marks

These database-managed options control trust mark publication, issuance specs,
and trust mark ownership. They are managed via the
[Admin API](../../features/admin_api.md), [`lhsetup`](../../deployment/lhsetup.md),
or [`lhmigrate config2db`](../../migration/0.22.md).

See also the [Trust Marks feature documentation](../../features/trustmarks.md)
for how trust mark issuance works in practice.

## Published Trust Marks (`trust_marks`)

<span class="badge badge-purple" title="Value Type">list</span>

Trust marks about this LightHouse entity that are published in the Entity
Configuration. These are trust marks *held by* this entity, not trust marks it
issues.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST/DELETE /api/v1/admin/entity-configuration/trust-marks` |
| lhsetup | `lhsetup --only=trust_marks` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_marks` |

### Fields per Trust Mark

#### `trust_mark_type`

<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="Required">required</span>

The identifier (URI) for the type of this trust mark.

#### `trust_mark_issuer`

<span class="badge badge-purple" title="Value Type">uri</span>
<span class="badge badge-red" title="Required">required if `trust_mark_jwt` not given</span>

The entity ID of the trust mark issuer. If given (instead of `trust_mark_jwt`),
[`refresh`](#refresh) is set to `true` and LightHouse will obtain trust mark
JWTs dynamically from the issuer.

#### `trust_mark_jwt`

<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="Required">required if `trust_mark_issuer` not given</span>

A trust mark JWT string to publish in the Entity Configuration. If it expires,
it must be manually updated or automatic refreshing enabled via
[`refresh`](#refresh).

#### `refresh`

<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>

When `true`, LightHouse automatically fetches a new trust mark JWT from the
trust mark issuer when the old one expires.

#### `min_lifetime`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">10 seconds</span>

When [`refresh`](#refresh) is `true`, LightHouse ensures the published trust
mark JWT will not expire before this lifetime whenever an Entity Configuration
is requested.

#### `refresh_grace_period`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">1 hour</span>

When [`refresh`](#refresh) is `true`, LightHouse checks if the trust mark
expires within this grace period on every Entity Configuration request. If it
does, the old (still valid) JWT is kept while a refresh is requested in the
background.

??? example "Example published trust mark"

    ```json
    {
      "trust_mark_type": "https://example.com/tm",
      "trust_mark_issuer": "https://example.com/tmi",
      "refresh": true,
      "min_lifetime": 300,
      "refresh_grace_period": 7200
    }
    ```

## Trust Mark Specs (`trust_mark_specs`)

<span class="badge badge-purple" title="Value Type">list</span>

Defines which trust marks LightHouse can issue. Each spec describes a trust
mark type and its properties.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST/PUT/DELETE /api/v1/admin/trust-mark-types/{type}/specs` |
| lhsetup | `lhsetup --only=trust_mark_specs` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_mark_specs` |

### Fields per Trust Mark Spec

#### `trust_mark_type`

<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="Required">required</span>

The trust mark type (ID) of the trust mark.

#### `lifetime`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>

The lifetime of each issued trust mark JWT.

#### `ref`

<span class="badge badge-purple" title="Value Type">uri</span>

The `ref` URI inside the trust mark JWT, as defined in the OpenID Federation
Specification.

#### `logo_uri`

<span class="badge badge-purple" title="Value Type">uri</span>

The logo URI inside the trust mark JWT.

#### `delegation_jwt`

<span class="badge badge-purple" title="Value Type">string</span>

The delegation JWT inside the trust mark JWT. Required if this LightHouse
instance is not the trust mark owner but issues trust marks on behalf of the
owner.

#### Extra Claims

Any additional claim not listed above is added to the trust mark JWT.

#### `checker`

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Optional [entity check](../../features/entity_checks.md) configuration used to
dynamically issue trust marks to entities.

??? example "Example trust mark spec"

    ```json
    {
      "trust_mark_type": "https://tm.example.org",
      "lifetime": "1d",
      "ref": "https://tm.example.org/ref",
      "logo_uri": "https://tm.example.org/logo"
    }
    ```

## Trust Mark Issuers (`trust_mark_issuers`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Allowed trust mark issuers per trust mark type, as defined in the OpenID
Federation Specification.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST/DELETE /api/v1/admin/trust-mark-types/{type}/issuers` |
| lhsetup | `lhsetup --only=trust_mark_issuers` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_mark_issuers` |

??? example "Example"

    ```json
    {
      "https://openid.net/certification/op": [],
      "https://refeds.org/sirtfi": ["https://example.org"]
    }
    ```

## Trust Mark Owners (`trust_mark_owners`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Trust mark owners recognized within this federation, as defined in the OpenID
Federation Specification.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT/DELETE /api/v1/admin/trust-mark-types/{type}/owner` |
| lhsetup | `lhsetup --only=trust_mark_owners` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_mark_owners` |

??? example "Example"

    ```json
    {
      "https://refeds.org/sirtfi": {
        "sub": "https://refeds.org",
        "jwks": {"keys": [{"kty": "RSA", "kid": "key1", "...": "..."}]}
      }
    }
    ```
