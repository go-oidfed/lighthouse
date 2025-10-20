---
icon: material/routes
---


The following endpoints are available:

| Endpoint                      | Config Parameter     | Description                                                                                                                                                                                        |
|-------------------------------|----------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Federation Config             | n/a                  | Always enabled. The federation endpoint where the entity configuration is published.                                                                                                               |
| Fetch                         | `fetch`              | Federation Subordinate Fetch Endpoint per Spec Section 8.1                                                                                                                                         |
| Subordinate Listing           | `list`               | Federation Subordinate Listing Endpoint per Spec Section 8.2                                                                                                                                       |
| Resolve                       | `resolve`            | Resolve Endpoint per Spec Section 8.3                                                                                                                                                              |
| Trust Mark Status             | `trust_mark_status`  | Trust Mark Status Endpoint per Spec Section 8.4                                                                                                                                                    |
| Trust Marked Entities Listing | `trust_mark_listing` | Trust Marked Entities Listing Endpoint per Spec Section 8.5                                                                                                                                        |
| Trust Mark                    | `trust_mark`         | Trust Mark Endpoint per Spec Section 8.6                                                                                                                                                           |
| Federation Historical Keys    | `historical_keys`    | Historical Keys Endpoint per Spec Section 8.7; only usable with automatic key rollover                                                                                                             |
| Enrollment                    | `enroll`             | An endpoint where entities can automatically enroll into the federation. For details see #enrolling-entities                                                                                       |
| Request Enrollment            | `enroll_request`     | An endpoint where entities can request enrollment into the federation. An federation administrator then can check and approve the request. The request is analog to the enroll request             |
| Trust Mark Request            | `trust_mark_request` | An endpoint where entities can request to be entitled for a trust mark. A federation administrator then can check and approve the request. The request is analog to the trust mark request         |
| Entity Collection             | `entity_collection`  | An endpoint to query a filterable list of all entities in a federation. Per [Entity Collection Endpoint Extension Draft](https://zachmann.github.io/openid-federation-entity-collection/main.html) |

## Enrolling Entities

LightHouse implements a custom enrollment / onboarding endpoint which can be 
configured in the config file.
This endpoint is used to easily add entities to the federation. Entities can
also be manually added to the database (or with a simple command line
application).

The enrollment endpoint can also be guarded by so-called [Entity Checks](entity_checks.md).
If the enroll endpoint is enabled, but no checks defined, all entities can 
enroll (obviously not recommended outside a proof-of-concept).

### Enrollment Request

To enroll, the entity sends a `GET` request to the enroll endpoint with the 
following request parameter:

| Parameter     | Necessity   | Description     |
|---------------|-------------|-----------------|
| `sub`         | REQUIRED    | Its entity id   |
| `entity_type` | RECOMMENDED | Its entity type |

`entity_type` can be provided multiple times to pass multiple entity types.

LightHouse will query the entity's federation endpoint for its Entity 
Configuration and obtain the jwks from there and (if configured) performs the entity checks.
