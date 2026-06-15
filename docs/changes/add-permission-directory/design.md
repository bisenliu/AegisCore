## Context

`user-service/internal/features/permission/` is currently only a future RBAC skeleton. The service needs a permission directory that can be manually maintained by operators and audited against registered HTTP routes, while preserving the repository rule that RBAC role management and enforcement remain out of scope.

The implementation must follow the existing feature layering model: domain rules in `domain`, use cases and ports in `application`, Gin boundary code in `transport/http`, Ent persistence in `infrastructure/postgres`, and feature assembly in `fx.go`. Application code must not import Gin, Ent, or Redis, and route discovery must be read-only.

## Goals / Non-Goals

**Goals:**

- Introduce a durable permission directory with uniqueness based on `http_method + path_template`.
- Provide manual permission management use cases for create, update, enable, and disable.
- Provide query use cases for list, detail, user effective permissions, and route diff.
- Seed or expose a manually maintained baseline of system permissions in application catalog code.
- Discover authorizable Gin routes and compare them against stored permissions without writing discovery results to the database.
- Protect system permissions from ordinary deletion or destructive changes to key identity fields.
- Register permission HTTP routes under `/api/v1/permissions`, including `GET /api/v1/permissions/route-diff`.

**Non-Goals:**

- No Casbin enforcer integration.
- No role management feature.
- No user-role binding or role-permission binding.
- No automatic route scan writes to the `permissions` table.
- No automatic grants to any role or user.
- No `permissions.code`, `permissions.source`, `permissions.resource`, or `permissions.action` fields.
- No menu permissions or button permissions.

## Decisions

1. Model permission identity as HTTP method plus route template.

   The permission directory represents authorizable HTTP endpoints, so `http_method + path_template` is the natural stable identity. A database unique index enforces this rule, and domain validation normalizes method casing and validates route template shape before persistence. This avoids introducing unrelated concepts such as code/resource/action while keeping the catalog aligned with actual Gin route paths.

   Alternative considered: add a separate permission code. This was rejected because the requested scope explicitly excludes `permissions.code`, and code generation would create another identity source that can drift from routes.

2. Keep the system permission catalog manually maintained in application code.

   `application/catalog` will expose `DefaultPermissions()` with explicit `PermissionSpec` entries. These entries document baseline permissions that should exist, but they do not automatically grant roles or replace operator-managed records. The catalog belongs in application because it is service policy, not transport or persistence behavior.

   Alternative considered: derive all permissions from route discovery. This was rejected because the directory is manual-first, and automatic discovery must remain an audit aid.

3. Implement route discovery as an application port backed by a Gin adapter.

   `application/ports.go` defines `RouteCatalogScanner`, returning route method and path values independent of Gin. The infrastructure or provider-side scanner can use `engine.Routes()` and exclude `OPTIONS`, non-`/api/v1/` paths, and public auth session routes. The route diff query compares scanner output with stored permissions and returns only `MissingInPermissions` and `StalePermissions`.

   Alternative considered: call Gin directly from the query service. This was rejected because application must stay transport-neutral and must not import Gin.

4. Use disable instead of ordinary delete for permission lifecycle.

   The command layer exposes enable/disable operations and rejects destructive changes to system permissions, especially method and path template updates. Ordinary deletion is not part of the initial surface. If a physical delete is later required, it should be a separate privileged operation with explicit system-permission protections.

   Alternative considered: allow delete with soft-delete semantics. This was rejected for the initial change because enable/disable satisfies lifecycle needs while reducing accidental catalog loss.

5. Keep user effective permissions query as a permission-facing read model placeholder backed by ports.

   The endpoint and query model can exist, but because role and binding features are out of scope, the use case should depend on a narrow store/query port and return effective permission results only from currently available permission data. It must not introduce role tables, joins, or authorization enforcement.

   Alternative considered: add role-permission storage now to make the query complete. This was rejected because role binding is explicitly out of scope.

## Risks / Trade-offs

- Route template drift between Gin and manual catalog -> Mitigation: route diff reports `MissingInPermissions` and `StalePermissions` using normalized method/path identity.
- System permission updates may be too restrictive for legitimate maintenance -> Mitigation: permit non-destructive metadata changes such as name, description, module, and enabled state while rejecting identity-field changes for system records.
- Route discovery depends on route registration order and Fx wiring -> Mitigation: register the scanner after Gin routes are assembled and test scanner behavior with a real Gin engine.
- User effective permissions may be limited until role binding exists -> Mitigation: keep the query transport and application boundary narrow and avoid adding role-specific persistence in this change.

## Migration Plan

1. Add Ent permission schema with fields for name, description, module, http method, path template, system flag, enabled flag, and timestamps.
2. Generate Ent code and an Atlas migration for the `permissions` table and the `(http_method, path_template)` unique index.
3. Add feature code under `internal/features/permission/` and wire the feature Fx module into service bootstrap.
4. Register HTTP routes under `/api/v1/permissions`.
5. Add route scanner provider that reads Gin routes without mutating permission storage.
6. Validate with `go test ./...` under `user-service/` and targeted tests for domain, application, and transport boundaries.

Rollback is standard database/application rollback: revert service code, remove route registration, and roll back the permission migration if no production permission records must be preserved.

## Open Questions

- Whether default system permissions should be inserted through a separate explicit admin command, migration seed, or operational runbook is deferred to implementation planning; this change only requires the catalog baseline and manual-first directory behavior.
