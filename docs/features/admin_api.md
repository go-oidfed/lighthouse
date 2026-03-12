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
    See [Admin API Configuration](../config/api.md) for details.

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
- **Metadata** - Configure subordinate-specific metadata
- **Metadata Policies** - Define policies that apply to subordinate metadata
- **Constraints** - Set constraints on subordinate trust chains
- **Additional Claims** - Add custom claims to subordinate statements
- **Statement Preview** - Preview the subordinate statement that would be issued
- **Event History** - View the history of changes for a subordinate

### Federation Trust Marks

Configure trust mark issuance for your federation.

- **Trust Mark Types** - Define the types of trust marks your entity can issue
- **Owners & Issuers** - Configure trust mark delegation (owners and authorized issuers)
- **Issuance Specifications** - Define issuance parameters for each trust mark type
- **Subjects** - Manage which entities are entitled to receive specific trust marks

### Users

Manage admin users for API access. This functionality is available at a separate Swagger UI endpoint (`/api/v1/admin/docs/users`) when user management is enabled.

- **User CRUD** - Create, read, update, and delete admin users
- **Password Management** - Set and update user passwords

## Security Considerations

!!! warning "Production Deployments"
    
    When deploying the Admin API in production:
    
    - **Use HTTPS** - Always serve the Admin API over TLS to protect credentials and data in transit
    - **Network Isolation** - Consider running the Admin API on a separate port accessible only from trusted networks
    - **Strong Credentials** - Use strong, unique passwords for admin users
    - **Firewall Rules** - Restrict access to the Admin API endpoints using firewall rules

For configuration options including separate port binding and password hashing settings, see 
[Admin API Configuration](../config/api.md).
