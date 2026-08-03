---
icon: material/shield-check
---

# Trust Anchors

LightHouse maintains a **common trust anchor repository** in the database. This
is the single source of truth for all trust anchors and their JWKS. See the
[Trust Anchor Repository](../../features/trust_anchors.md) feature documentation
for additional details.

## Fields

### `entity_id`

<span class="badge badge-purple" title="Value Type">string (URI)</span>
<span class="badge badge-red" title="Required">required</span>

The entity ID of the trust anchor.

### `jwks`

<span class="badge badge-purple" title="Value Type">JWKS object</span>
<span class="badge badge-green" title="Optional">optional (recommended)</span>

The JWKS of the trust anchor, obtained out-of-band. If omitted, LightHouse will
fetch it from the trust anchor's Entity Configuration on the first poll.

### `enable_jwks_update`

<span class="badge badge-purple" title="Value Type">boolean</span>
<span class="badge badge-blue" title="Default Value">`false`</span>

When `true`, LightHouse periodically polls the trust anchor's Entity
Configuration and updates the stored JWKS when keys change. See
[Automatic JWKS Refreshing](../../features/trust_anchors.md#automatic-jwks-refreshing)
for details.

### `key_poll_interval`

<span class="badge badge-purple" title="Value Type">integer (seconds)</span>
<span class="badge badge-blue" title="Default Value">`0` (derive from EC expiration)</span>

Polling interval in seconds. When `0` (or `<= 0`), LightHouse derives the
interval from the trust anchor's Entity Configuration expiration time. See
[Polling Interval](../../features/trust_anchors.md#polling-interval) for the
resolution logic.

## Management

| Tool | Command |
|------|---------|
| Admin API | `GET /api/v1/admin/trust-anchors` (list) |
| Admin API | `POST /api/v1/admin/trust-anchors` (create) |
| Admin API | `GET/PUT/DELETE /api/v1/admin/trust-anchors/{entityID}` |
| lhsetup | `lhsetup --only=trust_anchors` |
| config2db | `lhmigrate config2db --only=trust_anchors` |
