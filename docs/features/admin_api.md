---
icon: material/api
---

# Admin API

The Admin API provides a comprehensive REST interface for managing your LightHouse federation entity at runtime. It enables programmatic control over entity configuration, subordinate management, trust mark issuance, key management, and more.

## Accessing the API

The Admin API is served at the base path `/api/v1/admin/` (optionally on a 
separate port) and can require HTTP Basic Authentication.

!!! info "Separate Port Configuration"
    The Admin API can be configured to run on a separate port from the main federation endpoints. 
    See [Admin API Configuration](../config/static/api.md) for details.

## Interactive API Documentation

LightHouse includes built-in interactive API documentation via Swagger UI. When the Admin API is enabled, 
you can access the full API reference at:

| Documentation      | Path                               |
|--------------------|------------------------------------|
| Main API           | `/api/v1/admin/docs`               |
| Users API          | `/api/v1/admin/docs/users`         |
| OpenAPI Spec       | `/api/v1/admin/openapi.yaml`       |
| Users OpenAPI Spec | `/api/v1/admin/openapi-users.yaml` |

The Swagger UI provides an interactive interface where you can explore all endpoints, view request/response 
schemas, and test API calls directly from your browser.

!!! tip "Quick Access"
    If your LightHouse instance is running at `https://federation.example.com`, access the API documentation at:
    
    - `https://federation.example.com/api/v1/admin/docs`

## Endpoint Categories

The Admin API is organized into the following functional areas:

### Keys

Manage the cryptographic keys used for signing federation statements and entity configurations.

- **JWKS Management** - View and manage the JSON Web Key Set published in your entity configuration
- **Public Key Operations** - Add, rotate, revoke, and delete signing keys
- **KMS Configuration** - Configure signing algorithm, RSA key length, and automatic key rotation settings

### Entity Configuration

Manage your federation entity's published configuration.

- **Metadata** - Update metadata for different entity types (federation_entity, openid_provider, etc.)
- **Authority Hints** - Manage the list of superior entities in your trust chain
- **Additional Claims** - Add custom claims to your entity configuration
- **Trust Marks** - Configure trust marks to be included in your entity configuration (external, self-issued, or directly provided)
- **Lifetime** - Configure the validity period of your entity configuration

### Subordinates

Full lifecycle management of subordinate entities in your federation.

- **Registration** - Add new subordinate entities
- **Status Management** - Approve, suspend, or remove subordinates
- **JWKS** - Manage subordinate signing keys
- **JWKS Refreshing** - Enable periodic JWKS refresh per subordinate (`enable_jwks_update`, `jwks_poll_interval`); see [Subordinate JWKS Refreshing](subordinate_jwks_refresh.md)
- **Metadata** - Configure subordinate-specific metadata
- **Metadata Policies** - Define policies that apply to subordinate metadata
- **Constraints** - Set constraints on subordinate trust chains
- **Additional Claims** - Add custom claims to subordinate statements
- **Statement Preview** - Preview the subordinate statement that would be issued
- **Event History** - View the history of changes for a subordinate
- **Events** - Query the events of all entities, including removed subordinates, by entity, type, actor and time

### Federation Trust Marks

Configure trust mark issuance for your federation.

- **Trust Mark Types** - Define the types of trust marks your entity can issue
- **Owners & Issuers** - Configure trust mark delegation (owners and authorized issuers)
- **Issuance Specifications** - Define issuance parameters for each trust mark type
- **Subjects** - Manage which entities are entitled to receive specific trust marks, optionally issuing the trust mark immediately
- **Issued Instances** - List, count and revoke the trust marks your entity has issued

### Trust Anchors

Manage the [Trust Anchor Repository](trust_anchors.md) — the single source of
truth for all trust anchors and their JWKS.

- **CRUD** - Create, read, update, and delete trust anchors
- **JWKS Refreshing** - Enable or disable automatic JWKS refreshing per trust anchor
- **Polling Interval** - Configure how often each trust anchor's entity configuration is polled for key changes

Endpoints:

| Operation | Method | Path |
|-----------|--------|------|
| List all trust anchors | `GET` | `/api/v1/admin/trust-anchors` |
| Get a trust anchor | `GET` | `/api/v1/admin/trust-anchors/{entityID}` |
| Create a trust anchor | `POST` | `/api/v1/admin/trust-anchors` |
| Update a trust anchor | `PUT` | `/api/v1/admin/trust-anchors/{entityID}` |
| Delete a trust anchor | `DELETE` | `/api/v1/admin/trust-anchors/{entityID}` |

### Federation Endpoints

Manage federation endpoint paths, URLs, authentication, and type-specific
configuration at runtime. See [Endpoints](endpoints.md) for details.

- **CRUD** - Create, read, update, and delete federation endpoints by type
- **Authentication** - Enable/disable `private_key_jwt` auth and assign trust anchors
- **Type-Specific Config** - Configure resolve settings, entity collection, enroll checkers, etc.
- **Dynamic Reload** - Changes take effect immediately without restart

Endpoints:

| Operation | Method | Path |
|-----------|--------|------|
| List all endpoints | `GET` | `/api/v1/admin/federation-endpoints` |
| Get an endpoint by type | `GET` | `/api/v1/admin/federation-endpoints/{type}` |
| Create an endpoint | `POST` | `/api/v1/admin/federation-endpoints` |
| Update an endpoint | `PUT` | `/api/v1/admin/federation-endpoints/{type}` |
| Delete an endpoint | `DELETE` | `/api/v1/admin/federation-endpoints/{type}` |
| Set auth trust anchors | `PUT` | `/api/v1/admin/federation-endpoints/{type}/auth-trust-anchors` |

### Users

Manage admin users for API access. This functionality is available at a separate Swagger UI endpoint (`/api/v1/admin/docs/users`) when user management is enabled.

- **User CRUD** - Create, read, update, and delete admin users
- **Password Management** - Set and update user passwords

!!! info "Authentication Behavior"
    The whole Admin API has the following authentication behavior for initial setup:
    
    - **No users exist**: The API does not require authentication, allowing you to create the first admin user
    - **At least one user exists**: All API requests require HTTP Basic Authentication with valid credentials

    This applies to the admin port. Bearer tokens are refused there once at
    least one user exists; while no user exists the admin port checks no
    credentials, so a bearer token sent there is neither checked nor used as
    the actor. The [service listener](#service-listener-and-service-tokens)
    behaves differently.

### Service Listener and Service Tokens

A service, such as an operator UI or an automation running as its own
service, can call the Admin API on a separate listener,
`api.admin.service_port`, with a bearer service token:

```
Authorization: Bearer <token>
```

- The service listener serves the same Admin API under `/api/v1/admin`, but
  accepts **only** the service tokens configured in
  `api.admin.service_tokens`, also while no users exist. HTTP Basic is
  refused there.
- Service tokens are **not** accepted on the admin port.
- A missing or unknown token returns `401` with error `invalid_client`.
- Tokens are read from files (at least 32 bytes), kept only as SHA-256 hashes
  and compared in constant time. They are never logged or echoed.
- The service listener serves plain HTTP. Bind it to an internal network only,
  for example with `api.admin.service_ip_listen`.

Why not HTTP Basic: a UI makes many calls per page, and each Basic call runs an
Argon2id password check. Keeping the tokens on a listener of their own keeps
them off an admin port that may be reachable from outside.

**Acting for an operator.** The actor recorded in event history for a request
on the service listener is the token's `name`. A token configured with
`act_for_users: true` may instead name, in the `X-Actor` header, the signed-in
user it acts for:

- `X-Actor` must name an existing, enabled user; the user's stored username is
  recorded. Otherwise the request is refused with `400` and error
  `invalid_actor`, and nothing is written.
- While no users exist, no name can be honoured, so any `X-Actor` is refused
  with `400 invalid_actor`. Requests without `X-Actor` are recorded as the
  token's name, so a service can still create the first user.
- Without `X-Actor`, the actor is the token's name.
- Tokens without `act_for_users` ignore `X-Actor`.

The admin port's `actor_source` and `actor_header` options do not apply to the
service listener.

Give tokens names that are not usernames, so that the actor in history tells a
service's own requests apart from a user's.

See the [`service_port`](../config/static/api.md#service_port),
[`service_ip_listen`](../config/static/api.md#service_ip_listen) and
[`service_tokens`](../config/static/api.md#service_tokens) options.

## Security Considerations

!!! warning "Production Deployments"
    
    When deploying the Admin API in production:
    
    - **Use HTTPS** - Always serve the Admin API over TLS to protect credentials and data in transit
    - **Network Isolation** - Consider running the Admin API on a separate port accessible only from trusted networks
    - **Strong Credentials** - Use strong, unique passwords for admin users
    - **Firewall Rules** - Restrict access to the Admin API endpoints using firewall rules

For configuration options including separate port binding and password hashing settings, see 
[Admin API Configuration](../config/static/api.md).
