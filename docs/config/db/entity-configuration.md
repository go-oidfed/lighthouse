---
icon: material/openid
---

# Entity Configuration

These database-managed options control the content and lifetime of the Entity
Configuration JWT and subordinate statements that LightHouse publishes. They are
managed via the [Admin API](../../features/admin_api.md),
[`lhsetup`](../../deployment/lhsetup.md), or
[`lhmigrate config2db`](../../migration/0.22.md).

## Entity Configuration Lifetime (`config_lifetime`)

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">1 day</span>

The lifetime of Entity Configuration JWTs. This defines how long the published
Entity Configuration is valid before it must be re-fetched.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/entity-configuration/lifetime` |
| lhsetup | `lhsetup --only=config_lifetime` |
| config2db | `lhmigrate config2db --only=config_lifetime` |

## Subordinate Statement Lifetime (`statement_lifetime`)

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">~600 000 seconds (≈ 1 week)</span>

The lifetime of Entity Statements (subordinate statements) issued by the Fetch
Endpoint. This defines how long a subordinate statement remains valid.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/entity-statement/lifetime` |
| lhsetup | `lhsetup --only=statement_lifetime` |
| config2db | `lhmigrate config2db --only=statement_lifetime` |

## Federation Entity Metadata (`metadata`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Federation entity metadata included in `metadata.federation_entity` of the
Entity Configuration. Managed as a JSON object via the Admin API.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/metadata` |
| lhsetup | `lhsetup --only=metadata` (via JSON file) |
| config2db | `lhmigrate config2db --only=metadata` |

### Available Fields

| Field | Type | Description |
|-------|------|-------------|
| `display_name` | string | Display name of the entity |
| `description` | string | Description of the entity |
| `keywords` | list of strings | Keywords for the entity |
| `contacts` | list of strings | Contact email addresses |
| `logo_uri` | uri | Logo URI |
| `policy_uri` | uri | Policy URI |
| `information_uri` | uri | Information URI |
| `organization_name` | string | Organization name |
| `organization_uri` | uri | Organization URI |
| `extra` | object | Additional metadata fields |

??? example "Example metadata object"

    ```json
    {
      "display_name": "Example Trust Anchor",
      "organization_name": "Example Organization",
      "contacts": ["contact@example.com"]
    }
    ```

## Constraints (`constraints`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Constraints object included in the Entity Configuration, as defined in the
OpenID Federation Specification. Managed as a JSON object via the Admin API.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/subordinates/constraints` |
| lhsetup | `lhsetup --only=constraints` (via JSON file) |
| config2db | `lhmigrate config2db --only=constraints` |

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

## Metadata Policies (`metadata_policies`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

General metadata policies applicable to all subordinates, as defined in the
OpenID Federation Specification. Managed as a JSON object via the Admin API.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/metadata-policies` |
| lhsetup | `lhsetup --only=metadata_policies` (via JSON file) |
| config2db | `lhmigrate config2db --only=metadata_policies` |

## Metadata Policy Crit (`metadata_policy_crit`)

<span class="badge badge-purple" title="Value Type">list of strings</span>

Critical metadata policy operators as per the OpenID Federation Specification.

| Tool | Command |
|------|---------|
| Admin API | `GET/PUT /api/v1/admin/metadata-policy-crit` |
| lhsetup | `lhsetup --only=metadata_policy_crit` |
| config2db | `lhmigrate config2db --only=metadata_policy_crit` |

## Authority Hints (`authority_hints`)

<span class="badge badge-purple" title="Value Type">list of URIs</span>

Entity IDs of Federation Entities that are direct superiors to this entity.
These are published in the `authority_hints` claim of the Entity Configuration.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST/DELETE /api/v1/admin/authority-hints` |
| lhsetup | `lhsetup --only=authority_hints` (add/remove) |
| config2db | `lhmigrate config2db --only=authority_hints` |

## Extra Entity Configuration Claims (`extra_entity_config`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Additional claims included in the Entity Configuration. Each claim can be
individually marked as critical.

| Tool | Command |
|------|---------|
| Admin API | `GET/POST/DELETE /api/v1/admin/entity-configuration/additional-claims` |
| lhsetup | `lhsetup --only=extra_entity_config` (add/remove) |
| config2db | `lhmigrate config2db --only=extra_entity_config` |

!!! note
    All migrated claims have `crit: false` (non-critical) by default. Update
    individual claims via the Admin API if you need to mark them as critical.

??? example "Example"

    ```json
    {
      "custom_claim": "value",
      "another_claim": 42
    }
    ```
