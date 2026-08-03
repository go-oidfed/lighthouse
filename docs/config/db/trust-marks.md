---
icon: material/shield-check-outline
---

# Trust Marks

These database-managed options control trust mark publication, issuance specs,
trust mark types, and trust mark ownership/issuance delegation. They are
managed via the [Admin API](../../features/admin_api.md),
[`lhsetup`](../../deployment/lhsetup.md), or
[`lhmigrate config2db`](../../migration/0.22.md).

See also the [Trust Marks feature documentation](../../features/trustmarks.md)
for how trust mark issuance works in practice.

## Published Trust Marks (`trust_marks`)

<span class="badge badge-purple" title="Value Type">list</span>

Trust marks about this LightHouse entity that are published in the Entity
Configuration. These are trust marks *held by* this entity, not trust marks it
issues.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST /api/v1/admin/entity-configuration/trust-marks` |
| Admin API | `GET/PUT/PATCH/DELETE /api/v1/admin/entity-configuration/trust-marks/{trustMarkID}` |
| lhsetup | `lhsetup --only=trust_marks` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_marks` |

### Fields per Trust Mark

#### `trust_mark_type`

<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="Required">required</span>

The identifier (URI) for the type of this trust mark.

#### `trust_mark_issuer`

<span class="badge badge-purple" title="Value Type">uri</span>
<span class="badge badge-red" title="Required">required if `trust_mark` not given</span>

The entity ID of the trust mark issuer. If given (instead of `trust_mark`),
[`refresh`](#refresh) is set to `true` and LightHouse will obtain trust mark
JWTs dynamically from the issuer.

#### `trust_mark`

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

#### `refresh_rate_limit`

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>

When [`refresh`](#refresh) is `true`, this limits how frequently a refresh is
attempted, preventing excessive requests to the trust mark issuer.

#### `self_issuance_spec`

<span class="badge badge-purple" title="Value Type">object / mapping</span>

When set, LightHouse self-issues the trust mark JWT instead of fetching it from
an external issuer. The spec contains:

| Field                          | Type               | Description                                                    |
|--------------------------------|--------------------|----------------------------------------------------------------|
| `lifetime`                     | integer (seconds)  | Lifetime of each self-issued trust mark JWT                    |
| `ref`                          | uri                | The `ref` URI inside the trust mark JWT                        |
| `logo_uri`                     | uri                | The logo URI inside the trust mark JWT                         |
| `additional_claims`            | object / mapping   | Additional claims included in the self-issued trust mark JWT   |
| `include_extra_claims_in_info` | boolean            | Whether to include extra claims in the trust mark info         |

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

??? example "Example self-issued trust mark"

    ```json
    {
      "trust_mark_type": "https://example.com/self-issued",
      "self_issuance_spec": {
        "lifetime": 86400,
        "ref": "https://example.com/ref",
        "logo_uri": "https://example.com/logo"
      }
    }
    ```

## Trust Mark Specs (`trust_mark_specs`)

<span class="badge badge-purple" title="Value Type">list</span>

Defines which trust marks LightHouse can issue. Each spec describes a trust
mark type and its issuance properties, including lifetime, eligibility rules,
and per-subject overrides.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST /api/v1/admin/trust-marks/issuance-spec` |
| Admin API | `GET/PUT/PATCH/DELETE /api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}` |
| lhsetup | `lhsetup --only=trust_mark_specs` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_mark_specs` |

### Fields per Trust Mark Spec

#### `trust_mark_type`

<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-red" title="Required">required</span>

The trust mark type (ID) of the trust mark.

#### `lifetime`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>

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

#### `additional_claims`

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Additional claims included in every trust mark JWT issued under this spec.

#### `description`

<span class="badge badge-purple" title="Value Type">string</span>

Human-readable description of the trust mark spec.

#### `eligibility_config`

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Configuration that determines how trust mark eligibility is evaluated. See
[Eligibility Modes](../../features/trustmarks.md#eligibility-modes) in the
feature documentation for details.

| Field             | Type    | Description                                                                          |
|-------------------|---------|--------------------------------------------------------------------------------------|
| `mode`            | string  | Eligibility mode: `db_only`, `check_only`, `db_or_check`, `db_and_check`, `custom`   |
| `checker`         | object  | Entity checker configuration (type + config). See [Entity Checks](../../features/entity_checks.md) |
| `check_cache_ttl` | integer | How long to cache eligibility check results (seconds); `0` = no cache                |

#### `cache_ttl`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>

How long to cache issued trust mark JWTs for this type. Reduces signing
operations and database writes for repeated requests. `0` = no caching
(default).

??? example "Example trust mark spec"

    ```json
    {
      "trust_mark_type": "https://tm.example.org",
      "lifetime": 86400,
      "ref": "https://tm.example.org/ref",
      "logo_uri": "https://tm.example.org/logo",
      "eligibility_config": {
        "mode": "db_only"
      }
    }
    ```

### Trust Mark Subjects

Each spec maintains a list of subjects (entities) that are eligible to receive
the trust mark. Each subject has a status (`active`, `blocked`, `pending`,
`inactive`) and optional per-subject additional claims.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST /api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}/subjects` |
| Admin API | `GET/PUT/DELETE /api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}/subjects/{trustMarkSubjectID}` |
| Admin API | `PUT /api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}/subjects/{trustMarkSubjectID}/status` |
| Admin API | `GET/PUT/POST /api/v1/admin/trust-marks/issuance-spec/{trustMarkSpecID}/subjects/{trustMarkSubjectID}/additional-claims` |

#### Fields per Subject

| Field               | Type              | Description                                          |
|---------------------|-------------------|------------------------------------------------------|
| `entity_id`         | string            | Entity ID of the subject                             |
| `status`            | string            | `active`, `blocked`, `pending`, or `inactive`        |
| `description`       | string            | Human-readable description                           |
| `additional_claims` | object / mapping  | Per-subject additional claims merged into the JWT    |

The `POST` on `additional-claims` copies the spec-level additional claims into
the subject, merging with any existing subject-level claims.

## Federation Trust Marks

This section covers trust mark **types**, **issuers**, and **owners** — the
federation-level configuration that defines which trust mark types exist in
this federation, who owns them, and who is authorized to issue them, as
defined in the OpenID Federation Specification.

Each can be managed through two complementary API patterns:

- **Type-centric** (`/trust-marks/types/{typeID}/...`): Start from a trust
  mark type and manage its owner and issuers.
- **Entity-centric** (`/trust-marks/owners`, `/trust-marks/issuers`): Start
  from an owner or issuer entity and manage which trust mark types it is
  associated with.

### Trust Mark Types

<span class="badge badge-purple" title="Value Type">list</span>

Trust mark types are the registry of trust mark identifiers in this
federation. Each type can have an optional owner and zero or more authorized
issuers. Types
are created implicitly by `lhsetup` and `config2db` when adding owners or
issuers, but can also be managed directly via the Admin API.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST /api/v1/admin/trust-marks/types` |
| Admin API | `GET/PUT/DELETE /api/v1/admin/trust-marks/types/{trustMarkTypeID}` |
| lhsetup | Not applicable (created implicitly when adding owners/issuers) |
| config2db | Not applicable (created implicitly when migrating owners/issuers) |

#### Fields per Trust Mark Type

| Field             | Type    | Description                                          |
|-------------------|---------|------------------------------------------------------|
| `id`              | integer | Read-only identifier (assigned by the API)           |
| `trust_mark_type` | string  | The trust mark type identifier (URI)                 |
| `description`     | string  | Human-readable description                           |
| `owner`           | object  | The [owner](#trust-mark-owners-trust_mark_owners) of this type, if set |
| `issuers`         | list    | The [issuers](#trust-mark-issuers-trust_mark_issuers) authorized for this type |

When creating a type, the owner and issuers can be set inline for convenience
(see `trust_mark_owner` and `trust_mark_issuers` in the request body).

??? example "Create a type with owner and issuers"

    ```json
    {
      "trust_mark_type": "https://refeds.org/sirtfi",
      "description": "Sirtfi compliance",
      "trust_mark_owner": {
        "entity_id": "https://refeds.org",
        "jwks": {"keys": [...]}
      },
      "trust_mark_issuers": [
        {"issuer": "https://issuer.example.org"}
      ]
    }
    ```

### Trust Mark Issuers (`trust_mark_issuers`)

<span class="badge badge-purple" title="Value Type">list</span>

Authorized trust mark issuers per trust mark type, as defined in the OpenID
Federation Specification.

#### Type-centric API

Manage the issuers authorized for a specific trust mark type:

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT/POST /api/v1/admin/trust-marks/types/{trustMarkTypeID}/issuers` |
| Admin API | `DELETE /api/v1/admin/trust-marks/types/{trustMarkTypeID}/issuers/{issuerID}` |
| lhsetup | `lhsetup --only=trust_mark_issuers` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_mark_issuers` |

#### Entity-centric API

Manage issuer entities and their associated trust mark types:

| Tool | Command |
|------|---------|
| Admin API | `GET/POST /api/v1/admin/trust-marks/issuers` |
| Admin API | `GET/PUT/DELETE /api/v1/admin/trust-marks/issuers/{issuerID}` |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/trust-marks/issuers/{issuerID}/types` |

#### Fields per Issuer

| Field         | Type    | Description                                    |
|---------------|---------|------------------------------------------------|
| `id`          | integer | Read-only identifier (assigned by the API)     |
| `issuer`      | string  | Entity ID of the issuer                        |
| `description` | string  | Human-readable description                     |
| `types`       | list    | Trust mark types this issuer is authorized for (entity-centric API only) |

??? example "Add an issuer to a type (type-centric)"

    ```bash
    curl -X POST -u admin:password \
      -H "Content-Type: application/json" \
      -d '{"issuer": "https://issuer.example.org"}' \
      https://localhost:8081/api/v1/admin/trust-marks/types/1/issuers
    ```

### Trust Mark Owners (`trust_mark_owners`)

<span class="badge badge-purple" title="Value Type">list</span>

Trust mark owners recognized within this federation, as defined in the OpenID
Federation Specification. Each trust mark type has at most one owner, whose
JWKS is used to verify delegation when another entity issues trust marks on
the owner's behalf.

#### Type-centric API

Manage the single owner of a specific trust mark type:

| Tool | Command |
|------|---------|
| Admin API | `GET/POST/PUT/DELETE /api/v1/admin/trust-marks/types/{trustMarkTypeID}/owner` |
| lhsetup | `lhsetup --only=trust_mark_owners` (add/remove) |
| config2db | `lhmigrate config2db --only=trust_mark_owners` |

#### Entity-centric API

Manage owner entities and their associated trust mark types:

| Tool | Command |
|------|---------|
| Admin API | `GET/POST /api/v1/admin/trust-marks/owners` |
| Admin API | `GET/PUT/DELETE /api/v1/admin/trust-marks/owners/{ownerID}` |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/trust-marks/owners/{ownerID}/types` |

#### Fields per Owner

| Field         | Type    | Description                                    |
|---------------|---------|------------------------------------------------|
| `id`          | integer | Read-only identifier (assigned by the API)     |
| `entity_id`   | string  | Entity ID of the owner                         |
| `jwks`        | object  | JWKS used to verify delegation JWTs            |
| `description` | string  | Human-readable description                     |

??? example "Set the owner of a type (type-centric)"

    ```bash
    curl -X POST -u admin:password \
      -H "Content-Type: application/json" \
      -d '{
        "entity_id": "https://refeds.org",
        "jwks": {"keys": [{"kty": "RSA", "kid": "key1", "...": "..."}]}
      }' \
      https://localhost:8081/api/v1/admin/trust-marks/types/1/owner
    ```
