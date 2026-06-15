## 1. Catalogs And Contracts

- [x] 1.1 Add `user-service/internal/features/role/application/catalog/` with `SuperAdminRoleID`, `RoleSpec`, `DefaultRoles`, `RolePermissionSpec`, and `DefaultRolePermissions`.
- [x] 1.2 Extend `user-service/internal/features/permission/application/catalog/PermissionSpec` to include stable `PermissionID` values and ensure default user route permissions include list and create user permissions.
- [x] 1.3 Add seed-specific application input/result types for roles, permissions, role-permission bindings, and super-admin assignment.
- [x] 1.4 Add minimal application ports for system role upsert, system permission upsert, system role-permission completion/synchronization, and user-role assignment.

## 2. Seed Application Flow

- [x] 2.1 Implement an RBAC seed application use case that loads role, permission, and role-permission catalogs and validates duplicate IDs or duplicate route identities before writing.
- [x] 2.2 Implement default seed behavior that upserts catalog-managed system roles and permissions without reactivating existing `active=false` entries.
- [x] 2.3 Implement `--reactivate-system` behavior that explicitly applies catalog active state to existing system roles and permissions.
- [x] 2.4 Implement default role-permission binding behavior that adds missing catalog bindings without removing extra bindings.
- [x] 2.5 Implement `--sync-system-bindings` behavior that precisely synchronizes catalog-managed system role bindings.
- [x] 2.6 Implement super-admin assignment use case that idempotently binds a specified existing user to the catalog super-admin role.

## 3. PostgreSQL Adapters

- [x] 3.1 Implement role PostgreSQL seed adapter methods for idempotent system role upsert by `role_id`.
- [x] 3.2 Implement permission PostgreSQL seed adapter methods for idempotent system permission upsert by `permission_id` or `http_method + path_template`.
- [x] 3.3 Implement role-permission seed adapter methods for adding missing bindings and synchronizing system role bindings in a transaction.
- [x] 3.4 Implement user-role assignment adapter behavior that treats existing super-admin binding as success.
- [x] 3.5 Ensure adapter errors map existing domain errors consistently and do not introduce HTTP, Gin, or transport dependencies.

## 4. CLI And Makefile Wiring

- [x] 4.1 Add an `rbac` Cobra command group under `aegiscore-user-services` with shared `--config` handling consistent with `serve`.
- [x] 4.2 Add `rbac seed` with `--reactivate-system` and `--sync-system-bindings` flags.
- [x] 4.3 Add `rbac assign-super-admin --user-id <uuid>` with UUID validation and clear command errors for missing users or roles.
- [x] 4.4 Build a seed runner that initializes only configuration, logging, PostgreSQL/Ent resources, and required seed dependencies without starting the HTTP server.
- [x] 4.5 Add `make seed-rbac` that invokes the user-service RBAC seed command using `USER_SERVICE_CONFIG`.

## 5. Tests

- [x] 5.1 Add catalog tests for stable IDs, duplicate prevention, and expected default role/permission entries.
- [x] 5.2 Add application seed tests for idempotent repeat execution, active-state preservation, `--reactivate-system`, default binding completion, and `--sync-system-bindings`.
- [x] 5.3 Add PostgreSQL adapter tests for upsert conflict paths and binding synchronization transaction behavior.
- [x] 5.4 Add CLI tests for command registration, required flags, invalid UUID handling, and ensuring `serve` does not invoke seed.

## 6. Verification And Documentation Updates

- [x] 6.1 Update deployment/development documentation or command help to show migrate schema -> seed RBAC data -> start HTTP server.
- [x] 6.2 Run `go test ./...` under `user-service/` and fix failures.
- [ ] 6.3 Run route-diff after seed in a seeded environment and verify MissingInPermissions and StalePermissions behavior for catalog updates.
