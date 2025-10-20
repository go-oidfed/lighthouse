# Config
LightHouse is configured through a single configuration file named `config.yaml`.

## Config File Location

LightHouse will search for this file at startup at different locations, the 
first file that is found will be used. Supported locations are:

- `config.yaml`
- `config/config.yaml`
- `/config/config.yaml`
- `/lighthouse/config/config.yaml`
- `/lighthouse/config.yaml`
- `/data/config/config.yaml`
- `/data/config.yaml`
- `/etc/lighthouse/config.yaml`

## Example Config File
The following is an example `config.yaml` file:

??? file "config.yaml"

    ```yaml



    server:
        port: 7672
    signing:
        key_dir: "/keys"
    federation_data:
        entity_id: "https://ta.example.lh"
        authority_hints:
            - "https://trust-anchor.spid-cie.fedservice.lh/"
        federation_entity_metadata:
            display_name: "Example Federation TA"
            organization_name: "Example Organization"
        metadata_policy_file: "/metadata-policy.json"
        trust_mark_issuers:
            "https://go-ia.federservice.lh/tm/federation-member":
                - "https://go-ia.fedservice.lh"
        trust_marks:
            - id: "https://go-ia.federservice.lh/tm/federation-member"
              trust_mark: "eyJhbGciOiJFUzUxMiIsImtpZCI6IlpsSFBmQXJTRnFGdjNHRlh3ZUptbmFkZDI4YTM4X3plcEJybEZkWHdIaTQiLCJ0eXAiOiJ0cnVzdC1tYXJrK2p3dCJ9.eyJleHAiOj..."
              refresh: true
            - id: "https://trust-anchor.federservice.lh/tm/federation-member"
              trust_mark: "eyJhbGciOiJFUzUxMiIsImtpZCI6InpFLTlhVlhJanJZOUcxVU0tYURQVkxVR1RkWmFuOTk0NlJJUWhraWFjUVkiLCJ0eXAiOiJ0cnVzdC1tYXJrK2p3dCJ9.eyJleHAiO..."
              refresh: true
    storage:
        backend: json
        data_dir: "/data"
    endpoints:
        fetch:
            path: "/fetch"
        list:
            path: "/list"
        resolve:
            path: "/resolve"
        entity_collection:
            path: "/entity-collection"
            allowed_trust_anchors:
                - https://ta.example.org
            interval: 8h
        trust_mark:
            path: "/trustmark"
            trust_mark_specs:
                - trust_mark_type: "https://tm.example.org"
                  lifetime: 3600
                  ref: "https://tm.example.org/ref"
                  logo_uri: "https://tm.example.org/logo"
                  extra_claim: "example"
                  delegation_jwt:
                - trust_mark_type: "https://edugain.org"
                  lifetime: 86400
        trust_mark_status:
            path: "/trustmark/status"
        trust_mark_list:
            path: "/trustmark/list"
        enroll:
            path: "/enroll"
            checker:
                type: multiple_or
                config:
                    - type: trust_mark
                      config:
                          trust_mark_type: https://tm.example.org
                          trust_anchors:
                              - entity_id: https://ta.example.org
                    - type: trust_mark
                      config:
                          trust_mark_type: https://tm.example.com
                          trust_anchors:
                              - entity_id: https://example.com
                              - entity_id: https://foo.bar.com
    ```

## Configuration Sections

<div class="grid cards" markdown>


- [:material-server-network: Server](server.md)
- [:material-script-text: Logging](logging.md)
- [:material-database: Storage](storage.md)
- [:material-database-clock: Cache](cache.md)
- [:material-signature-freehand: Signing](signing.md)
- [:material-routes: Endpoints](endpoints.md)
- [:simple-openid: Federation Data](federation_data.md)

</div>

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
