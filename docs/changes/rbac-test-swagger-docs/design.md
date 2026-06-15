## Context

Role and permission features are implemented service features with HTTP endpoints, application command/query use cases, PostgreSQL adapters, and RBAC authorization integration through Casbin. The repository guidance and generated API documentation must now reflect the implemented RBAC model rather than treating role and permission as placeholder skeletons.

The change is intentionally regression-focused. It validates existing RBAC semantics, generated documentation, and architectural documentation without changing tables, adding capabilities, or moving responsibilities into `common/`.

## Goals / Non-Goals

**Goals:**

- Add focused unit tests for role and permission command/query behavior, permission route diff behavior, Casbin policy loading/enforcement/reload behavior, and Gin RBAC authorization middleware behavior.
- Cover RBAC negative and lifecycle scenarios, including missing user roles, disabled roles, disabled permissions, user-role unbinding, role-permission unbinding, and `super_admin` wildcard authorization.
- Prove Casbin policies use `role_id` and do not require `roles.code` to be present in role query results.
- Regenerate Swagger artifacts so API docs match current role, permission, and authorization behavior.
- Update architecture and development/testing docs to describe role and permission feature boundaries accurately.

**Non-Goals:**

- No new business endpoints, permissions, roles, policies, or authorization modes.
- No schema, migration, Ent schema, or persisted data changes.
- No Redis multi-instance synchronization, menu permissions, multi-tenancy, audit logging, eventing, or outbox work.
- No expansion of `common/` beyond existing cross-service primitives and contracts.

## Decisions

- Keep tests close to owning packages. Role use case tests remain under `internal/features/role`, permission use case and route diff tests remain under `internal/features/permission`, and RBAC middleware/Casbin tests remain with the authorization provider or middleware package that owns the behavior. This preserves the feature boundaries documented in `AGENTS.md` and avoids cross-feature test fixtures becoming informal shared abstractions.
- Use fakes or in-memory collaborators for application command/query tests. This exercises application behavior without importing Gin, Ent, Redis, or SQL into application packages, and makes the layer boundary check explicit.
- Cover authorization through HTTP middleware tests at the Gin boundary. Middleware tests should assert HTTP status and response envelope behavior while keeping Casbin and store behavior controlled by test doubles or deterministic test policies.
- Treat permission route diff as read-only comparison logic. Tests must prove automatic route discovery reports differences only and does not create formal permissions or bind roles.
- Regenerate Swagger after annotation updates instead of hand-editing generated docs. This keeps the generated files reproducible and aligned with the documented `make swagger-generate` workflow.
- Update docs in the same change as tests and Swagger. The acceptance criteria depend on developers understanding that role and permission are implemented bounded features, that policies use `role_id`, and that `roles.code` is not a required authorization field.

## Risks / Trade-offs

- Test fixtures may accidentally encode implementation details instead of externally observable behavior -> Mitigate by naming scenarios after RBAC outcomes and keeping assertions focused on command/query outputs, authorization decisions, and response status/envelope.
- Middleware tests may become brittle if they depend on global route registration order -> Mitigate by building minimal Gin engines in tests with explicit protected routes.
- Swagger generation may modify broad generated files -> Mitigate by running the documented generator once after annotation updates and reviewing generated diffs for unrelated churn.
- Documentation may drift if it describes future RBAC capabilities -> Mitigate by documenting only implemented role, permission, route diff, and Casbin authorization boundaries, and explicitly preserving non-goals.

## Migration Plan

No runtime migration is required. The implementation can be merged after tests, Swagger generation, architecture documentation, and development/testing documentation are updated and the acceptance commands pass.

Rollback is a normal code revert because the change adds regression coverage and documentation without schema or runtime data migration.

## Open Questions

- None. The provided scope and non-goals are sufficient for implementation.
