---
icon: material/cog
---

# Config

LightHouse configuration is split into two categories:

- **Static configuration** — set in `config.yaml` and/or environment variables;
  requires a restart to change. See [Static Configuration](static/index.md).
- **Database-managed configuration** — stored in the database, changed at
  runtime via the Admin API, `lhsetup`, or `lhmigrate config2db`; no restart
  required. See [Database-Managed Configuration](db/index.md).

!!! danger "Both are needed!"

    Those are not alternative approaches! Both static and database-managed configuration is needed.

## Configuration Sections

### Static Configuration (Config File & Environment Variables)

<div class="grid cards" markdown>

- [:material-file-cog: Static Configuration](static/index.md)
- [:material-server-network: Server](static/server.md)
- [:material-script-text: Logging](static/logging.md)
- [:material-database: Storage](static/storage.md)
- [:material-database-clock: Cache](static/cache.md)
- [:material-signature-freehand: Signing](static/signing.md)
- [:material-api: Admin API](static/api.md)
- [:material-chart-line: Statistics](static/stats.md)

</div>

### Database-Managed Configuration

<div class="grid cards" markdown>

- [:material-database-cog: Database-Managed Config](db/index.md)
- [:material-signature-freehand: Signing](db/signing.md)
- [:material-openid: Entity Configuration](db/entity-configuration.md)
- [:material-shield-check-outline: Trust Marks](db/trust-marks.md)
- [:material-shield-check: Trust Anchors](db/trust-anchors.md)
- [:material-routes: Federation Endpoints](db/federation-endpoints.md)
- [:material-account-group: Subordinates](db/subordinates.md)

</div>

!!! info "DB-Managed Configuration"

    Federation endpoints, trust anchors, metadata, authority hints, trust marks,
    signing algorithm, key rotation, and other runtime options are stored in the
    database. Use [`lhsetup`](../deployment/lhsetup.md) for interactive setup,
    [`lhmigrate config2db`](../migration/0.22.md) for non-interactive migration
    from a config file, or the [Admin API](../features/admin_api.md) for runtime
    management.

## :fontawesome-solid-stopwatch: Time Duration Configuration Options
Some configuration option take a duration, e.g. the lifetime of entity 
statements or the entity configuration.

There are different options how to pass a duration in the config file:

- **Number**: If only a number is given, this is the number of seconds.
- **String**: The duration can also be given as a string which supports 
  different units.

For a duration string the following units are supported and multiple units 
can be used in a single string:

| Symbol | Unit        | Comment            |
|--------|-------------|--------------------|
| `y`    | Year        | = 365 days         |
| `w`    | Week        | = 7 days           |
| `d`    | Day         | = 24 hours         |
| `h`    | Hour        |                    |
| `m`    | Minute      |                    |
| `s`    | Second      |                    |
| `ms`   | Millisecond | SHOULD NOT be used |
| `µs`   | Microsecond | SHOULD NOT be used |
| `ns`   | Nanosecond  | SHOULD NOT be used |


!!! Example "Examples"
    ```
    1y
    2w6d
    20d
    1h30m
    ```
