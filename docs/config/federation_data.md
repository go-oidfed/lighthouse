---
icon: material/openid
title: Federation Data
---
<span class="badge badge-yellow" title="Deprecation status">deprecated</span>

The `federation_data` configuration section is **deprecated**. All options have been either 
moved to top-level config or are now managed in the database.

!!! warning "Deprecated Configuration Section"
    
    **This entire section is deprecated and ignored at runtime.**
    
    - **`entity_id`**: Moved to [top-level config](index.md)
    - **All other options**: Now managed in the database via the Admin API
    
    Use [`lhmigrate config2db`](../migration.md#config-to-database-migration-config2db) to 
    migrate values from your existing config file to the database.

## Migrating federation_data

1. **Move `entity_id` to top level** - use `lhmigrate config` to transform your config file automatically
2. **Migrate other options to database** - use `lhmigrate config2db`
3. **Remove `federation_data` from config** - no longer needed after migration

```bash
# Transform config file (moves entity_id to top level)
lhmigrate config --source config.yaml --dest config-new.yaml

# Migrate federation_data values to database
lhmigrate config2db --config=config-new.yaml --db-dir=/data
```

See the [Migration Guide](../migration.md) for details.

## Admin API

After migration, use the Admin API to manage federation data:

| Data | Admin API Endpoint |
|------|-------------------|
| Authority Hints | `GET/POST/DELETE /admin/api/v1/authority-hints` |
| Federation Metadata | `GET/PUT /admin/api/v1/metadata` |
| Constraints | `GET/PUT /admin/api/v1/subordinates/constraints` |
| Configuration Lifetime | `GET/PUT /admin/api/v1/entity-configuration/lifetime` |
| Trust Mark Issuers | `GET/POST/DELETE /admin/api/v1/trust-mark-types/{type}/issuers` |
| Trust Mark Owners | `GET/PUT/DELETE /admin/api/v1/trust-mark-types/{type}/owner` |

---

## Legacy Options Reference

The following options were previously available under `federation_data`. They are 
documented here for **migration reference only** - these options are ignored at runtime.

---

### `entity_id`
<span class="badge badge-yellow">moved</span>

**Moved to top-level [`entity_id`](index.md).**

The entity identifier (URI) for this federation entity.

---

### `authority_hints`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">list of URIs</span>

Entity IDs of Federation Entities that are direct superiors to this entity.

**Migration section:** `authority_hints`

**Admin API:** `GET/POST/DELETE /admin/api/v1/authority-hints`

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        authority_hints:
            - https://ta.example.com
    ```

---

### `federation_entity_metadata`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">object</span>

Federation entity metadata included in `metadata.federation_entity` of the Entity Configuration.

**Migration section:** `metadata`

**Admin API:** `GET/PUT /admin/api/v1/metadata`

Available sub-options (all deprecated):

- `display_name` - Display name of the entity
- `description` - Description of the entity
- `keywords` - List of keywords
- `contacts` - List of contact emails
- `logo_uri` - Logo URI
- `policy_uri` - Policy URI
- `information_uri` - Information URI
- `organization_name` - Organization name
- `organization_uri` - Organization URI
- `extra` - Additional metadata fields

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        federation_entity_metadata:
            display_name: Example Trust Anchor
            organization_name: Example Organization
            contacts:
                - contact@example.com
    ```

---

### `constraints`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">object</span>

Constraints object included in the Entity Configuration.

**Migration section:** `constraints`

**Admin API:** `GET/PUT /admin/api/v1/subordinates/constraints`

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        constraints:
            max_path_len: 2
            naming_constraints:
                permitted:
                    - .example.com
            allowed_entity_types:
                - openid_provider
                - openid_relying_party
    ```

---

### `crit`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">list of strings</span>

Critical claims in subordinate entity statements as per OpenID Federation Specification.

!!! warning "Not Migrated"
    
    This configuration option is **deprecated** and **not migrated** by `lhmigrate config2db`.
    
    The `crit` attribute was used to mark critical claims in subordinate entity statements.
    This functionality has been replaced: additional claims can now be added to entity 
    statements via the Admin API, and each claim can be individually marked as critical.
    
    Use the Admin API to manage additional claims for subordinates:
    
    - `POST /admin/api/v1/subordinates/{id}/additional-claims` - Add a claim (with `crit` flag)
    - `GET /admin/api/v1/subordinates/{id}/additional-claims` - List claims
    - `DELETE /admin/api/v1/subordinates/{id}/additional-claims/{claim}` - Remove a claim

??? file "Legacy config.yaml (no longer supported)"

    ```yaml
    federation_data:
        crit:
            - custom_claim
    ```

---

### `metadata_policy_crit`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">list of strings</span>

Critical metadata policy operators as per OpenID Federation Specification.

**Migration section:** `metadata_policy_crit`

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        metadata_policy_crit:
            - remove
    ```

---

### `configuration_lifetime`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">[duration](index.md#time-duration-configuration-options)</span>
<span class="badge badge-blue">default: 1 day</span>

Lifetime of Entity Configuration JWTs.

**Migration section:** `config_lifetime`

**Admin API:** `GET/PUT /admin/api/v1/entity-configuration/lifetime`

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        configuration_lifetime: 1w
    ```

---

### `trust_anchors`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">list</span>

Trust anchors for resolution. This option is defined but **not used at runtime**.

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        trust_anchors:
            - entity_id: https://ta.example.com
              jwks: {...}
    ```

---

### `metadata_policy_file`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">file path</span>

Path to a JSON file containing the default metadata policy. This option is defined but 
**not used at runtime**. Metadata policies are now managed per-subordinate via the Admin API.

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        metadata_policy_file: /path/to/metadata-policy.json
    ```

---

### `trust_marks`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">list</span>

Trust marks to include in this entity's Entity Configuration. This option is defined but 
**not used at runtime**.

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        trust_marks:
            - trust_mark_type: https://example.com/tm
              trust_mark_issuer: https://example.com/tmi
              refresh: true
    ```

---

### `trust_mark_issuers`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">object</span>

Allowed trust mark issuers per trust mark type.

**Migration section:** `trust_mark_issuers`

**Admin API:** `GET/POST/DELETE /admin/api/v1/trust-mark-types/{type}/issuers`

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        trust_mark_issuers:
            "https://refeds.org/sirtfi":
                - https://example.org
    ```

---

### `trust_mark_owners`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">object</span>

Trust mark owners per trust mark type.

**Migration section:** `trust_mark_owners`

**Admin API:** `GET/PUT/DELETE /admin/api/v1/trust-mark-types/{type}/owner`

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        trust_mark_owners:
            "https://refeds.org/sirtfi":
                sub: https://refeds.org
                jwks: {...}
    ```

---

### `extra_entity_configuration_data`
<span class="badge badge-yellow">deprecated</span>
<span class="badge badge-purple">object</span>

Additional claims to include in the Entity Configuration. This configuration is now 
managed in the database via the Admin API.

!!! info "Migration"
    
    Use [`lhmigrate config2db`](../migration.md#config-to-database-migration-config2db) to 
    migrate this value from a config file to the database:
    
    ```bash
    lhmigrate config2db --config=config.yaml --db-dir=/data --only=extra_entity_config
    ```
    
    All migrated claims will have `crit: false` (non-critical) by default. You can update
    individual claims via the Admin API if you need to mark them as critical.

??? file "Legacy config.yaml (for migration only)"

    ```yaml
    federation_data:
        extra_entity_configuration_data:
            custom_claim: value
    ```
