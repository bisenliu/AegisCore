## 1. Package Setup

- [x] 1.1 Add Casbin v2 dependency to `user-service/go.mod` if it is not already present.
- [x] 1.2 Create `user-service/internal/features/permission/infrastructure/casbin/` with package-local tests.
- [x] 1.3 Add `model.conf` with the RBAC model for `sub`, `obj`, `act`, grouping policy, allow effect, exact matching, and wildcard `*` object/action matching.
- [x] 1.4 Add a `go:embed` model loader that returns a Casbin `model.Model` from `model.conf`.

## 2. Policy Loading

- [x] 2.1 Define policy loader data structures for grouping policies, permission policies, and super-admin role configuration.
- [x] 2.2 Implement Ent-backed loading of active user-role bindings as `g(user:<user_uuid>, role:<role_uuid>)`.
- [x] 2.3 Implement Ent-backed loading of active role-permission bindings as `p(role:<role_uuid>, <path_template>, <http_method>)`.
- [x] 2.4 Ensure inactive roles and inactive permissions are excluded from all loaded policies.
- [x] 2.5 Add fixed super-admin role ID wildcard policy `p(role:<super_admin_role_uuid>, *, *)` without relying on `roles.code`.

## 3. Enforcer Runtime

- [x] 3.1 Implement enforcer construction that creates a fresh in-memory Casbin enforcer and loads all policies before publishing it.
- [x] 3.2 Implement `Enforce(ctx, userID, pathTemplate, method)` using only the currently published in-memory enforcer.
- [x] 3.3 Implement fail-closed behavior when no valid enforcer has been published.
- [x] 3.4 Implement full `Reload` that publishes a newly built enforcer only after all loading succeeds.
- [x] 3.5 Ensure reload failures preserve the previously published policy and expose no partially updated policy.

## 4. Feature Wiring

- [x] 4.1 Add a provider for the Casbin policy engine in the permission feature module.
- [x] 4.2 Keep the new engine disconnected from Gin middleware and router registration.
- [x] 4.3 Keep RBAC database access confined to initialization and reload paths, not enforcement calls.

## 5. Verification

- [x] 5.1 Add tests for embedded model loading and wildcard matcher behavior.
- [x] 5.2 Add tests for translating active user-role and role-permission records into Casbin policies.
- [x] 5.3 Add tests proving inactive roles and inactive permissions are not loaded.
- [x] 5.4 Add tests for super-admin wildcard access by fixed role ID.
- [x] 5.5 Add tests for `Enforce` allow, deny, and no-database-on-enforce behavior.
- [x] 5.6 Add tests for fail-closed initialization and reload failure preserving previous policy.
- [x] 5.7 Run `go test ./...` from `user-service/` and fix any failures.
