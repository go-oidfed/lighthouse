---
icon: material/routes
---

# Federation Endpoints

Federation endpoints are managed in the database. Their paths, URLs,
authentication settings, and type-specific configuration can be changed at
runtime via the Admin API without restarting LightHouse. See the
[Federation Endpoints](../../features/endpoints.md) feature documentation for
dynamic dispatch, authentication, and enrollment details.

## Endpoint Types

| Endpoint Type         | Description                                                                                                                            |
|-----------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| `fetch`               | Federation Subordinate Fetch Endpoint (Spec §8.1)                                                                                      |
| `list`                | Federation Subordinate Listing Endpoint (Spec §8.2)                                                                                    |
| `resolve`             | Resolve Endpoint (Spec §8.3)                                                                                                           |
| `trust_mark_status`   | Trust Mark Status Endpoint (Spec §8.4)                                                                                                 |
| `trust_mark_listing`  | Trust Marked Entities Listing Endpoint (Spec §8.5)                                                                                     |
| `trust_mark`          | Trust Mark Endpoint (Spec §8.6)                                                                                                        |
| `historical_keys`     | Historical Keys Endpoint (Spec §8.7); requires automatic key rollover                                                                  |
| `enroll`              | Automatic enrollment endpoint                                                                                                          |
| `enroll_request`      | Request enrollment endpoint (admin approval)                                                                                           |
| `trust_mark_request`  | Trust mark request endpoint (admin approval)                                                                                           |
| `entity_collection`   | Entity Collection Endpoint ([extension draft](https://openid.github.io/federation-entity-collection/main.html))                        |
| `jwks_update_trigger` | POST trigger for a subordinate to request JWKS re-fetch. See [Subordinate JWKS Refreshing](../../features/subordinate_jwks_refresh.md) |
| `jwks_update`         | POST endpoint accepting a signed JWK Set. See [Subordinate JWKS Refreshing](../../features/subordinate_jwks_refresh.md)                |

## Common Fields

All endpoint types share these fields:

| Field                | Type                         | Description                                                                                                  |
|----------------------|------------------------------|--------------------------------------------------------------------------------------------------------------|
| `type`               | string (enum)                | The endpoint type (see table above)                                                                          |
| `path`               | string \| null               | URL path under which the endpoint is served. Set to `null` to disable the endpoint without deleting it.      |
| `url`                | string \| null               | External URL published in the Entity Configuration. If not set, the URL is derived from `<entity_id><path>`. |
| `auth_enabled`       | boolean                      | When `true`, requires `private_key_jwt` client authentication.                                               |
| `auth_trust_anchors` | list of strings (entity IDs) | Trust anchor entity IDs used to verify client assertions (when `auth_enabled` is `true`).                    |
| `config`             | JSON                         | Type-specific configuration (see below).                                                                     |

## Type-Specific Configuration

Some endpoint types store additional configuration in a JSON `config` field.

### Resolve (`resolve`)

```json
{
  "allowed_trust_anchors": ["https://ta.example.com"],
  "use_entity_collection_allowed_trust_anchors": true,
  "grace_period_seconds": 3600,
  "time_elapsed_grace_factor": 0.5,
  "proactive_resolver": {
    "enabled": false,
    "concurrency_limit": 64,
    "queue_size": 10000,
    "response_storage_dir": "/var/lib/lighthouse/resolver",
    "response_storage_store_json": false,
    "response_storage_store_jwt": true
  }
}
```

| Field | Description |
|-------|-------------|
| `allowed_trust_anchors` | List of trust anchor entity IDs permitted on the resolver. |
| `use_entity_collection_allowed_trust_anchors` | When `true`, dynamically uses the entity collection endpoint's `allowed_trust_anchors`. |
| `grace_period_seconds` | Grace period for the resolver cache (seconds). |
| `time_elapsed_grace_factor` | Fraction of lifetime that must elapse before a grace-period refresh is triggered. |
| `proactive_resolver` | Background resolver that proactively refreshes cached statements. See [features/endpoints.md](../../features/endpoints.md) for requirements. |

### Enroll (`enroll`)

```json
{
  "checker_type": "multiple_or",
  "checker_config": [
    {
      "type": "trust_mark",
      "config": {
        "trust_mark_type": "https://tm.example.org",
        "trust_anchors": ["https://ta.example.org"]
      }
    },
    {
      "type": "trust_path",
      "config": {
        "trust_anchors": ["https://ta2.example.org"]
      }
    }
  ]
}
```

The `trust_anchors` in checker configs are **entity ID strings** (not inline
trust anchor objects with JWKS). They are resolved live from the
[Trust Anchor Repository](trust-anchors.md) at check time. See
[Entity Checks](../../features/entity_checks.md) for checker configuration
details.

### Entity Collection (`entity_collection`)

```json
{
  "allowed_trust_anchors": ["https://ta.example.com"],
  "interval_seconds": 28800,
  "concurrency_limit": 4,
  "pagination_limit": 512
}
```

| Field | Description |
|-------|-------------|
| `allowed_trust_anchors` | Trust anchor entity IDs permitted in requests. |
| `interval_seconds` | When `> 0`, starts a background periodic collector. |
| `concurrency_limit` | How many periodic collection tasks may run in parallel. |
| `pagination_limit` | Maximum page size for paginated requests. |

## Management

| Tool | Command |
|------|---------|
| Admin API | `GET /api/v1/admin/federation-endpoints` (list) |
| Admin API | `POST /api/v1/admin/federation-endpoints` (create) |
| Admin API | `GET/PUT/DELETE /api/v1/admin/federation-endpoints/{type}` |
| Admin API (auth TAs) | `PUT /api/v1/admin/federation-endpoints/{type}/auth-trust-anchors` |
| lhsetup | `lhsetup --only=endpoints` |
| config2db | `lhmigrate config2db --only=endpoints` |

!!! note "New endpoint types"
    `jwks_update_trigger` and `jwks_update` have no equivalent in the old config
    file. `lhmigrate config2db` does not create them — use the Admin API or
    `lhsetup` to create them if you need subordinate JWKS refreshing.
