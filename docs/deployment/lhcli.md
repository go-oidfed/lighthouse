---
icon: material/console
---

# LightHouse CLI (lhcli)

The `lhcli` command-line tool provides administrative access to your LightHouse 
instance directly from the terminal. It connects to the same database as your 
running LightHouse server, allowing you to manage subordinates, trust marks, 
and statistics without using the Admin API.

## When to Use lhcli

We recommend to use the [admin http API](../features/admin_api.md) for most 
administration tasks, since it offers more features. However, `lhcli` is useful 
for quick and easy management tasks or if you prefer a command line interface.

- **Scripting and automation** - Integrate LightHouse management into shell scripts and CI/CD pipelines
- **Server administration** - Quick operations when SSH'd into your server
- **Batch operations** - Process multiple requests interactively
- **Statistics analysis** - View and export statistics data
- **Offline management** - Manage data even when the HTTP server is not running

## Installation

The `lhcli` binary is included in LightHouse docker containers alongside the 
main `lighthouse` binary. You can also build it from source:

```bash
go build -o lhcli ./cmd/lhcli
```

## Usage

```bash
lhcli [command] [subcommand] [flags]
```

### Global Flags

| Flag       | Short | Default       | Description                               |
|------------|-------|---------------|-------------------------------------------|
| `--config` | `-c`  | `config.yaml` | Path to the LightHouse configuration file |

The CLI uses the same configuration file as the LightHouse server to connect 
to the database.

## Commands Overview

| Command        | Description                         |
|----------------|-------------------------------------|
| `subordinates` | Manage subordinate entities         |
| `trustmarks`   | Manage trust mark entitlements      |
| `stats`        | View and manage statistics          |
| `delegation`   | Generate trust mark delegation JWTs |

---

## Subordinates

Manage subordinate entities in your federation.

### subordinates add

Add a new subordinate entity to the federation.

```bash
lhcli subordinates add <entity_id> [flags]
```

**Arguments:**

| Argument    | Description                                           |
|-------------|-------------------------------------------------------|
| `entity_id` | The entity identifier (URL) of the subordinate to add |

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--entity_type` | `-t` | Entity type(s) to assign (can be specified multiple times) |
| `--jwks` | `-k` | Path to a JWKS file containing the entity's public keys |

**Behavior:**

1. Fetches the entity's entity configuration from the provided `entity_id`
2. Verifies the entity configuration signature
3. If `--jwks` is provided, verifies against those keys
4. If `--entity_type` is not provided, auto-detects from entity metadata
5. Stores the subordinate with status `active`

**Examples:**

```bash
# Add a subordinate, auto-detecting entity types
lhcli subordinates add https://rp.example.com

# Add a subordinate with explicit entity types
lhcli subordinates add https://op.example.com -t openid_provider -t oauth_authorization_server

# Add a subordinate with JWKS verification
lhcli subordinates add https://rp.example.com --jwks /path/to/entity-jwks.json
```

### subordinates remove

Remove a subordinate entity from the federation.

```bash
lhcli subordinates remove <entity_id>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `entity_id` | The entity identifier of the subordinate to remove |

**Example:**

```bash
lhcli subordinates remove https://rp.example.com
```

### subordinates block

Block a subordinate entity. Blocked entities remain in the database but are 
not included in the subordinate listing or fetch responses.

!!! note "Shortcut Command"
    This command is a shortcut for `subordinates status <entity_id> blocked`.
    For more flexibility, use the [`subordinates status`](#subordinates-status) command.

```bash
lhcli subordinates block <entity_id>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `entity_id` | The entity identifier of the subordinate to block |

**Example:**

```bash
lhcli subordinates block https://malicious.example.com
```

### subordinates status

Update the status of a subordinate entity.

```bash
lhcli subordinates status <entity_id> <status>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `entity_id` | The entity identifier of the subordinate |
| `status` | New status: `active`, `blocked`, `pending`, or `inactive` |

**Status Values:**

| Status | Description |
|--------|-------------|
| `active` | Subordinate is active and included in federation responses |
| `blocked` | Subordinate is blocked and excluded from federation responses |
| `pending` | Subordinate is awaiting approval |
| `inactive` | Subordinate is inactive (soft-disabled) |

**Examples:**

```bash
# Activate a subordinate
lhcli subordinates status https://rp.example.com active

# Block a subordinate (equivalent to: lhcli subordinates block)
lhcli subordinates status https://rp.example.com blocked

# Set to inactive (soft-disable without deleting)
lhcli subordinates status https://rp.example.com inactive

# Set back to pending for re-review
lhcli subordinates status https://rp.example.com pending
```

### subordinates requests

Interactively manage pending subordinate registration requests.

```bash
lhcli subordinates requests [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--print` | `-p` | Only print pending requests without prompting for action |
| `--only-ids` | | Only print entity IDs (not full subordinate info) |

**Behavior:**

Without `--print`, the command iterates through each pending request and 
prompts you to approve or reject it:

```
Do you approve entity 'https://new-rp.example.com' (y/n): 
```

- Answering `y` sets the subordinate status to `active`
- Answering `n` sets the subordinate status to `blocked`

**Examples:**

```bash
# Interactively process all pending requests
lhcli subordinates requests

# List pending requests without taking action
lhcli subordinates requests --print

# List only entity IDs of pending requests
lhcli subordinates requests --print --only-ids
```

---

## Trust Marks

Manage trust mark entitlements for entities. This controls which entities are 
entitled to receive specific trust marks from your Trust Mark Issuer.

!!! note "Command Aliases"
    The `trustmarks` command can also be invoked as `tm` or `trustmarked`.

### trustmarks add

Entitle an entity to receive a specific trust mark.

```bash
lhcli trustmarks add <trust_mark_type> <entity_id>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `trust_mark_type` | The trust mark type identifier (URI) |
| `entity_id` | The entity identifier to entitle |

**Example:**

```bash
lhcli trustmarks add https://federation.example.com/trustmarks/certified https://rp.example.com
```

### trustmarks remove

Remove a trust mark entitlement from an entity.

```bash
lhcli trustmarks remove <trust_mark_type> <entity_id>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `trust_mark_type` | The trust mark type identifier (URI) |
| `entity_id` | The entity identifier to remove the entitlement from |

**Example:**

```bash
lhcli trustmarks remove https://federation.example.com/trustmarks/certified https://rp.example.com
```

### trustmarks block

Block a trust mark entitlement for an entity. The entity will no longer be 
able to obtain this trust mark.

```bash
lhcli trustmarks block <trust_mark_type> <entity_id>
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `trust_mark_type` | The trust mark type identifier (URI) |
| `entity_id` | The entity identifier to block |

**Example:**

```bash
lhcli trustmarks block https://federation.example.com/trustmarks/certified https://bad-actor.example.com
```

### trustmarks requests

Interactively manage pending trust mark requests.

```bash
lhcli trustmarks requests [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--print`, `-p` | Only print pending requests without prompting for action |
| `--id` | Only process requests for a specific trust mark type |

**Behavior:**

Without `--print`, the command iterates through pending requests for each 
trust mark type (or a specific type if `--id` is provided) and prompts you 
to approve or reject:

```
Managing trust mark id 'https://federation.example.com/trustmarks/certified':

Do you approve entity 'https://rp.example.com' (y/n): 
```

- Answering `y` approves the trust mark entitlement
- Answering `n` blocks the trust mark entitlement

**Examples:**

```bash
# Interactively process all pending trust mark requests
lhcli trustmarks requests

# List all pending requests without taking action
lhcli trustmarks requests --print

# Process requests for a specific trust mark type only
lhcli trustmarks requests --id https://federation.example.com/trustmarks/certified
```

---

## Statistics

View and manage federation endpoint statistics. Statistics must be enabled 
in the configuration for these commands to work.

!!! info "Enabling Statistics"
    Statistics collection is disabled by default. Enable it in your configuration:
    
    ```yaml
    stats:
        enabled: true
    ```
    
    See [Statistics Configuration](../config/stats.md) for all options.

### Common Flags

These flags are available on most stats commands:

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | 24 hours ago | Start date (YYYY-MM-DD or RFC3339) |
| `--to` | now | End date (YYYY-MM-DD or RFC3339) |

### stats summary

Display an overall summary of request statistics.

```bash
lhcli stats summary [flags]
```

**Example:**

```bash
lhcli stats summary --from 2024-01-01 --to 2024-01-31
```

**Output:**

```
Statistics Summary (2024-01-01 to 2024-01-31)
============================================================
Total Requests:      1234567
Total Errors:        1234
Error Rate:          0.10%
Avg Latency:         45.20 ms
P50 Latency:         32 ms
P95 Latency:         120 ms
P99 Latency:         250 ms
Unique Clients:      5678
Unique User Agents:  42

Requests by Endpoint:
  well-known           500000
  fetch                400000
  resolve              334567

Requests by Status:
  200                  1233333
  404                  1000
  500                  234
```

### stats top

Show top-N statistics for various dimensions.

#### stats top endpoints

```bash
lhcli stats top endpoints [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | 10 | Number of results to show |

**Example:**

```bash
lhcli stats top endpoints --limit 20 --from 2024-01-01
```

#### stats top user-agents

```bash
lhcli stats top user-agents [flags]
```

**Example:**

```bash
lhcli stats top user-agents --limit 10
```

#### stats top clients

Show top client IP addresses by request count.

```bash
lhcli stats top clients [flags]
```

**Example:**

```bash
lhcli stats top clients --limit 10
```

#### stats top countries

Show top countries by request count (requires GeoIP configuration).

```bash
lhcli stats top countries [flags]
```

**Example:**

```bash
lhcli stats top countries --limit 10
```

### stats timeseries

Display time series data for request counts.

```bash
lhcli stats timeseries [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `hour` | Time bucket size: `minute`, `hour`, `day`, `week`, `month` |
| `--endpoint` | all | Filter by specific endpoint |

**Example:**

```bash
lhcli stats timeseries --interval hour --endpoint fetch --from 2024-01-01
```

**Output:**

```
Time Series (2024-01-01 to 2024-01-02, interval: hour)
Timestamp                 Requests     Errors  Avg Latency
------------------------------------------------------------
2024-01-01 00:00               1234        5      42.50 ms
2024-01-01 01:00               1456        3      38.20 ms
2024-01-01 02:00                892        1      35.10 ms
```

### stats latency

Display latency percentile statistics.

```bash
lhcli stats latency [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--endpoint` | all | Filter by specific endpoint |

**Example:**

```bash
lhcli stats latency --endpoint resolve
```

**Output:**

```
Latency Percentiles (2024-01-01 to 2024-01-31)
Endpoint: resolve
========================================
P50:  32 ms
P75:  55 ms
P90:  95 ms
P95:  120 ms
P99:  250 ms
Avg:  45.20 ms
Min:  5 ms
Max:  2500 ms
```

### stats export

Export statistics data to a file.

```bash
lhcli stats export [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | | `csv` | Export format: `csv` or `json` |
| `--output` | `-o` | stdout | Output file path |

**Examples:**

```bash
# Export to CSV file
lhcli stats export --format csv --output stats.csv --from 2024-01-01

# Export to JSON file
lhcli stats export --format json --output stats.json --from 2024-01-01

# Export to stdout
lhcli stats export --format json
```

### stats purge

Delete old statistics data based on retention settings.

```bash
lhcli stats purge [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be purged without actually deleting |

**Examples:**

```bash
# Preview what would be purged
lhcli stats purge --dry-run

# Actually purge old data
lhcli stats purge
```

**Output (dry run):**

```
Dry run mode - no data will be deleted
Would purge detailed logs before: 2023-10-03
Would purge aggregated stats before: 2023-01-01
```

### stats aggregate

Manually run daily aggregation for a specific date. This is normally done 
automatically at 2 AM UTC.

```bash
lhcli stats aggregate [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--date` | yesterday | Date to aggregate (YYYY-MM-DD) |

**Example:**

```bash
lhcli stats aggregate --date 2024-01-15
```

---

## Delegation

Generate trust mark delegation JWTs for delegating trust mark issuance 
authority to other entities.

```bash
lhcli delegation <config_file> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `config_file` | Path to a YAML configuration file defining delegations |

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON instead of YAML |

### Configuration File Format

The delegation configuration file has the following structure:

```yaml
# The trust mark owner's entity ID
trust_mark_owner: https://federation.example.com

# Optional: JWKS for the owner (generated if not provided)
jwks:
  keys: []

# Optional: PEM-encoded signing key (generated if not provided)
signing_key: |
  -----BEGIN EC PRIVATE KEY-----
  ...
  -----END EC PRIVATE KEY-----

# Trust marks to delegate
trust_marks:
  - trust_mark_type: https://federation.example.com/trustmarks/certified
    delegation_lifetime: 365d
    ref: https://federation.example.com/trustmarks/certified/policy
    trust_mark_issuers:
      - entity_id: https://issuer1.example.com
      - entity_id: https://issuer2.example.com

  - trust_mark_type: https://federation.example.com/trustmarks/verified
    delegation_lifetime: 180d
    trust_mark_issuers:
      - entity_id: https://issuer1.example.com
```

### Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `trust_mark_owner` | Yes | Entity ID of the trust mark owner |
| `jwks` | No | JWKS containing owner's public keys (auto-generated if omitted) |
| `signing_key` | No | PEM-encoded private key for signing (auto-generated if omitted) |
| `trust_marks` | Yes | List of trust mark delegations |

**Trust Mark Fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `trust_mark_type` | Yes | The trust mark type identifier (URI) |
| `delegation_lifetime` | No | Validity period (e.g., `365d`, `1y`, `8760h`) |
| `ref` | No | Reference URL for the trust mark policy |
| `trust_mark_issuers` | Yes | List of entities to delegate to |

### Behavior

1. Reads the configuration file
2. If `signing_key` is not provided, generates a new EC P-521 key pair
3. If `jwks` is not provided, generates it from the signing key
4. For each trust mark issuer, generates a delegation JWT
5. Writes the updated configuration back to the file (with generated keys and JWTs)

### Example

**Input (`delegation-config.yaml`):**

```yaml
trust_mark_owner: https://federation.example.com
trust_marks:
  - trust_mark_type: https://federation.example.com/trustmarks/certified
    delegation_lifetime: 365d
    trust_mark_issuers:
      - entity_id: https://issuer.example.com
```

**Command:**

```bash
lhcli delegation delegation-config.yaml
```

**Output (`delegation-config.yaml` - updated):**

```yaml
trust_mark_owner: https://federation.example.com
jwks:
  keys:
    - kty: EC
      crv: P-521
      x: "..."
      y: "..."
signing_key: |
  -----BEGIN EC PRIVATE KEY-----
  MIHuAgEAMBAGByqGSM49AgEGBSuBBAAjBIHWMIHTAgEBBEIA...
  -----END EC PRIVATE KEY-----
trust_marks:
  - trust_mark_type: https://federation.example.com/trustmarks/certified
    delegation_lifetime: 365d
    trust_mark_issuers:
      - entity_id: https://issuer.example.com
        delegation_jwt: eyJhbGciOiJFUzUxMiIsInR5cCI6InRydXN0LW1hcmstZGVsZWdhdGlvbitqd3QifQ...
```

**JSON Output:**

```bash
lhcli delegation delegation-config.yaml --json
```

This writes the output to `delegation-config.json` in JSON format.

---

## Examples

### Onboarding a New Subordinate

```bash
# Add the subordinate
lhcli subordinates add https://new-rp.example.com -t openid_relying_party

# Entitle them to a trust mark
lhcli trustmarks add https://federation.example.com/trustmarks/certified https://new-rp.example.com
```

### Processing Pending Requests

```bash
# Check for pending subordinate requests
lhcli subordinates requests --print

# Process them interactively
lhcli subordinates requests

# Check for pending trust mark requests
lhcli trustmarks requests --print

# Process a specific trust mark type
lhcli trustmarks requests --id https://federation.example.com/trustmarks/certified
```

### Daily Statistics Review

```bash
# View yesterday's summary
lhcli stats summary --from $(date -d yesterday +%Y-%m-%d)

# Check top endpoints
lhcli stats top endpoints --limit 5

# View hourly traffic pattern
lhcli stats timeseries --interval hour --from $(date -d yesterday +%Y-%m-%d)
```

### Monthly Statistics Export

```bash
# Export last month's data to CSV
lhcli stats export \
  --from $(date -d "last month" +%Y-%m-01) \
  --to $(date -d "$(date +%Y-%m-01) -1 day" +%Y-%m-%d) \
  --format csv \
  --output monthly-stats.csv
```

### Emergency: Block a Compromised Entity

```bash
# Block the subordinate (using shortcut)
lhcli subordinates block https://compromised.example.com

# Or using the status command
lhcli subordinates status https://compromised.example.com blocked

# Also block their trust marks
lhcli trustmarks block https://federation.example.com/trustmarks/certified https://compromised.example.com
```

### Reactivating a Previously Blocked Entity

```bash
# Reactivate the subordinate
lhcli subordinates status https://rp.example.com active

# Also reactivate their trust marks if needed
lhcli trustmarks add https://federation.example.com/trustmarks/certified https://rp.example.com
```
