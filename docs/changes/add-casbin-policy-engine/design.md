## Context

The permission and role features already own normalized RBAC business tables: `roles`, `permissions`, `user_roles`, and `role_permissions`. Current permission queries can compute effective permissions through Ent/PostgreSQL, but there is no long-lived runtime authorization engine that evaluates access in memory for future request authorization middleware.

This change adds a Casbin-backed policy engine inside `user-service/internal/features/permission/infrastructure/casbin/`. The engine is an infrastructure adapter because it loads persisted RBAC state and exposes a runtime enforcement primitive; it is not an HTTP transport concern and does not alter route registration.

## Goals / Non-Goals

**Goals:**

- Embed the Casbin model with `go:embed` from `model.conf`.
- Construct a Casbin enforcer from normalized RBAC tables instead of `casbin_rules`.
- Load only active roles and active permissions into policy.
- Represent users as `user:<user_uuid>` and roles as `role:<role_uuid>` in Casbin subjects.
- Add a fixed super-admin role ID wildcard policy without depending on `roles.code`.
- Enforce access through `Enforce(ctx, userID, pathTemplate, method)` without database access per call.
- Support full `Reload` with atomic replacement semantics so a failed reload does not leave a partially updated policy.
- Fail closed when initialization cannot build a valid enforcer.

**Non-Goals:**

- No `casbin_rules` table or Casbin database adapter.
- No Gin middleware or router integration.
- No multi-instance Redis or event synchronization.
- No per-request database lookup.
- No schema migration for new RBAC authority tables.

## Decisions

1. Use the permission feature infrastructure package as the owner.

The new package belongs under `permission/infrastructure/casbin` because it is derived from permission and role persistence state and provides a permission runtime adapter. Keeping it under permission avoids a cross-feature shared package and preserves the existing feature-first structure.

Alternative considered: place the engine in `common/runtime`. That was rejected because the subject prefixes, RBAC table shape, super-admin role policy, and path-template semantics are user-service business semantics rather than cross-service primitives.

2. Load policies from normalized RBAC tables.

The loader reads active roles, active permissions, user-role bindings, and role-permission bindings through Ent queries. It creates grouping policies for active user-role bindings and permission policies for active role-permission bindings. It must not use `casbin_rules` because the existing RBAC tables remain the business authority.

Alternative considered: add Casbin's SQL adapter. That was rejected because it would duplicate authority, introduce synchronization problems, and conflict with the requirement to keep normalized RBAC tables as source of truth.

3. Build a fresh enforcer for initialization and reload.

Initialization and reload construct a new model and new in-memory enforcer, populate it completely, and validate the result before publishing it. Reload swaps the current enforcer only after all policy loading succeeds, so failures leave the previous enforcer intact.

Alternative considered: clear and repopulate the live enforcer. That was rejected because a mid-load failure could temporarily remove or partially replace authorization state.

4. Fail closed on unavailable policy.

If initial construction fails, the exported engine remains in a fail-closed state and `Enforce` returns denial rather than allowing access. This prevents startup or dependency failures from accidentally opening protected operations.

Alternative considered: fail open until policy is loaded. That was rejected for security reasons.

5. Use path templates and HTTP methods as policy object/action.

Policies use the permission table's `path_template` and `http_method` values directly. The future HTTP middleware must pass the registered route template rather than the raw URL path to match the permission catalog.

Alternative considered: teach Casbin model to match Gin path patterns. That was rejected for this change because route-template extraction belongs to future HTTP middleware integration, and the current scope avoids request routing changes.

## Risks / Trade-offs

- Initial load can be expensive as RBAC data grows -> keep load out of per-request path, use bounded queries with clear ordering where useful, and add unit tests around policy translation.
- Stale policies between RBAC changes and reload -> document that authorization reflects the latest successful reload; future middleware or admin flows can trigger reload explicitly.
- Super-admin wildcard depends on fixed role ID configuration -> validate configured role ID during provider construction and do not depend on role code fields that do not exist in the schema.
- Path-template mismatch can deny valid requests -> require callers to pass permission catalog route templates, not raw paths.
- Multi-instance deployments can diverge after local reload -> accept as out of scope; future distributed invalidation needs a separate design.

## Migration Plan

1. Add the Casbin dependency to `user-service/go.mod` if absent.
2. Add `permission/infrastructure/casbin/model.conf` and embedded model loader.
3. Add policy loader and enforcer types with tests for policy loading, active filtering, wildcard super-admin policy, reload atomicity, and fail-closed behavior.
4. Wire the provider in the permission feature Fx module without invoking Gin middleware or router changes.
5. Run `go test ./...` under `user-service/`.

Rollback is code-only: remove the provider/package and dependency if no caller has been integrated yet. Because no schema migration or router behavior is introduced, rollback does not require data migration.

## Open Questions

- The super-admin role ID should be supplied as configuration or a feature-local constant during implementation; it must be fixed and must not rely on `roles.code`.
