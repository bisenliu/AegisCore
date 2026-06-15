## 1. Role And Permission Use Case Tests

- [x] 1.1 Add role command unit tests for role lifecycle behavior and validation outcomes.
- [x] 1.2 Add role query unit tests for role listing/detail behavior without requiring `roles.code` for authorization decisions.
- [x] 1.3 Add user-role binding tests for assign, replace, remove, no-role, disabled-role, and user-role unbinding scenarios.
- [x] 1.4 Add role-permission binding tests for assign, replace, remove, disabled-permission, and role-permission unbinding scenarios.
- [x] 1.5 Add permission command unit tests for permission lifecycle behavior and validation outcomes.
- [x] 1.6 Add permission query unit tests for permission listing/detail and effective-permission lookup behavior.

## 2. Route Diff And Authorization Tests

- [x] 2.1 Add permission route diff tests for discovered missing and extra route permission differences.
- [x] 2.2 Add route diff tests proving discovery does not create formal permissions or bind roles.
- [x] 2.3 Add Casbin policy loader tests proving policies use `role_id` subjects and do not depend on `roles.code`.
- [x] 2.4 Add Casbin enforcer tests for allow, deny, wildcard `super_admin`, and disabled role/permission outcomes.
- [x] 2.5 Add Casbin reload tests proving authorization decisions reflect refreshed role, permission, and binding state.
- [x] 2.6 Add Gin RBAC middleware HTTP tests for authorized requests, no-role users, disabled roles, disabled permissions, user-role unbinding, role-permission unbinding, and `super_admin` wildcard access.

## 3. Swagger And Documentation

- [x] 3.1 Update Swagger annotations for role and permission endpoints and RBAC-protected behavior where current annotations are incomplete or stale.
- [x] 3.2 Run the documented Swagger generation command and commit generated artifacts without manual generated-file edits.
- [x] 3.3 Update `AGENTS.md` to describe role and permission as implemented features and remove any skeleton-only wording.
- [x] 3.4 Update `docs/ARCHITECTURE.md` with current role, permission, Casbin, route diff, and feature-boundary guidance.
- [x] 3.5 Update `docs/DEVELOPMENT.md` or `docs/TESTING.md` with RBAC regression scope, acceptance commands, route diff read-only behavior, and the fact that `roles.code` is not required by Casbin authorization.

## 4. Verification

- [x] 4.1 Run `go test ./...` in `common/` and fix any regression in shared packages.
- [x] 4.2 Run `go test ./...` in `user-service/` and fix any role, permission, Casbin, middleware, or Swagger-related regression.
- [x] 4.3 Run `make test` or the equivalent full repository test scope.
- [x] 4.4 Verify application packages do not import Gin, Ent, Redis clients, or SQL infrastructure packages.
- [x] 4.5 Review generated Swagger and documentation diffs to confirm no schema changes, new business capabilities, Redis multi-instance synchronization, menu permissions, multi-tenancy, audit logging, or expanded `common/` responsibilities were introduced.
