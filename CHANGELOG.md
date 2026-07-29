## LightHouse 0.22.0

This is a major release. Federation endpoints and trust anchors have moved from the config file into the database and are now managed at runtime via the Admin API. A new `lhsetup` tool, subordinate JWKS refreshing, post-quantum signing, and a zerolog-based logging stack round out the headline changes.

> **You must be on at least 0.20.0 to migrate to 0.22.x.** See the [migration guide](docs/migration/0.22.md). Back up your config file and database before migrating.

### Breaking Changes

#### Trust Anchor Repository
Trust anchors are no longer configured in the config file. They are now stored once in the database as a single source of truth, referenced by entity ID, and resolved live at request time. All former config locations (`federation_data.trust_anchors`, `endpoints.auth.trust_anchors`, per-endpoint `auth_trust_anchors`, entity checker `trust_anchors`, `allowed_trust_anchors`) are consolidated into the `trust_anchors` table. Manage them via `GET/POST/PUT/DELETE /api/v1/admin/trust-anchors`. Trust anchors can optionally auto-refresh their JWKS (`enable_jwks_update`, `key_poll_interval`).

#### Dynamic Federation Endpoints
Federation endpoints (fetch, list, resolve, trust mark, entity collection, enroll, etc.) are now database-managed. Paths, URLs, auth settings, and type-specific config are stored in the `federation_endpoints` table and can be changed at runtime via the Admin API without restarting. Endpoints are served through a single catch-all dispatcher backed by an in-memory registry that reloads atomically on Admin-API changes (with entity-configuration cache invalidation). The `endpoints` section has been removed from the config file; only JTI storage settings remain, now under `storage.endpoint_auth`.

#### Prefork Mode Removed
`server.prefork` and `LH_SERVER_PREFORK` are removed. LightHouse always runs as a single process (prefork caused inconsistencies with in-memory caches, the trust anchor repository, and the endpoint registry). Existing `prefork: true` entries are silently ignored — remove them.

#### Environment Variable Rename
JTI env vars renamed:
- `LH_ENDPOINTS_AUTH_JTI_BACKEND` → `LH_STORAGE_ENDPOINT_AUTH_JTI_BACKEND`
- `LH_ENDPOINTS_AUTH_JTI_CLEANUP_INTERVAL` → `LH_STORAGE_ENDPOINT_AUTH_JTI_CLEANUP_INTERVAL`

### New Features

#### Subordinate JWKS Refreshing
Three complementary mechanisms keep a subordinate's JWKS current after key rotation:
- **Periodic refreshing** — LightHouse polls the subordinate's entity configuration on a schedule (opt-in per subordinate via `enable_jwks_update` / `jwks_poll_interval`).
- **JWKS Update Trigger endpoint** (`jwks_update_trigger`) — a subordinate POSTs to request LightHouse re-fetches its JWKS.
- **JWKS Update endpoint** (`jwks_update`) — a subordinate POSTs a signed JWK Set (`application/jwk-set+jwt`) with its new keys directly.

Each mechanism records events (`jwks_refreshed`, `jwks_update_triggered`, `jwks_updated`) in the subordinate's history.

#### Expired Key Filtering & Statement Expiration Capping
Subordinate statements now omit keys whose `exp` has passed (publish-only; stored JWKS retains all keys for 
verification during rotation). The statement's own `exp` is capped to the latest published key expiration so it never outlives advertised keys.

#### Subordinate Statement Caching
Fetch-endpoint subordinate statements are now cached, keyed by entity ID, with key-based invalidation. Admin routes that modify subordinate state resolve the DB ID → entity ID to invalidate the precise cache entry.

#### Post-Quantum & Hybrid Signing Algorithms
Added support for ML-DSA (FIPS 204) and composite PQC-hybrid signatures. Also added `Ed448`, `ES256K`, and curve-specific `Ed25519` (the polymorphic `EdDSA` form is deprecated).

#### Key Announcement Lead Time
Two new key-rotation options control how far in advance a new key is published before becoming active: 
`key_announcement_lead_time` (fixed) and `key_announcement_lead_time_ec_multiplier` (multiplier on EC lifetime; takes precedence).
Default is `max(5 × EC lifetime, 24h)`.

#### Interactive Setup Tool (`lhsetup`)
A new `lhsetup` binary interactively prompts for all DB-managed configuration and writes it to the database — an alternative to `lhmigrate config2db` (automated) or the Admin API (runtime). Includes a `show` command for read-only inspection. Shipped in the Docker image.

#### Stats Aggregator with Backfill
Daily statistics aggregation now backfills missing days on startup (within the detailed-log retention window) so `/stats/daily` is populated immediately rather than only after the next 2 AM tick. Safe to run repeatedly; existing rows and already-purged data are never overwritten.

#### New `except` metadata policy operator
The [`except` metadata policy operator](https://zachmann.github.io/federation-except-operator/main.html) is now supported.

### Improvements

- **Logging migrated from logrus to zerolog.** New `logging.internal.stderr_format` (`console`/`json`) and `dir_format` (`json`/`console`) let stderr and file outputs use different formats independently. The dead "smart logger" config was removed.
- **PATCH support** for partial subordinate updates (preferred over PUT for partial changes).

### Bug Fixes

- **Soft-deleted subordinates are reactivated** when updated via the Admin API (mirrors Add behavior).
- **Stats middleware fix:** response status is now logged correctly for error codes set by Fiber (e.g. 404).

### Dependencies / Build

- Go **1.26** (with `GOEXPERIMENT=jsonv2` in the Docker build).
- `lestrrat-go/jwx` upgraded v3 → **v4**.
- `go-oidfed/lib` upgraded to 0.11.0.
- `badger/v4` 4.9.2 → 4.9.4, `fiber/v2` 2.52.13 → 2.52.14, `go-redis/v9` 9.20.1 → 9.21.0, plus `golang.org/x/crypto`, `gorm` dependency bumps.

### Migration

Run `lhmigrate config2db --only=trust_anchors,endpoints --update-config` to migrate trust anchors and endpoints to the database (use `--dry-run -v` to preview). Update any JTI environment variables manually. See `docs/migration/0.22.md` for full details.

---

## LightHouse 0.21.0

#### Features
- Added new KMS backend that stores private keys in the database
- Federation Endpoint can now support client authentication (with POST)


**Full Changelog**: https://github.com/go-oidfed/lighthouse/compare/v0.20.4...v0.21.0

## LightHouse 0.20.4

#### What's Changed
- Added support for TLS on admin api with separate port
- Fix the endpoints logging on the statistics
- fixed a problem on the admin API when re-adding a subordinate that was previously deleted.

## LightHouse 0.20.3

- Fixed problems with mysql db indices
- Fixed a bug with unmarshalling custom entity type metadata policies

## LightHouse 0.20.2

#### Overview

This release includes improvements to the admin API, database schema fixes, soft-delete handling enhancements, and extensive test coverage additions.

---

#### Enhancements

##### Admin API Improvements

- **Additional Claims Format**: Changed `additional_claims` response format from array of objects to simple key-value object for better usability
  - Before: `[{"claim": "name", "value": "org", "crit": false}]`
  - After: `{"name": "org"}`
  - Affected endpoints: All subordinate GET endpoints

- **Conflict Detection**: Added proper 409 Conflict responses for duplicate resource creation attempts
  - Subordinate creation now returns 409 when entity_id already exists
  - OpenAPI spec updated to document 409 responses

---

#### 🐛 Bug Fixes

##### JWKS Requirement for Pending Subordinates
**Issue**: Creating subordinates with status=pending could fail with foreign key constraint error when no JWKS was provided.

##### Soft-Delete Reactivation Issues
**Issue**: Re-creating deleted resources after soft-delete caused nil pointer dereferences or unexpected behavior across multiple entity types.

**Affected Resources**:
- Subordinates
- Trust Mark Types
- Trust Mark Owners
- Trust Mark Issuers
- Trust Mark Specs
- Authority Hints
- Entity Configuration Additional Claims
- Published Trust Marks

#### 📊 Test Coverage Improvements

* Extended test coverage for admin api endpoints

---

#### 📝 API Changes

##### OpenAPI Specification Updates

###### Added Response Codes
- `409 Conflict` for POST `/api/v1/admin/subordinates`
  - Triggered when entity_id already exists
  - Response: `{"error": "invalid_request", "error_description": "subordinate with entity_id X already exists"}`

###### Schema Changes
- `AdditionalClaims`: Changed from array to object type
  ```yaml
  # Before
  type: array
  items:
    $ref: '#/components/schemas/AdditionalClaim'
  
  # After
  type: object
  additionalProperties: true
  example:
    organization_name: Example Organization
    homepage_uri: https://example.org
  ```

##### Backwards Compatibility

⚠️ **Breaking Change**: The `additional_claims` field format change may affect clients expecting array format.

**Migration Path**:
- Clients should update to expect object format: `{"claim_name": "value"}`
- Old array format will no longer be returned
- This aligns with OIDC Federation specification expectations

---


**Full Changelog**: https://github.com/go-oidfed/lighthouse/compare/v0.20.1...v0.20.2

## LightHouse 0.20.1

#### Bugfixes
* Fixed a bug in the admin api when PATCHing TrustMarkSpecs json fields.
* Fixed a bug where the `essential=true` metadata policy would not correctly produce a metadata policy error on array claims (e.g. `contacts`) when the claim was not provided.
#### Dependencies
* bump github.com/go-oidfed/lib from 0.10.4 to 0.10.9
* bump github.com/lestrrat-go/jwx/v3 from 3.1.0 to 3.1.1
* bump golang.org/x/crypto from 0.50.0 to 0.51.0
* bump github.com/go-oidfed/lib from 0.10.5 to 0.10.6


**Full Changelog**: https://github.com/go-oidfed/lighthouse/compare/v0.20.0...v0.20.1

## LightHouse 0.20.0

LightHouse 0.20.0 is a major release introducing a comprehensive Admin API, database-backed storage with GORM,
and a migration tool for upgrading from v0.8.x. It has breaking changes and major changes. Therefore, a prerelease was published 7 weeks ago. No major complaints where submitted since then.

#### Highlights        

- **Admin API**: Full management API for dynamic federation configuration
- **Database Storage**: GORM-based storage with SQLite, MySQL, and PostgreSQL support
- **Migration Tool**: `lhmigrate` CLI for seamless upgrades from v0.8.x
- **Trust Mark Management**: Issuance tracking, status endpoints, and revocation
- **Statistics System**: Collection and aggregation of usage metrics
- **Prefork Mode**: Improved performance with multi-process support (#36)

#### ⚠️ Breaking Changes

- Configuration file format has changed.
  - There are renamed and dropped options.
  - [**Migration needed!**](https://go-oidfed.github.io/lighthouse/migration/)

#### ✨ Features

##### Admin API

- User management and authentication
- Full CRUD for subordinate entities (metadata, JWKS, policies, constraints, additional claims)
- Subordinate status management and statement preview
- Trust mark type management with ownership and issuer aggregation
- Trust mark issuance management via API
- Entity configuration trust marks management
- Event logging and subordinate history API
- Key management and KMS integration
- Global metadata policy crit management
- Actor extraction for event recording (configurable via `ActorHeader` and `ActorSource`)
- ...

##### Trust Marks

- Issued trust marks tracking with status endpoint
- Trust mark revocation support
- Caching for issued trust marks (configurable `CacheTTL`)
- Enhanced trust marked entities listing with instance-level validation

##### Migration Tool (`lhmigrate`)

https://go-oidfed.github.io/lighthouse/migration/

- `all` subcommand for sequential migration of all steps
- Config transformation with `--update-config` for auto-cleanup
- Database-backed public key storage migration
- Trust marks migration
- Metadata policies migration (file-based with `MetadataPolicyFile`)
- Extra entity configuration claims migration
- Multiple database driver support (`--db-type`, `--db-dsn`, `--db-dir`)
- Included in Docker image

##### Server & Configuration

- Prefork mode support (closes #36)
- Environment variable support for configuration
- Cache invalidation middleware for entity configuration and subordinate statements

##### Storage

- Database-backed public key storage
- Transactional support for multi-backend operations
- Statistics storage and aggregation system

#### 🐛 Bug Fixes

- **Server**: ProxyHeader is now only set when TrustedProxies are configured, 
  fixing empty `c.IP()` returns for direct connections
- Introduced new bugs to fix in future releases.

#### 📚 Documentation

- Updated deployment guides (Caddy, Docker, PostgreSQL setup)
- Admin API documentation with configuration options
- Migration guide with detailed CLI examples and workflows
- Entity checkers and trust marks feature documentation
- CLI and deployment documentation

## LightHouse 0.8.4

Upgraded various dependencies

## LightHouse 0.8.3

#### What's Changed
- Fixed the expiration time of subordinate statements
- Add  a startup banner including logo and version
- Add support to disable cache (testing only!)
- Add support to set a maximum lifetime for cache entries.
- Bumped several dependencies

## LightHouse 0.8.2

#### What's Changed
* Bump github.com/redis/go-redis/v9 from 9.16.0 to 9.17.0 by @dependabot[bot] in https://github.com/go-oidfed/lighthouse/pull/35
* Bump lib to 0.8.4


**Full Changelog**: https://github.com/go-oidfed/lighthouse/compare/v0.8.1...v0.8.2

## LightHouse 0.8.1

- Hopefully fixed a bug that could lead to an invalid resolve response.
- Redis cache can now be configured with username and password, as well as db index.
- Updated the go-oidfed lib to 0.8.3, this includes:
  - Better logging
  - Fixed a bug in the TrustResolver that could prevent caching

## LightHouse 0.8.0

#### Features
- Add support for setting redis as cache
- Entity Collection Endpoint now supports limiting the allowed trust anchors
- Entity Collection Endpoint now supports periodic collection runs
- Entity Collection Endpoint now support pagination
- Resolve Endpoint now supports limiting the allowed trust anchors
- Resolve Endpoint now supports pre-resolving all entities

#### Others
- Added multi-platform docker builds

#### Dependencies
- Bump github.com/spf13/cobra from 1.9.1 to 1.10.1
- Bump github.com/lestrrat-go/jwx/v3 from 3.0.10 to 3.0.12
- Bump github.com/go-oidfed/lib from 0.7.1 to 0.8.1

## LightHouse 0.7.0

#### Endpoints
##### Entity Collection
- Added multi language support

#### Enhancements
- Docker container now stops gracefully
- Added support to log remote client IP instead of proxy IP if behind a proxy
- Added config option to disable automatic generation of signing keys
- Verify Entity Configuration signature on enrollment
- Added option to `lhcli` to pass the jwks of an entity that should be used to verify the Entity Configuration
- Removed error log messages when loading the config file.

#### Bug Fixes
- Fixed a bug in the badger storage backend, that for example did not allow to enroll entities

## LightHouse 0.6.0

#### Important Config Changes
- `signing.key_file` is no longer support. `signing.key_dir` must be used instead. To keep the existing signing key it must be renamed. See https://go-oidfed.github.io/lighthouse/config/signing/#key_file

#### What's Changed
- Added support for signing key rotation and historical keys endpoint
- Updated entity collection endpoint

## LightHouse 0.5.1

- Searches on more locations for `config.yaml`

## LightHouse 0.5.0

This is the first release of LightHouse. LightHouse has moved from the zachmann/go-oidfed example TA into its own repo.

#### Configuration Changes

Compared to the example TA the configuration format was reworked.

###### Bad News
I broke the config file format; you need to update your configuration file

###### Good News
I can help you with that. The attached python script eases transition. While I cannot guarantee that everything will be perfect, it should help a lot - and in most cases the script should do the work and you are good to go - hopefully.

Just download the attached script and run it on your old config file like this:
```shell
python migrate-config.py config.yaml new-config.yaml
```

For more information about the configuration, refer to the [documentation](https://go-oidfed.github.io/lighthouse/config/)

#### What is New
- Renamed `trust_mark_id` to `trust_mark_type` per draft 43
- You can now choose another signing algorithm than ES512
- You can add much more metadata to the entity configuration, e.g. the new UI related claims
- We have nice documentation:
