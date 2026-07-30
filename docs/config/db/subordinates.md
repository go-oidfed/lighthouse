---
icon: material/account-group
---

# Subordinates

Per-subordinate configuration that controls JWKS refreshing. Subordinates
themselves (entity ID, JWKS, metadata, policies, status, etc.) are managed via
the [Admin API](../../features/admin_api.md). See
[Subordinate JWKS Refreshing](../../features/subordinate_jwks_refresh.md) for
the full feature documentation.

## JWKS Refresh Fields

These fields are set on each subordinate and control periodic JWKS polling.

### `enable_jwks_update`

<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>

When `true`, LightHouse periodically polls the subordinate's Entity
Configuration and updates the stored JWKS when keys change. The fetched
Entity Configuration's signature is verified against the currently stored
JWKS; if verification fails, the JWKS is **not** updated.

### `jwks_poll_interval`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>
<span class="badge badge-blue" title="Default Value">`0` (derive from EC expiration)</span>

Poll interval in seconds. When `0` (the default) or `<= 0`, LightHouse derives
the interval from the subordinate's Entity Configuration expiration time
(with a small buffer), floored to a minimum of **1 minute**.

## Management

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
