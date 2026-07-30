---
icon: material/database-cog
---

# Database-Managed Configuration

Many federation-specific settings are stored in the database rather than the
config file. They can be changed at runtime without restarting LightHouse.

## Management Tools

Three tools manage database-backed configuration:

| Tool | Use case |
|------|----------|
| [Admin API](../../features/admin_api.md) | Runtime management via HTTP — create, update, delete individual settings. |
| [`lhsetup`](../../deployment/lhsetup.md) | Interactive CLI wizard — prompts for all DB-managed values; ideal for initial setup. |
| [`lhmigrate config2db`](../../migration/0.22.md) | Non-interactive migration from an existing config file to the database. |

## Configurable Sections

| Section | Description |
|---------|-------------|
| [`signing`](signing.md) | Signing algorithm, RSA key length, key rotation (incl. key announcement lead time) |
| [`entity-configuration`](entity-configuration.md) | Entity configuration & statement lifetimes, federation metadata, constraints, metadata policies, authority hints, extra entity config claims |
| [`trust-marks`](trust-marks.md) | Published trust marks, trust mark specs, trust mark issuers, trust mark owners |
| [`trust-anchors`](trust-anchors.md) | Trust anchor repository (entity IDs, JWKS, automatic JWKS refreshing) |
| [`federation-endpoints`](federation-endpoints.md) | Federation endpoint paths, URLs, auth settings, and type-specific configuration |
| [`subordinates`](subordinates.md) | Per-subordinate JWKS refresh settings |

Use `lhsetup --only=<section>` or `--skip=<section>` to select specific
sections. See the [lhsetup documentation](../../deployment/lhsetup.md) for the
full list of section names.
