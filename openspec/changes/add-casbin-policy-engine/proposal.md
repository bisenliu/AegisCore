## Why

Permission and role data already exists as normalized RBAC business tables, but the service does not yet expose a runtime authorization engine that can evaluate a user's effective access without querying storage on each request. This change introduces an in-memory Casbin policy engine owned by the permission feature so later HTTP authorization middleware can reuse a fail-closed, reloadable enforcement boundary.

## What Changes

- Add a permission feature infrastructure package for Casbin policy enforcement.
- Embed a Casbin RBAC model from `permission/infrastructure/casbin/model.conf` using `go:embed`.
- Build an Enforcer from active roles, active permissions, user-role bindings, and role-permission bindings.
- Load user-role bindings as `g(user:<user_uuid>, role:<role_uuid>)` policies.
- Load role-permission bindings as `p(role:<role_uuid>, <path_template>, <http_method>)` policies.
- Support a configured fixed super-admin role ID with wildcard policy `p(role:<super_admin_role_uuid>, *, *)`.
- Provide `Enforce(ctx, userID, pathTemplate, method)` without database access per request.
- Provide full `Reload` that preserves the current policy if rebuilding the new policy fails.
- Default initialization failures to fail-closed behavior.
- Do not add `casbin_rules`, Gin middleware, router changes, multi-instance sync, or per-request database lookups.

## Capabilities

### New Capabilities
- `casbin-policy-engine`: Defines the permission feature runtime policy engine that loads normalized RBAC state into an embedded Casbin model and evaluates user access in memory.

### Modified Capabilities

## Impact

- Affected code: `user-service/internal/features/permission/infrastructure/casbin/` and permission feature composition where the new infrastructure provider is wired.
- Affected dependencies: adds Casbin v2 Go dependency if not already present.
- Affected runtime behavior: authorization policy can be loaded from existing RBAC tables and enforced in memory; initialization failure must deny access by default.
- Out of scope: persistence schema changes, `casbin_rules`, HTTP middleware integration, router modification, Redis synchronization, and request-time database reads.
