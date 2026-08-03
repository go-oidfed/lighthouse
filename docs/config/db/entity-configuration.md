---
icon: material/openid
---

# Entity Configuration

These database-managed options control the content and lifetime of the Entity
Configuration JWT that LightHouse publishes. They are managed via the
[Admin API](../../features/admin_api.md),
[`lhsetup`](../../deployment/lhsetup.md), or
[`lhmigrate config2db`](../../migration/0.22.md).

Subordinate statement options are documented in
[Subordinates](subordinates.md).

## Entity Configuration Lifetime (`config_lifetime`)

<span class="badge badge-purple" title="Value Type">[duration](../index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue" title="Default Value">1 day</span>

The lifetime of Entity Configuration JWTs. This defines how long the published
Entity Configuration is valid before it must be re-fetched.

| Tool      | Command                                               |
|-----------|-------------------------------------------------------|
| Admin API | `GET/PUT /api/v1/admin/entity-configuration/lifetime` |
| lhsetup   | `lhsetup --only=config_lifetime`                      |
| config2db | `lhmigrate config2db --only=config_lifetime`          |

## Entity Metadata (`metadata`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Entity metadata included in the `metadata` claim of the Entity Configuration.
Supports different entity types, not just
`federation_entity`. Managed as a JSON object via the Admin API.

The metadata is the full `Metadata` mapping (entity type → claims). Use the
granular endpoints to manage a single entity type or a single claim without
replacing the whole object.

| Tool      | Command                                                                         |
|-----------|---------------------------------------------------------------------------------|
| Admin API | `GET/PUT /api/v1/admin/entity-configuration/metadata`                           |
| Admin API | `GET/PUT/POST/DELETE /api/v1/admin/entity-configuration/metadata/:entityType`   |
| Admin API | `GET/PUT/DELETE /api/v1/admin/entity-configuration/metadata/:entityType/:claim` |
| lhsetup   | `lhsetup --only=metadata` (via JSON file)                                       |
| config2db | `lhmigrate config2db --only=metadata`                                           |

### Common `federation_entity` Fields

The fields below are the informational claims most commonly set on the
`federation_entity` entity type. Other entity types (e.g. `openid_provider`)
have their own type-specific fields; see the OpenID Federation specification
for details.

| Field               | Type            | Description                |
|---------------------|-----------------|----------------------------|
| `display_name`      | string          | Display name of the entity |
| `description`       | string          | Description of the entity  |
| `keywords`          | list of strings | Keywords for the entity    |
| `contacts`          | list of strings | Contact email addresses    |
| `logo_uri`          | uri             | Logo URI                   |
| `policy_uri`        | uri             | Policy URI                 |
| `information_uri`   | uri             | Information URI            |
| `organization_name` | string          | Organization name          |
| `organization_uri`  | uri             | Organization URI           |
| anything else       | json            | Additional metadata fields |

??? example "Example metadata object"

    ```json
    {
      "federation_entity": {
        "display_name": "Example Trust Anchor",
        "organization_name": "Example Organization",
        "contacts": ["contact@example.com"]
      }
    }
    ```

## Authority Hints (`authority_hints`)

<span class="badge badge-purple" title="Value Type">list of URIs</span>

Entity IDs of Federation Entities that are direct superiors to this entity.
These are published in the `authority_hints` claim of the Entity Configuration.

| Tool      | Command                                                              |
|-----------|----------------------------------------------------------------------|
| Admin API | `GET/POST/DELETE /api/v1/admin/entity-configuration/authority-hints` |
| lhsetup   | `lhsetup --only=authority_hints` (add/remove)                        |
| config2db | `lhmigrate config2db --only=authority_hints`                         |

## Extra Entity Configuration Claims (`extra_entity_config`)

<span class="badge badge-purple" title="Value Type">object / mapping</span>

Additional claims included in the Entity Configuration. Each claim can be
individually marked as critical.

| Tool      | Command                                                                |
|-----------|------------------------------------------------------------------------|
| Admin API | `GET/POST/DELETE /api/v1/admin/entity-configuration/additional-claims` |
| lhsetup   | `lhsetup --only=extra_entity_config` (add/remove)                      |
| config2db | `lhmigrate config2db --only=extra_entity_config`                       |

!!! note
    All migrated claims have `crit: false` (non-critical) by default. Update
    individual claims via the Admin API if you need to mark them as critical.
