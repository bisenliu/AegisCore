## Why

The user service currently keeps `permission` as a future skeleton, so there is no durable permission catalog for operators to maintain or inspect. Adding a permission directory establishes the foundation for RBAC without coupling the service to role assignment, Casbin enforcement, or automatic authorization side effects.

## What Changes

- Expand `user-service/internal/features/permission/` into a real feature with domain, application, transport, infrastructure, and Fx wiring.
- Add a manually maintained system permission baseline for known service routes.
- Add permission management use cases for creating, updating, enabling, and disabling permissions.
- Add permission query use cases for listing permissions, reading permission details, querying a user's effective permissions, and reporting route catalog differences.
- Add Gin route discovery as a read-only catalog scanner that excludes non-authorizable routes and reports only missing routes and stale permission entries.
- Add `GET /api/v1/permissions/route-diff` plus the surrounding permission management HTTP DTOs, controller, routes, and response models.
- Add Ent-backed PostgreSQL persistence for permissions with uniqueness based on `http_method + path_template`.
- Preserve current exclusions: no Casbin enforcer, no role management, no user-role binding, no role-permission binding, no automatic database writes from route discovery, no automatic authorization grants, and no menu or button permissions.

## Capabilities

### New Capabilities

- `permission-directory`: Manual system permission catalog, permission management/query use cases, and read-only route difference reporting.

### Modified Capabilities

- None.

## Impact

- Adds a real permission feature under `user-service/internal/features/permission/` following the repository's feature layering rules.
- Adds or updates Ent schema, generated Ent code, and Atlas migration for the `permissions` table.
- Adds HTTP endpoints under `/api/v1/permissions`, including `GET /api/v1/permissions/route-diff`.
- Adds route catalog scanning against the service Gin engine without mutating permission storage.
- Adds tests for domain validation, application use cases, route diff behavior, HTTP boundary behavior, and PostgreSQL adapter behavior where feasible.
