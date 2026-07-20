---
icon: material/shield-check
---

# Trust Anchor Repository

LightHouse maintains a **common trust anchor repository** in the database. This
is the single source of truth for all trust anchors and their JWKS across the
entire LightHouse instance.

## Why a Repository?

Previously, trust anchors were configured independently at multiple locations:

- `endpoints.auth.trust_anchors` (global endpoint auth default)
- Per-endpoint `auth_trust_anchors`
- Entity checker `trust_anchors` (enroll, trust mark checks)
- `allowed_trust_anchors` (resolve, entity collection)

This meant the same trust anchor could appear at several places with duplicated
JWKS, and updating a TA's keys required changing every location.

With the repository, all trust anchors are stored once in the database. Every
usage references a trust anchor by entity ID and resolves its JWKS live from the
repository at request time. Updating a TA's keys in the repository immediately
propagates to all usages.

## Managing Trust Anchors

Trust anchors are managed via the [Admin API](admin_api.md):

| Operation | Endpoint |
|-----------|----------|
| List all trust anchors | `GET /api/v1/admin/trust-anchors` |
| Get a trust anchor | `GET /api/v1/admin/trust-anchors/{entityID}` |
| Create a trust anchor | `POST /api/v1/admin/trust-anchors` |
| Update a trust anchor | `PUT /api/v1/admin/trust-anchors/{entityID}` |
| Delete a trust anchor | `DELETE /api/v1/admin/trust-anchors/{entityID}` |

### Creating a Trust Anchor

```bash
curl -X POST -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "entity_id": "https://ta.example.com",
    "jwks": {"keys": [...]},
    "enable_jwks_update": true,
    "key_poll_interval": 3600
  }' \
  https://localhost:8081/api/v1/admin/trust-anchors
```

### Fields

| Field                | Type    | Description                                                 |
|----------------------|---------|-------------------------------------------------------------|
| `entity_id`          | string  | The entity ID of the trust anchor (required)                |
| `jwks`               | JWKS    | Optional initial JWKS for the trust anchor                  |
| `enable_jwks_update` | boolean | Enable automatic JWKS refreshing (default: `false`)         |
| `key_poll_interval`  | integer | Polling interval in seconds (0 = derive from EC expiration) |

## Automatic JWKS Refreshing

When `enable_jwks_update` is set to `true` for a trust anchor, LightHouse
periodically polls the trust anchor's entity configuration endpoint and updates
the stored JWKS when keys change.

### How It Works

1. LightHouse fetches the trust anchor's entity configuration at
   `/.well-known/openid-federation`.
2. The fetched entity configuration's signature is verified using the current
   JWKS.
3. If the KIDs have changed, the new JWKS is stored in the database and the
   in-memory repository is updated atomically.
4. All usages (client auth, entity checks, etc.) immediately see the new keys.

### Polling Interval

The polling interval is determined as follows:

1. If `key_poll_interval` is set (> 0), that value is used.
2. Otherwise, the entity configuration's expiration time is used (with a small
   buffer).
3. If neither is available, a default of 1 hour is used.

### First-Poll Seed

If a trust anchor is created with `enable_jwks_update=true` but no initial JWKS,
LightHouse will fetch the entity configuration on the first poll and store the
JWKS automatically. This allows bootstrapping a trust anchor with just its
entity ID.

### Error Handling

If a poll fails (network error, signature verification failure, etc.),
LightHouse retries with exponential backoff (starting at 1 second, capped at 5
minutes). The backoff resets to 1 second on the next successful poll.

## See Also

Subordinate JWKS can be refreshed similarly; see
[Subordinate JWKS Refreshing](subordinate_jwks_refresh.md) for the three
supported approaches (periodic polling, trigger endpoint, and signed JWK Set
update endpoint).
