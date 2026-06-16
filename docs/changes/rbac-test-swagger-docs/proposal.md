## Why

RBAC integration is now part of the user service surface, but its regression coverage, generated API documentation, and architecture guidance need to be aligned with the implemented role, permission, Casbin, and HTTP authorization behavior. This change stabilizes the permission model and feature boundaries before further RBAC-dependent work builds on them.

## What Changes

- Add unit coverage for role command/query behavior, permission command/query behavior, permission route diff behavior, Casbin policy loading/enforcement/reload behavior, and Gin RBAC authorization middleware behavior.
- Cover RBAC edge cases including users with no roles, disabled roles, disabled permissions, user-role unbinding, role-permission unbinding, `super_admin` wildcard access, and Casbin policies that use `role_id` without requiring `roles.code`.
- Update Swagger annotations and generated Swagger artifacts so documented role, permission, and RBAC-protected interface behavior matches the current API.
- Update `AGENTS.md`, `docs/ARCHITECTURE.md`, and RBAC-related development/testing documentation so role and permission features are documented as implemented bounded features rather than skeletons.
- Preserve existing business behavior, database schema, module boundaries, and common module responsibilities.

## Capabilities

### New Capabilities
- `rbac-regression-readiness`: Covers regression requirements for RBAC tests, Swagger generation, and architecture/testing documentation needed before implementation and future RBAC-dependent changes.

### Modified Capabilities

## Impact

- Affected service code: `user-service/internal/features/role`, `user-service/internal/features/permission`, RBAC/Casbin authorization providers or middleware, route diff logic, and related tests.
- Affected docs and generated artifacts: Swagger source annotations/generated docs, `AGENTS.md`, `docs/ARCHITECTURE.md`, and `docs/DEVELOPMENT.md` or `docs/TESTING.md`.
- No database schema changes, no new business capabilities, no Redis multi-instance synchronization, no menu permissions, no multi-tenancy, no audit logging, and no expansion of `common/` responsibilities.
