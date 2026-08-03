---
icon: material/account-group
---

# Subordinates

These database-managed options control the subordinate statements (Entity
Statements) that LightHouse issues via its Fetch endpoint. They are split into
**general** settings that apply to all subordinate statements and
**per-subordinate** settings.

Subordinates themselves (entity ID, JWKS, status, etc.) are managed via the
[Admin API](../../features/admin_api.md).

## General Subordinate Statement Configuration

These options apply to every subordinate statement issued by the Fetch
endpoint. Per-subordinate overrides (where available) take precedence — see
[Per-Subordinate Configuration](#per-subordinate-configuration) below.

### Statement Lifetime (`statement_lifetime`)

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">~600 000 seconds (≈ 1 week)</span>

The lifetime of Entity Statements (subordinate statements) issued by the Fetch
Endpoint. This defines how long a subordinate statement remains valid.

| Tool      | Command                                           |
|-----------|---------------------------------------------------|
| Admin API | `GET/PUT /api/v1/admin/subordinates/lifetime`     |
| lhsetup   | `lhsetup --only=statement_lifetime`               |
| config2db | `lhmigrate config2db --only=statement_lifetime`   |

### Constraints (`constraints`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Constraints object included in subordinate statements, as defined in the
OpenID Federation Specification. Managed as a JSON object via the Admin API.

| Tool      | Command                                          |
|-----------|--------------------------------------------------|
| Admin API | `GET/PUT /api/v1/admin/subordinates/constraints` |
| lhsetup   | `lhsetup --only=constraints` (via JSON file)     |
| config2db | `lhmigrate config2db --only=constraints`         |

??? example "Example constraints object"

    ```json
    {
      "max_path_len": 2,
      "naming_constraints": {
        "permitted": [".example.com"],
        "excluded": ["east.example.com"]
      },
      "allowed_entity_types": ["openid_provider", "openid_relying_party"]
    }
    ```

### Metadata Policies (`metadata_policies`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

General metadata policies applicable to all subordinates, as defined in the
OpenID Federation Specification. Managed as a JSON object via the Admin API.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/subordinates/metadata-policies` |
| lhsetup | `lhsetup --only=metadata_policies` (via JSON file) |
| config2db | `lhmigrate config2db --only=metadata_policies` |

### Metadata Policy Crit (`metadata_policy_crit`)

<span class="badge badge-purple" title="Value Type">list of strings</span>

Critical metadata policy operators as per the OpenID Federation Specification.

| Tool      | Command                                                   |
|-----------|-----------------------------------------------------------|
| Admin API | `GET/PUT /api/v1/admin/subordinates/metadata-policy-crit` |
| lhsetup   | `lhsetup --only=metadata_policy_crit`                     |
| config2db | `lhmigrate config2db --only=metadata_policy_crit`         |

### Additional Claims (`additional_claims`)

<span class="badge badge-purple" title="Value Type">list of objects</span>

Additional claims included in every subordinate statement. Each claim can be
individually marked as critical.

Each claim object has the following fields:

| Field   | Type    | Description                                  |
|---------|---------|----------------------------------------------|
| `id`    | integer | Read-only identifier (assigned by the API)   |
| `claim` | string  | Claim name                                   |
| `value` | any     | Claim value (any JSON type)                  |
| `crit`  | boolean | Whether the claim is marked as critical      |

| Tool      | Command                                                                  |
|-----------|--------------------------------------------------------------------------|
| Admin API | `GET/PUT/POST /api/v1/admin/subordinates/additional-claims`              |
| Admin API | `GET/PUT/DELETE /api/v1/admin/subordinates/additional-claims/{id}`       |
| lhsetup   | `lhsetup --only=subordinate_additional_claims` (add/remove)              |

??? example "Example additional claim"

    ```json
    {
      "claim": "custom_scope",
      "value": "read write",
      "crit": false
    }
    ```

## Per-Subordinate Configuration

Each subordinate is stored as a record with basic fields, JWKS, JWKS refresh
settings, and optional per-subordinate statement overrides. When a statement
override is not set, the corresponding
[general subordinate statement configuration](#general-subordinate-statement-configuration)
is used as a fallback.

See [Subordinate JWKS Refreshing](../../features/subordinate_jwks_refresh.md)
for the full JWKS refresh feature documentation.

### Subordinate Record

The basic fields on each subordinate, set when creating or updating via the
Admin API:

| Field                     | Type    | Description                                                                |
|---------------------------|---------|----------------------------------------------------------------------------|
| `entity_id`               | string  | Entity ID of the subordinate                                               |
| `status`                  | string  | `active`, `blocked`, `pending`, or `inactive`                              |
| `description`             | string  | Human-readable description                                                 |
| `registered_entity_types` | list    | Entity types this subordinate is registered as (e.g. `openid_provider`)    |
| `enable_jwks_update`      | boolean | Enable periodic JWKS polling (see [JWKS Refresh](#enable_jwks_update))     |
| `jwks_poll_interval`      | integer | Poll interval in seconds (see [JWKS Refresh](#jwks_poll_interval))         |

| Tool      | Command                                                                          |
|-----------|----------------------------------------------------------------------------------|
| Admin API | `GET/POST /api/v1/admin/subordinates`                                            |
| Admin API | `GET/PUT/PATCH/DELETE /api/v1/admin/subordinates/{subordinateID}`                |
| Admin API | `PUT /api/v1/admin/subordinates/{subordinateID}/status`                          |
| lhsetup   | Not applicable (per-subordinate, set via Admin API or enroll endpoint)           |

??? example "Create a subordinate"

    ```bash
    curl -X POST -u admin:password \
      -H "Content-Type: application/json" \
      -d '{
        "entity_id": "https://sub.example.com",
        "status": "active",
        "jwks": {"keys": [...]},
        "enable_jwks_update": true,
        "jwks_poll_interval": 3600
      }' \
      https://localhost:8081/api/v1/admin/subordinates
    ```

??? example "Update a subordinate (PATCH for partial updates)"

    ```bash
    curl -X PATCH -u admin:password \
      -H "Content-Type: application/json" \
      -d '{"enable_jwks_update": true, "jwks_poll_interval": 7200}' \
      https://localhost:8081/api/v1/admin/subordinates/{subordinateID}
    ```

### JWKS

The subordinate's JWKS, used to verify signatures on its Entity Configuration
and signed JWK Sets. Managed via dedicated JWKS endpoints:

| Tool      | Command                                                                  |
|-----------|--------------------------------------------------------------------------|
| Admin API | `GET/PUT/POST /api/v1/admin/subordinates/{subordinateID}/jwks`          |
| Admin API | `DELETE /api/v1/admin/subordinates/{subordinateID}/jwks/{kid}`          |

### JWKS Refresh

#### `enable_jwks_update`

<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>

When `true`, LightHouse periodically polls the subordinate's Entity
Configuration and updates the stored JWKS when keys change. The fetched
Entity Configuration's signature is verified against the currently stored
JWKS; if verification fails, the JWKS is **not** updated.

#### `jwks_poll_interval`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>
<span class="badge badge-blue" title="Default Value">`0` (derive from EC expiration)</span>

Poll interval in seconds. When `0` (the default) or `<= 0`, LightHouse derives
the interval from the subordinate's Entity Configuration expiration time
(with a small buffer), floored to a minimum of **1 minute**.

### Statement Overrides

The following options override the
[general subordinate statement configuration](#general-subordinate-statement-configuration)
for a specific subordinate. When not set, the general value is used as a
fallback.

#### Metadata (`metadata`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Per-subordinate metadata included in the subordinate statement. When not set,
no metadata is included (there is no general subordinate metadata fallback).

| Tool      | Command                                                                                  |
|-----------|------------------------------------------------------------------------------------------|
| Admin API | `GET/PUT /api/v1/admin/subordinates/{subordinateID}/metadata`                            |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/subordinates/{subordinateID}/metadata/{entityType}`   |
| Admin API | `GET/PUT/DELETE /api/v1/admin/subordinates/{subordinateID}/metadata/{entityType}/{claim}`|

#### Constraints (`constraints`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Per-subordinate constraints, overriding the
[general constraints](#constraints-constraints). When not set, the general
constraints are used.

| Tool      | Command                                                                                              |
|-----------|------------------------------------------------------------------------------------------------------|
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/subordinates/{subordinateID}/constraints`                        |
| Admin API | `GET/PUT/DELETE /api/v1/admin/subordinates/{subordinateID}/constraints/max-path-length`             |
| Admin API | `GET/PUT/DELETE /api/v1/admin/subordinates/{subordinateID}/constraints/naming-constraints`          |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/subordinates/{subordinateID}/constraints/allowed-entity-types`   |

#### Metadata Policies (`metadata_policies`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Per-subordinate metadata policies, overriding the
[general metadata policies](#metadata-policies-metadata_policies). When not
set, the general metadata policies are used.

| Tool      | Command                                                                                                    |
|-----------|------------------------------------------------------------------------------------------------------------|
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/subordinates/{subordinateID}/metadata-policies`                        |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/subordinates/{subordinateID}/metadata-policies/{entityType}`           |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/subordinates/{subordinateID}/metadata-policies/{entityType}/{claim}`   |
| Admin API | `GET/PUT/DELETE /api/v1/admin/subordinates/{subordinateID}/metadata-policies/{entityType}/{claim}/{operator}` |

#### Additional Claims (`additional_claims`)

<span class="badge badge-purple" title="Value Type">list of objects</span>

Per-subordinate additional claims, overriding the
[general additional claims](#additional-claims-additional_claims). When not
set, the general additional claims are used.

Each claim object has the following fields:

| Field   | Type    | Description                                  |
|---------|---------|----------------------------------------------|
| `id`    | integer | Read-only identifier (assigned by the API)   |
| `claim` | string  | Claim name                                   |
| `value` | any     | Claim value (any JSON type)                  |
| `crit`  | boolean | Whether the claim is marked as critical      |

| Tool      | Command                                                                          |
|-----------|----------------------------------------------------------------------------------|
| Admin API | `GET/PUT/POST /api/v1/admin/subordinates/{subordinateID}/additional-claims`      |
| Admin API | `GET/PUT/DELETE /api/v1/admin/subordinates/{subordinateID}/additional-claims/{id}` |

### Statement Preview

Preview the subordinate statement (Entity Statement payload) that LightHouse
would issue for a specific subordinate, including all resolved overrides and
fallbacks:

| Tool      | Command                                                                  |
|-----------|--------------------------------------------------------------------------|
| Admin API | `GET /api/v1/admin/subordinates/{subordinateID}/statement`               |

### Event History

View the history of changes for a subordinate (status changes, JWKS updates,
constraint/policy/claim modifications, etc.):

| Tool      | Command                                                                  |
|-----------|--------------------------------------------------------------------------|
| Admin API | `GET /api/v1/admin/subordinates/{subordinateID}/history`                 |
