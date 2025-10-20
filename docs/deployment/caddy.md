---
title: Deployment
description: Deployment of LightHouse with a Caddy reverse proxy.
#icon: simple/caddy
icon: simple/docker
---

The following example configuration can be used (tweak as needed) to deploy 
LightHouse using docker and Caddy:

We assume the following project layout:

```tree
caddy/
    Caddyfile #(1)!
    config/ 
    data/
docker-compose.yaml #(2)!
lighthouse/
    config.yaml #(3)!
    data/
        metadata-policy.json
        storage/
        signing/
```

1. [`caddy/CaddyFile`](#caddycaddyfile)
2. [`docker-compose.yaml`](#docker-composeyaml)
3. [`lighthouse/config.yaml`](#lighthouseconfigyaml)


=== ":material-file-code: `docker-compose.yaml`"

    ```yaml
    services:
      caddy:
        image: caddy:latest
        restart: unless-stopped
        ports:
          - "80:80"
          - "443:443"
        volumes:
          - ./caddy/Caddyfile:/etc/caddy/Caddyfile
          - ./caddy/data:/data
          - ./caddy/config:/config

      lighthouse:
        image: oidfed/lighthouse:main
        restart: unless-stopped
        volumes:
          - ./lighthouse/config.yaml:/config.yaml:ro
          - ./lighthouse/data:/data

    ```

=== ":material-file-code: `caddy/Caddyfile`"

    ```caddy
    lighthouse.example.com {
      reverse_proxy lighthouse:7672
    }
    ```

=== ":material-file-code: `lighthouse/config.yaml`"

    ```yaml
    server:
        port: 7672
    signing:
        alg: ES256
        key_dir: "/data/signing"
    federation_data:
        entity_id: "https://lighthouse.example.com"
        federation_entity_metadata:
            display_name: "Example Federation TA"
            organization_name: "Example Organization"
        metadata_policy_file: "/data/metadata-policy.json"
    storage:
        backend: badger
        data_dir: "/data/storage"
    endpoints:
        fetch:
            path: "/fetch"
        list:
            path: "/list"
        resolve:
            path: "/resolve"
        trust_mark:
            path: "/trustmark"
            trust_mark_specs:
              - trust_mark_type: "https://tm.example.org"
                lifetime: 3600
                ref: "https://tm.example.org/ref"
                logo_uri: "https://tm.example.org/logo"
                checker:
                    type: trust_path
                    config:
                        trust_anchors:
                            - entity_id: "https://lighthouse.example.com"
        trust_mark_list:
            path: "/trustmark/list"
    ```

    For more information about the lighthouse config file, please refer to 
    [Configuration](../config/index.md).
