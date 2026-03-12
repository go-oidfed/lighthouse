---
icon: material/police-badge
---

# Entity Checks
With the **Entity Checks** mechanism checks on an entity can be defined. 
One can define their own Entity Checks by implementing the `EntityChecker` 
interface and registering it through the `RegisterEntityChecker` function before
loading the config file. 

The following Entity Checks are already implemented and supported out of the 
box by LightHouse:

- [`none`](#none): Always allows access (no checks performed)
- [`trust_mark`](#trust-mark): Checks if the entity advertises a Trust Mark and verifies that it is valid
- [`trust_path`](#trust-path): Checks if there is a valid trust path from the entity to the defined Trust Anchor
- [`authority_hints`](#authority-hints): Checks if the entity's published
  `authority_hints` contain the defined Entity ID
- [`entity_id`](#entity-ids): Checks if the entity's `entity_id` is one of the 
  defined ones
- [`multiple_and`](#multiple): Used to combine multiple `EntityChecker` 
  using AND
- [`multiple_or`](#multiple): Used to combine multiple `EntityChecker` 
  using OR
- [`db_list`](#db-list): Checks if the entity is in the database with active status (trust mark issuance only)
- [`http_list`](#http-list): Fetches a list of allowed entity IDs from an HTTP endpoint
- [`http_list_jwt`](#http-list-jwt): Fetches a signed JWT containing allowed entity IDs from an HTTP endpoint

In the following we describe in more details how to configure the different
Entity Checkers:

## None
No additional configuration applicable.

!!! file "Example"

    ```yaml
    checker:
      type: none
    ```

## Trust Mark
For a Trust Mark Entity Checker one must configure the Trust Mark Type of the
Trust Mark that should be checked. Additionally, one must provide either Trust
Anchors or the Trust Mark Issuer's jwks and in the case of delegation
information about the Trust Mark Owner.

### Config Parameters

| Claim                    | Necessity                                                       | Description                                                  |
|--------------------------|-----------------------------------------------------------------|--------------------------------------------------------------|
| `trust_mark_type`        | REQUIRED                                                        | The Trust Mark Type of the Trust Mark to check               |
| `trust_anchors`          | REQUIRED unless `trust_mark_issuer_jwks` is given               | A list of Trust Anchors used to verify the Trust Mark issuer |
| `trust_mark_issuer_jwks` | REQUIRED if `trust_anchors` is not given                        | The jwks of the Trust Mark Issuer                            |
| `trust_mark_owner`       | REQUIRED if `trust_anchors` is not given and delegation is used | Information about the Trust Mark Owner                       |

The `trust_anchors` claim is a list where each element can have the following
parameters:

| Claim       | Necessity | Description                                                                      |
|-------------|-----------|----------------------------------------------------------------------------------|
| `entity_id` | REQUIRED  | The Entity ID of the Trust Anchor                                                |
| `jwks`      | OPTIONAL  | The Trust Anchor's jwks; if omitted it is obtained from its Entity Configuration |

The `trust_mark_owner` claim has the following parameters:

| Claim       | Necessity | Description                           |
|-------------|-----------|---------------------------------------|
| `entity_id` | REQUIRED  | The Entity ID of the Trust Mark Owner |
| `jwks`      | REQUIRED  | The Trust Mark Owner's jwks           |


### Examples


=== ":material-file-code: Using Trust Anchor"

    ```yaml
    checker:
      type: trust_mark
      config:
        trust_mark_type: https://tm.example.org
        trust_anchors:
          - entity_id: https://ta.example.org
    ```

=== ":material-file-code: Using Trust Mark Issuer JWKS"

    ```yaml
    checker:
      type: trust_mark
      config:
        trust_mark_type: https://tm.example.org
        trust_mark_issuer_jwks: {"keys":[{"alg":"ES512","crv":"P-521","kid":"E6XirVKtuO2_76Ly8Lw1cS_W4FUfw_lx5M_z33aMO-I","kty":"EC","use":"sig","x":"AbZpRmHJVpqqJ2q4bFMPto5jVhReNe0toBHWm0y-AhdpqYIqLA-J3ICr_I42BgmC4pG9lQE4qU8mJjkX1I__PDK8","y":"AFl9aVDzsUJPbyxDe96FuLWJNYNOo68WcljWEXJ0QzsFaTDUtykNe1lf3UoOXQWnvNQ1eD2iyWTef1gRR9A6HOSI"}]}
    ```

=== ":material-file-code: Using Trust Mark Issuer JWKS and delegation"

    ```yaml
    checker:
      type: trust_mark
      config:
        trust_mark_type: https://tm.example.org
        trust_mark_issuer_jwks: {"keys":[{"alg":"ES512","crv":"P-521","kid":"E6XirVKtuO2_76Ly8Lw1cS_W4FUfw_lx5M_z33aMO-I","kty":"EC","use":"sig","x":"AbZpRmHJVpqqJ2q4bFMPto5jVhReNe0toBHWm0y-AhdpqYIqLA-J3ICr_I42BgmC4pG9lQE4qU8mJjkX1I__PDK8","y":"AFl9aVDzsUJPbyxDe96FuLWJNYNOo68WcljWEXJ0QzsFaTDUtykNe1lf3UoOXQWnvNQ1eD2iyWTef1gRR9A6HOSI"}]}
        trust_mark_owner:
          entity_id: https://ta.example.org
          jwks: {"keys":[{"alg":"ES512","crv":"P-521","kid":"gChx94HqIDTscqMzxDps6degt2j_Z7OrDsx0Fc24rKA","kty":"EC","use":"sig","x":"AAyVRMA84JsAtJ9z3qKVzgBN1DL8lDIrHRRYtnYiSkfe-i0V7W21QJ_VBBRF3kWFEYadRL9z4yJC7gYvsojF6p8C","y":"AYx1JCtCfrvNR8x8KibI2mQJKAsszjslfd8WlTha8lxtvncpg5c-UxjJgpCYRo3jwdvxUCa6LKHu0TzbUhKfFK8f"}]}
    ```

## Trust Path

For a trust path Entity Checker one must configure the Trust Anchors that should
be used to verify that there is an existing trust path to one of these Trust
Anchors.

### Config Parameters

| Claim           | Necessity | Description                                           |
|-----------------|-----------|-------------------------------------------------------|
| `trust_anchors` | REQUIRED  | A list of Trust Anchors used to verify the trust path |

The `trust_anchors` claim is a list where each element can have the following
parameters:

| Claim       | Necessity | Description                                                                     |
|-------------|-----------|---------------------------------------------------------------------------------|
| `entity_id` | REQUIRED  | The Entity ID of the Trust Anchor                                               |
| `jwks`      | OPTIONAL  | The Trust Anchors jwks; if omitted it is obtained from its Entity Configuration |


### Example

!!! file "Example"

    ```yaml
    checker:
      type: trust_path
      config:
        trust_anchors:
          - entity_id: https://ta.example.org
    ```

## Authority Hints

For an Authority Hints Entity Checker one must configure the Entity ID that
should be present in the authority hints.

### Config Parameters

| Claim       | Necessity | Description                                                          |
|-------------|-----------|----------------------------------------------------------------------|
| `entity_id` | REQUIRED  | The Entity ID that should be present in the entity's authority hints |

### Example

!!! file "Example"

    ```yaml
    checker:
      type: authority_hints
      config:
        entity_id: https://ia.example.org
    ```

## Entity IDs

For an Entity ID Entity Checker one must configure the Entity ID(s) that
are allowed.

### Config Parameters

| Claim        | Necessity | Description                  |
|--------------|-----------|------------------------------|
| `entity_ids` | REQUIRED  | A list of allowed Entity IDs |

### Example

!!! file "Example"

    ```yaml
    checker:
      type: entity_id
      config:
        entity_ids: 
          - https://op1.example.org
          - https://op2.example.org
    ```

## Multiple
To combine multiple Entity Checkers (either with AND or OR) one must provide all
Entity Checkers:

!!! file "Nested Example"

    ```yaml
    checker:
      type: multiple_and
      config:
        - type: trust_path
          config:
            trust_anchors:
              - entity_id: https://ta.example.org
        - type: multiple_or
          config:
            - type: trust_mark
              config: 
                trust_mark_type: https://tm.example.com
                trust_anchors:
                  - entity_id: https://ta.example.com
            - type: trust_mark
              config: 
                trust_mark_type: https://tm.example.org
                trust_anchors:
                  - entity_id: https://ta.example.org
    ```

## DB List

The DB List Entity Checker verifies that an entity is in the `TrustMarkSubject`
database table with an `active` status for the current trust mark type.

!!! warning "Trust Mark Issuance Only"

    This checker is a **contextual checker** that requires runtime context
    provided by the trust mark endpoint. It can only be used for trust mark
    issuance eligibility checks, not for enrollment or other purposes.

The checker returns different HTTP status codes based on the subject's status:

| Subject Status | Result | HTTP Code |
|----------------|--------|-----------|
| `active` | Pass | - |
| `blocked` | Fail | 403 Forbidden |
| `pending` | Fail | 202 Accepted |
| `inactive` | Fail | 404 Not Found |

### Config Parameters

No configuration parameters are required. The trust mark type and storage
backend are provided automatically by the trust mark endpoint.

### Example

!!! file "Example"

    ```yaml
    checker:
      type: db_list
    ```

## HTTP List

The HTTP List Entity Checker fetches a JSON array of entity IDs from an HTTP
endpoint and checks if the requesting entity is in the list. This checker can
be used for both enrollment and trust mark issuance.

### Config Parameters

| Parameter   | Necessity | Default | Description                                      |
|-------------|-----------|---------|--------------------------------------------------|
| `url`       | REQUIRED  | -       | The URL to fetch the entity list from            |
| `method`    | OPTIONAL  | `GET`   | HTTP method to use (`GET` or `POST`)             |
| `headers`   | OPTIONAL  | -       | Additional HTTP headers as key-value pairs       |
| `timeout`   | OPTIONAL  | `30`    | Request timeout in seconds                       |
| `cache_ttl` | OPTIONAL  | `60`    | How long to cache the fetched list (in seconds)  |

The endpoint must return a JSON array of entity ID strings:

```json
["https://entity1.example.org", "https://entity2.example.org"]
```

### Examples

=== ":material-file-code: Basic Usage"

    ```yaml
    checker:
      type: http_list
      config:
        url: https://registry.example.org/allowed-entities
    ```

=== ":material-file-code: With Headers and Caching"

    ```yaml
    checker:
      type: http_list
      config:
        url: https://registry.example.org/api/entities
        method: GET
        headers:
          Authorization: Bearer secret-token
          Accept: application/json
        timeout: 10
        cache_ttl: 300
    ```

## HTTP List JWT

The HTTP List JWT Entity Checker fetches a signed JWT containing a list of
entity IDs from an HTTP endpoint. The JWT signature is verified before
extracting the entity list. This checker can be used for both enrollment and
trust mark issuance.

This checker supports two verification modes:

- **JWKS mode**: Verify the JWT signature using a pre-configured JWKS
- **Trust Anchor mode**: Verify by building a trust chain from the JWT issuer
  to configured trust anchors

### Config Parameters

| Parameter      | Necessity | Default    | Description                                         |
|----------------|-----------|------------|-----------------------------------------------------|
| `url`          | REQUIRED  | -          | The URL to fetch the signed JWT from                |
| `method`       | OPTIONAL  | `GET`      | HTTP method to use (`GET` or `POST`)                |
| `headers`      | OPTIONAL  | -          | Additional HTTP headers as key-value pairs          |
| `timeout`      | OPTIONAL  | `30`       | Request timeout in seconds                          |
| `cache_ttl`    | OPTIONAL  | `60`       | How long to cache the fetched list (in seconds)     |
| `list_claim`   | OPTIONAL  | `entities` | The JWT claim containing the entity ID array        |
| `verification` | REQUIRED  | -          | Verification configuration (see below)              |

### Verification Configuration

The `verification` parameter configures how the JWT signature is verified:

| Parameter       | Necessity                          | Description                                  |
|-----------------|------------------------------------|----------------------------------------------|
| `mode`          | REQUIRED                           | Either `jwks` or `trust_anchor`              |
| `jwks`          | REQUIRED if mode is `jwks`         | The JWKS to verify the JWT signature         |
| `trust_anchors` | REQUIRED if mode is `trust_anchor` | List of trust anchors for chain verification |

When using `trust_anchor` mode, the checker:

1. Parses the JWT to extract the issuer claim
2. Attempts to build a trust path from the issuer to one of the configured trust anchors
3. Fetches the issuer's entity configuration to obtain their signing keys
4. Verifies the JWT signature using those keys

### Examples

=== ":material-file-code: Using JWKS Verification"

    ```yaml
    checker:
      type: http_list_jwt
      config:
        url: https://registry.example.org/entities.jwt
        list_claim: entities
        verification:
          mode: jwks
          jwks: {"keys":[{"alg":"ES256","crv":"P-256","kid":"key1","kty":"EC","use":"sig","x":"...","y":"..."}]}
    ```

=== ":material-file-code: Using Trust Anchor Verification"

    ```yaml
    checker:
      type: http_list_jwt
      config:
        url: https://registry.example.org/entities.jwt
        list_claim: allowed_entities
        cache_ttl: 600
        verification:
          mode: trust_anchor
          trust_anchors:
            - entity_id: https://ta.example.org
    ```

=== ":material-file-code: With Custom Headers"

    ```yaml
    checker:
      type: http_list_jwt
      config:
        url: https://registry.example.org/api/entities.jwt
        method: POST
        headers:
          Authorization: Bearer api-key
        timeout: 15
        verification:
          mode: trust_anchor
          trust_anchors:
            - entity_id: https://ta1.example.org
            - entity_id: https://ta2.example.org
    ```

