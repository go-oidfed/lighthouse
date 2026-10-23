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

Each hint may opt into **outbound JWKS syncing**: when LightHouse rotates its
own **federation** keys it can notify the superior so the superior can 
update the JWKS included in the subordinate statement. This is configured per hint via the
optional `jwks_sync` block and is independent for each hint. It never affects
the published `authority_hints` claim and applies only to
federation keys, never to OIDC signing keys.

`jwks_sync.mode` selects one of two mechanisms:

| Mode      | lib hook            | What happens                                                                                                                       |
|-----------|---------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `none`    | —                   | No syncing (default; omitted `jwks_sync` is equivalent).                                                                           |
| `push`    | `JWKSUpdateHook`    | LightHouse pushes a signed JWK Set (`application/jwk-set+jwt`) to the superior's `federation_jwks_update_endpoint`.                |
| `trigger` | `TriggerUpdateHook` | LightHouse POSTs its entity_id (authenticated with `private_key_jwt`) to the superior's `federation_jwks_update_trigger_endpoint`. |

In both modes the target endpoint URL, the acceptable signing algorithms, and
the trigger auth requirement are resolved dynamically from the superior's
Entity Configuration at the time of each rotation (served from the EC cache).
The signed JWK Set (push) and the
`private_key_jwt` assertion (trigger) are signed with LightHouse's federation
signing key.

!!! note "When hooks fire"

    Hooks fire whenever the federation KMS generates or rotates a signing key —
    both scheduled automatic rotations and manual operations (`POST
    /api/v1/admin/kms/rotate`, rotating a key via the public key admin API).
    The initial key seeding at first boot (no key file exists yet) happens
    before hooks are attached and does **not** notify superiors. Hooks require a
    rotation-capable KMS (`db`, `filesystem`, or `pkcs11`); the single-key-file
    mode never fires them.

!!! note "Runtime changes"

    `jwks_sync` changes made via the Admin API (`POST`/`PUT`/`DELETE` on
    authority hints) take effect immediately on the running process: the hooks
    are re-attached after each mutation, without a restart. A failed re-attach
    is logged; the change still applies on the next mutation or restart.

Example (create):

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "entity_id": "https://superior.example.com",
  "jwks_sync": { "mode": "trigger", "timeout": 20 }
}' https://localhost:8081/api/v1/admin/entity-configuration/authority-hints
```

To add or change `jwks_sync` on an **existing** hint, update it with
`PUT /api/v1/admin/entity-configuration/authority-hints/{authorityHintID}`
(`authorityHintID` is the numeric hint id or the entity_id). `entity_id` is
required to create a hint but **optional on update** — omit it to keep the
existing value unchanged:

```bash
# Changing only jwks_sync; entity_id is left untouched (no conflict).
curl -X PUT -H "Content-Type: application/json" -d '{
  "jwks_sync": { "mode": "push", "timeout": 20, "jwt_lifetime": 600 }
}' https://localhost:8081/api/v1/admin/entity-configuration/authority-hints/123
```

Disable syncing again by setting `jwks_sync.mode` to `none` (or delete the
hint with `DELETE`). A `POST` to the collection re-creates a hint and therefore
409s if the entity_id already exists.

#### `jwks_sync.mode`

<span class="badge badge-purple" title="Value Type">string</span>
<span class="badge badge-blue" title="Default Value">none</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

One of `none`, `push`, or `trigger` (see the table above).

#### `jwks_sync.jwt_lifetime`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>
<span class="badge badge-blue" title="Default Value">600</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The lifetime (`exp` - `iat`) of the signed JWK Set JWT sent in `push` mode,
in seconds. Ignored in other modes.

#### `jwks_sync.timeout`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>
<span class="badge badge-blue" title="Default Value">20</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

The HTTP client timeout for the sync request, in seconds.

#### `jwks_sync.headers`

<span class="badge badge-purple" title="Value Type">map of string to string</span>
<span class="badge badge-green" title="If this option is required or optional">optional</span>

Additional HTTP headers set on the sync request.

| Tool      | Command                                                              |
|-----------|----------------------------------------------------------------------|
| Admin API | `GET/POST/PUT/DELETE /api/v1/admin/entity-configuration/authority-hints` |
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
