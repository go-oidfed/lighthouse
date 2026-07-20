---
icon: material/key-change
---

# Subordinate JWKS Refreshing

A subordinate statement issued by LightHouse includes the subordinate's `jwks`
claim (per [Spec Section 3.1.1][spec-3.1.1]). When a subordinate rotates its
federation keys, LightHouse must update the stored JWKS so that subsequent
subordinate statements contain the current keys.
Otherwise Trust Chains cannot be resolved correctly after the key switch.

LightHouse supports three complementary mechanisms for keeping a subordinate's
JWKS up to date. They can be used independently or together (may not always 
make sense).

| Mechanism                       | Direction        | Trigger                        | Transport                  |
|---------------------------------|------------------|--------------------------------|----------------------------|
| Periodic refreshing             | LH → subordinate | LH polls on a schedule         | LH fetches the EC          |
| JWKS Update Trigger endpoint    | Subordinate → LH | Subordinate POSTs a trigger    | LH fetches the EC          |
| JWKS Update endpoint            | Subordinate → LH | Subordinate POSTs its new keys | Subordinate sends the keys |


## Periodic Refreshing

LightHouse periodically polls the subordinate's Entity Configuration and updates
the stored JWKS when the keys change. This is opt-in per subordinate.

### Enabling per Subordinate

Two fields on a subordinate control periodic refreshing:

| Field                | Type            | Description                                                                                         |
|----------------------|-----------------|-----------------------------------------------------------------------------------------------------|
| `enable_jwks_update` | boolean         | Enable periodic refreshing (default: `false`)                                                       |
| `jwks_poll_interval` | integer \| null | Poll interval in seconds. `null`/`<= 0` ⇒ derive from the EC expiration time (floored to 1 minute). |

Set them when creating or updating a subordinate via the
[Admin API](admin_api.md):

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
```

```bash
# Enable refreshing on an existing subordinate
curl -X PUT -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"enable_jwks_update": true, "jwks_poll_interval": 7200}' \
  https://localhost:8081/api/v1/admin/subordinates/{subordinateID}
```

### How It Works

1. LightHouse fetches the subordinate's Entity Configuration at
   `/.well-known/openid-federation`.
2. The fetched EC's signature is verified against the **currently stored** JWKS.
   If verification fails, the JWKS is **not** updated.
3. If the KIDs have changed, the new JWKS is stored in the database.
4. The next subordinate statement LightHouse issues will contain the new keys.

### Polling Interval

The interval is determined as follows:

1. If `jwks_poll_interval` is set (`> 0`), that value is used.
2. Otherwise, the EC's expiration time is used (with a small buffer), floored to
   a minimum of **1 minute**.

### Dynamic Updates

The refresher is started once at boot and then reconciles its poll set
dynamically:

- Creating a subordinate with `enable_jwks_update=true` (via Admin API or the
  [enroll endpoint](endpoints.md#enrolling-entities)) starts polling it
  immediately.
- Updating `enable_jwks_update`, `jwks_poll_interval`, or the status of a
  subordinate restarts its polling goroutine.
- Deleting a subordinate stops polling it.

No restart is required.

### Error Handling

If a poll fails (network error, signature verification failure, missing `exp`,
etc.), LightHouse retries with exponential backoff (starting at 1 second,
capped at 5 minutes). The backoff resets on the next successful poll.


## JWKS Update Trigger Endpoint

LightHouse can expose a `federation_jwks_update_trigger_endpoint` (endpoint type
`jwks_update_trigger`) that a subordinate can POST to in order to request that
LightHouse re-fetches its JWKS from its Entity Configuration. The logic is the
same as periodic refreshing, but instead of polling on a schedule, the refresh
is triggered on demand by the subordinate.

### Publishing the Endpoint

Create the endpoint via the [Admin API](admin_api.md):

```bash
curl -X POST -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "type": "jwks_update_trigger",
    "path": "/jwks/update-trigger",
    "auth_enabled": false
  }' \
  https://localhost:8081/api/v1/admin/federation-endpoints
```

The endpoint URL is published in the `federation_entity` metadata under
`federation_jwks_update_trigger_endpoint`.

### Request

`POST` to the endpoint. The target subordinate is identified depending on
whether client auth is enabled:

| `auth_enabled` | Target identification                                                                                    |
|----------------|----------------------------------------------------------------------------------------------------------|
| `true`         | The authenticated client entity from the `private_key_jwt` assertion is the target. The body is ignored. |
| `false`        | The subordinate's entity ID is supplied as `sub` in the request body.                                    |

```bash
# Without client auth
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"sub": "https://sub.example.com"}' \
  https://federation.example.com/jwks/update-trigger
```

!!! note "Eligible subordinates"
    The target must be a known subordinate with status `active` or `pending`.
    Blocked or inactive subordinates are rejected. Periodic refreshing does
    **not** need to be enabled for the trigger endpoint to work.


## JWKS Update Endpoint

LightHouse can expose a `federation_jwks_update_endpoint` (endpoint type
`jwks_update`) that a subordinate can POST to with its new keys directly,
encoded as a signed JWK Set. This reuses the signed JWK Set format defined for
`signed_jwks_uri` in [Spec Section 5.2.1][spec-5.2.1].

### Publishing the Endpoint

```bash
curl -X POST -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "type": "jwks_update",
    "path": "/jwks/update"
  }' \
  https://localhost:8081/api/v1/admin/federation-endpoints
```

The endpoint URL is published in the `federation_entity` metadata under
`federation_jwks_update_endpoint`.

!!! note "No client auth"
    The `jwks_update` endpoint does **not** use `private_key_jwt` client
    authentication. Authenticity is established by the signature on the signed
    JWK Set, which must be verifiable with one of the subordinate's currently
    known federation keys. `auth_enabled` is ignored for this endpoint type.

### Request

`POST` with content type `application/jwk-set+jwt`. The body is a signed JWT
with:

- **Header**: `typ: jwk-set+jwt` and a `kid` identifying the signing key.
- **Payload claims** (per spec §5.2.1):

| Claim  | Required | Description                                      |
|--------|----------|--------------------------------------------------|
| `keys` | yes      | Array of JWK values — the new key set.           |
| `iss`  | yes      | Issuer — the subordinate's entity ID.            |
| `sub`  | yes      | Owner of the keys - the subordinate's entity ID. |
| `iat`  | no       | Issued-at timestamp.                             |
| `exp`  | no       | Expiration of the JWT.                           |

Every key in `keys` **must** have a unique, non-empty `kid`.

### Example

```bash
curl -X POST \
  -H "Content-Type: application/jwk-set+jwt" \
  --data-binary @signed-jwk-set.jwt \
  https://federation.example.com/jwks/update
```

## Event History

Each mechanism records an event in the subordinate's
[event history](admin_api.md), visible via
`GET /api/v1/admin/subordinates/{subordinateID}/history`:

| Event type              | Recorded when                                                                |
|-------------------------|------------------------------------------------------------------------------|
| `jwks_refreshed`        | Periodic refresh detected a change and updated the JWKS.                     |
| `jwks_update_triggered` | Trigger endpoint was called. The message indicates whether the keys changed. |
| `jwks_updated`          | Update endpoint accepted a signed JWK Set.                                   |

These complement the existing `jwks_replaced`, `jwk_added`, and `jwk_removed`
events recorded by manual admin-API JWKS operations.
