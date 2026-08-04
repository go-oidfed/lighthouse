---
icon: material/routes
---

# Federation Endpoints

Federation endpoints are now managed in the database via the
[Admin API](admin_api.md). Their paths, URLs, authentication settings, and
type-specific configuration can be changed at runtime without restarting
LightHouse.

## Available Endpoints

| Endpoint                      | Type                  | Description                                                                                                                                                                                        |
|-------------------------------|-----------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Federation Config             | n/a                   | Always enabled. The federation endpoint where the entity configuration is published.                                                                                                               |
| Fetch                         | `fetch`               | Federation Subordinate Fetch Endpoint per Spec Section 8.1                                                                                                                                         |
| Subordinate Listing           | `list`                | Federation Subordinate Listing Endpoint per Spec Section 8.2                                                                                                                                       |
| Resolve                       | `resolve`             | Resolve Endpoint per Spec Section 8.3                                                                                                                                                              |
| Trust Mark Status             | `trust_mark_status`   | Trust Mark Status Endpoint per Spec Section 8.4                                                                                                                                                    |
| Trust Marked Entities Listing | `trust_mark_listing`  | Trust Marked Entities Listing Endpoint per Spec Section 8.5                                                                                                                                        |
| Trust Mark                    | `trust_mark`          | Trust Mark Endpoint per Spec Section 8.6                                                                                                                                                           |
| Federation Historical Keys    | `historical_keys`     | Historical Keys Endpoint per Spec Section 8.7; only usable with automatic key rollover                                                                                                             |
| Enrollment                    | `enroll`              | An endpoint where entities can automatically enroll into the federation. For details see [Enrolling Entities](#enrolling-entities)                                                                 |
| Request Enrollment            | `enroll_request`      | An endpoint where entities can request enrollment into the federation. An federation administrator then can check and approve the request.                                                         |
| Trust Mark Request            | `trust_mark_request`  | An endpoint where entities can request to be entitled for a trust mark. A federation administrator then can check and approve the request.                                                         |
| Entity Collection             | `entity_collection`   | An endpoint to query a filterable list of all entities in a federation. Per [Entity Collection Endpoint Extension Draft](https://zachmann.github.io/openid-federation-entity-collection/main.html) |
| JWKS Update Trigger           | `jwks_update_trigger` | POST trigger for a subordinate to request LightHouse re-fetches its JWKS from its Entity Configuration. See [Subordinate JWKS Refreshing](subordinate_jwks_refresh.md)                             |
| JWKS Update                   | `jwks_update`         | POST endpoint accepting a signed JWK Set (`application/jwk-set+jwt`) from the subordinate with its new federation keys. See [Subordinate JWKS Refreshing](subordinate_jwks_refresh.md)             |

## Managing Endpoints

Endpoints are managed via the [Admin API](admin_api.md):

| Operation | Endpoint |
|-----------|----------|
| List all endpoints | `GET /api/v1/admin/federation-endpoints` |
| Get an endpoint by type | `GET /api/v1/admin/federation-endpoints/{type}` |
| Create an endpoint | `POST /api/v1/admin/federation-endpoints` |
| Update an endpoint | `PUT /api/v1/admin/federation-endpoints/{type}` |
| Delete an endpoint | `DELETE /api/v1/admin/federation-endpoints/{type}` |
| Set auth trust anchors | `PUT /api/v1/admin/federation-endpoints/{type}/auth-trust-anchors` |

### Creating an Endpoint

```bash
curl -X POST -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "type": "fetch",
    "path": "/fetch",
    "auth_enabled": true,
    "auth_trust_anchors": ["https://ta1.example.com", "https://ta2.example.com"]
  }' \
  https://localhost:8081/api/v1/admin/federation-endpoints
```

### Disabling an Endpoint

Set `path` to `null` to disable an endpoint without deleting it. The endpoint
will not be served and its URL will be omitted from the entity configuration.

### Type-Specific Configuration

Some endpoints have type-specific configuration stored as a JSON `config` field.
See the [Federation Endpoints Configuration Reference](../config/db/federation-endpoints.md#type-specific-configuration)
for the full field reference and JSON shapes for each endpoint type.

When `use_entity_collection_allowed_trust_anchors` is `true` (resolve endpoint),
the resolve endpoint dynamically uses the entity collection endpoint's
`allowed_trust_anchors` at load time. Changes to the entity collection's TAs
via the Admin API automatically propagate to the resolve endpoint on the next
registry reload.

The `trust_anchors` in enroll checker configs are **entity ID strings** (not
inline trust anchor objects with JWKS). They are resolved live from the
[Trust Anchor Repository](trust_anchors.md) at check time.

When `interval_seconds` is set (> 0) on the entity collection endpoint, a
background periodic entity collector is started. Changes via the Admin API stop
and restart the collector with the new
settings.

## Dynamic Dispatch

All federation endpoints are served via a single catch-all dispatcher. When a
request comes in:

1. The dispatcher looks up the request path in an in-memory endpoint registry.
2. If found, the type-specific handler is invoked (with auth middleware if
   enabled).
3. If not found, a 404 is returned.

When endpoints are created, updated, or deleted via the Admin API, the registry
is rebuilt atomically (copy-on-write under a lock) and the entity configuration
cache is invalidated, so the published federation metadata immediately reflects
the new endpoint URLs.

## Authentication

Each endpoint can optionally require `private_key_jwt` authentication. When
enabled, the endpoint's `auth_trust_anchors` (linked via the join table to the
[Trust Anchor Repository](trust_anchors.md)) are used to verify client
assertions. The trust anchors' JWKS are resolved live from the repository, so
key updates propagate instantly.

## Enrolling Entities

LightHouse implements a custom enrollment / onboarding endpoint which can be
managed via the Admin API. This endpoint is used to easily add entities to the
federation. Entities can also be manually added to the database (or with a
simple command line application).

The enrollment endpoint can also be guarded by so-called
[Entity Checks](entity_checks.md). If the enroll endpoint is enabled, but no
checks defined, all entities can enroll (obviously not recommended outside a
proof-of-concept).

### Enrollment Request

To enroll, the entity sends a `GET` request to the enroll endpoint with the
following request parameter:

| Parameter     | Necessity   | Description     |
|---------------|-------------|-----------------|
| `sub`         | REQUIRED    | Its entity id   |
| `entity_type` | RECOMMENDED | Its entity type |

`entity_type` can be provided multiple times to pass multiple entity types.

LightHouse will query the entity's federation endpoint for its Entity
Configuration and obtain the jwks from there and (if configured) performs the
entity checks.
