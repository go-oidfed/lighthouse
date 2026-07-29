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
