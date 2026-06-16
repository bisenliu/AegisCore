## 1. Data Model And Generation

- [x] 1.1 Add Ent permission schema with name, description, module, http method, path template, system flag, enabled flag, timestamps, and a unique index on `http_method + path_template`.
- [x] 1.2 Run Ent code generation and verify generated code is not manually edited.
- [x] 1.3 Create and validate an Atlas migration for the `permissions` table and unique index.

## 2. Domain And Application Contracts

- [x] 2.1 Add `permission/domain` types for permission entity, route identity, supported HTTP method validation, route template validation, system permission protection, and domain errors.
- [x] 2.2 Add `permission/application/ports.go` with `PermissionStore`, `RouteCatalogScanner`, and query/store DTOs owned by the permission application layer.
- [x] 2.3 Add `permission/application/catalog` with `PermissionSpec` and `DefaultPermissions()` for manually maintained system permission baseline entries.
- [x] 2.4 Add `permission/application/validators` for transport-neutral command/query input validation and normalization.

## 3. Application Use Cases

- [x] 3.1 Implement create permission command use case with validation, method normalization, and duplicate identity error mapping.
- [x] 3.2 Implement update permission command use case with system permission identity protection.
- [x] 3.3 Implement enable and disable permission command use cases without physical deletion.
- [x] 3.4 Implement permission list and detail query use cases using application ports and pagination/filter inputs.
- [x] 3.5 Implement user effective permissions query without adding role management, user-role binding, or role-permission binding behavior.
- [x] 3.6 Implement route diff query that compares discovered routes with stored permissions and returns only `MissingInPermissions` and `StalePermissions`.

## 4. Infrastructure Adapters

- [x] 4.1 Add `permission/infrastructure/postgres` Ent adapter implementing permission persistence ports without importing Gin or HTTP response packages.
- [x] 4.2 Add route catalog scanner adapter that reads `gin.Engine.Routes()`, excludes non-authorizable routes, and does not write to the database.
- [x] 4.3 Map Ent constraint, not-found, and validation failures to application/domain errors at adapter boundaries.

## 5. HTTP Transport And Fx Wiring

- [x] 5.1 Add feature-local HTTP request DTOs, response DTOs, and mapping helpers for permission management and query operations.
- [x] 5.2 Add permission HTTP controller handlers for create, update, enable, disable, list, detail, user effective permissions, and route diff.
- [x] 5.3 Add `transport/http/routes.go` registering permission routes under `/api/v1/permissions`, including `GET /api/v1/permissions/route-diff`.
- [x] 5.4 Add `permission/fx.go` to provide feature services, controllers, adapters, and route registration.
- [x] 5.5 Wire the permission feature module into the service bootstrap/provider graph without moving business logic into service-level providers.

## 6. Tests And Verification

- [x] 6.1 Add domain tests for HTTP method normalization, route template validation, duplicate identity semantics, and system permission protection.
- [x] 6.2 Add application tests for command use cases, query use cases, route diff behavior, and read-only route discovery guarantees.
- [x] 6.3 Add HTTP transport tests for DTO mapping, response envelope usage, and `GET /api/v1/permissions/route-diff` behavior.
- [x] 6.4 Add PostgreSQL adapter tests where existing test infrastructure supports Ent-backed persistence checks.
- [x] 6.5 Run `gofmt` on changed Go files and run `go test ./...` under `user-service/`.
- [x] 6.6 Verify permission application packages do not import Gin, Ent, Redis, SQL, or HTTP response packages, and PostgreSQL adapter packages do not import Gin or HTTP response packages.
