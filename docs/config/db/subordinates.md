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

Per-subordinate configuration that controls JWKS refreshing. Subordinates
themselves (entity ID, JWKS, metadata, policies, status, etc.) are managed via
the [Admin API](../../features/admin_api.md). See
[Subordinate JWKS Refreshing](../../features/subordinate_jwks_refresh.md) for
the full feature documentation.

In addition to the JWKS refresh fields below, per-subordinate overrides for
constraints, metadata, metadata policies, and additional claims are available
via the Admin API at `/api/v1/admin/subordinates/{id}/...`. When set, these
overrides take precedence over the
[general subordinate statement configuration](#general-subordinate-statement-configuration).

### JWKS Refresh Fields

These fields are set on each subordinate and control periodic JWKS polling.

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

### Management

Set these fields when creating or updating a subordinate via the Admin API:

```bash
# Create a subordinate with periodic refreshing enabled
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

# Enable refreshing on an existing subordinate (PATCH preferred for partial updates)
curl -X PATCH -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"enable_jwks_update": true, "jwks_poll_interval": 7200}' \
  https://localhost:8081/api/v1/admin/subordinates/{subordinateID}
```

| Tool | Command |
|------|---------|
| Admin API | `POST/PATCH /api/v1/admin/subordinates` or `/api/v1/admin/subordinates/{id}` |
| lhsetup | Not applicable (per-subordinate, set via Admin API or enroll endpoint) |
