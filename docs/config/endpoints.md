---
icon: material/routes
---

# Endpoints Configuration

!!! danger "Configuration Section Removed"

    The `endpoints` configuration section has been **removed**. All federation
    endpoint configuration is now managed in the database via the
    [Admin API](../features/admin_api.md).

    - **Endpoint paths, URLs, auth settings**: managed via
      `POST/PUT/DELETE /api/v1/admin/federation-endpoints`
    - **Trust anchors**: managed via
      `POST/PUT/DELETE /api/v1/admin/trust-anchors`
    - **JTI storage settings**: moved to
      [`storage.endpoint_auth`](storage.md#endpoint_auth)

    Use [`lhmigrate config2db`](../migration/0.22.md) to migrate
    endpoint settings from an existing config file to the database.

    See [Endpoints](../features/endpoints.md) for details on managing
    endpoints at runtime.
