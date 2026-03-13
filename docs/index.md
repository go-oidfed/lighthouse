# LightHouse - A Configurable OIDFed Trust Anchor


  ![Logo](./assets/logo_dm.svg#only-dark){ width="300" loading=lazy align=left }
  ![Logo](./assets/logo_lm.svg#only-light){ width="300" loading=lazy align=left } 


LightHouse helps you to navigate the wild and complex sea of OpenID 
Federation. Based on the
[go-oidfed implementation](https://github.com/go-oidfed/lib), LightHouse 
provides an easy to use, flexible, and configurable Trust Anchor, 
Intermediate Authority, Resolver, and / or Trust Mark Issuer.
By deploying LightHouse to your federation, entities will know there now is 
an Entity that will guide them and which they can put their trust in so they 
can safely drop anchor.

The [LightHouse source code](https://github.com/go-oidfed/lighthouse) can 
also be used 
as a starting point to implement your own Trust Anchor based on the 
[go-oidfed library](https://github.com/go-oidfed/lib).

However, the primary goal of lighthouse is to have an easy to set up 
Federation Authority that can be configured according to your needs and 
requirements.


## Getting Started

<div class="grid cards" markdown>

-   :material-rocket-launch:{ .lg .middle } **Deployment**

    ---

    Get LightHouse up and running with Docker and Caddy reverse proxy.

    [:octicons-arrow-right-24: Deploy LightHouse](deployment/caddy.md)

-   :material-file-cog:{ .lg .middle } **Configuration**

    ---

    Configure LightHouse via YAML config file or environment variables.

    [:octicons-arrow-right-24: Configuration Guide](config)

</div>

## Core Features

<div class="grid cards" markdown>

-   :material-routes:{ .lg .middle } **Federation Endpoints**

    ---

    Entity configuration, fetch, resolve, list, and trust mark endpoints.

    [:octicons-arrow-right-24: Endpoints](features/endpoints.md)

-   :material-marker-check:{ .lg .middle } **Trust Marks**

    ---

    Issue, manage, and verify trust marks with delegation support.

    [:octicons-arrow-right-24: Trust Marks](features/trustmarks.md)

-   :material-police-badge:{ .lg .middle } **Entity Checks**

    ---

    Automatic validation of entities during enrollment and trust mark requests.

    [:octicons-arrow-right-24: Entity Checks](features/entity_checks.md)

-   :material-api:{ .lg .middle } **Admin API**

    ---

    RESTful API for managing subordinates, trust marks, keys, and configuration.

    [:octicons-arrow-right-24: Admin API](features/admin_api.md)

-   :material-chart-line:{ .lg .middle } **Statistics**

    ---

    Capture and analyze request metrics, latency, and usage patterns.

    [:octicons-arrow-right-24: Statistics](features/statistics.md)

-   :material-console:{ .lg .middle } **CLI Tool**

    ---

    Manage LightHouse from the command line with `lhcli`.

    [:octicons-arrow-right-24: CLI Reference](deployment/lhcli.md)

</div>

## Resources

<div class="grid cards" markdown>

-   :material-map-check:{ .lg .middle } **Feature Overview**

    ---

    Complete list of supported and planned features.

    [:octicons-arrow-right-24: Features](features)

-   :material-walk:{ .lg .middle } **Migration Guide**

    ---

    Upgrade from LightHouse < 0.20.0 to the latest version.

    [:octicons-arrow-right-24: Migration](migration)

-   :fontawesome-brands-github:{ .lg .middle } **Source Code**

    ---

    View the source code, report issues, or contribute.

    [:octicons-arrow-right-24: GitHub](https://github.com/go-oidfed/lighthouse)

-   :simple-openid:{ .lg .middle } **go-oidfed Library**

    ---

    The underlying OpenID Federation library for Go.

    [:octicons-arrow-right-24: go-oidfed](https://github.com/go-oidfed/lib)

</div>
