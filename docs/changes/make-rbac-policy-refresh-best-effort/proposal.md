## Why

Online RBAC write APIs currently persist permission, role, user-role, and role-permission changes before triggering policy refresh. If the local Casbin reload or Redis policy version/Pub/Sub notification fails after the PostgreSQL mutation has already committed, command services return the refresh error to the HTTP layer.

This creates a caller-visible consistency ambiguity: the client sees a failed operation even though the RBAC source data has already changed. Retrying the same request can then produce conflicts, not-found results, or confusing audit trails while operators still need to diagnose the refresh problem separately.

The project does not want to add a durable outbox table or any new persistence model for policy refresh in this change. Therefore the safest small correction is to make post-commit policy refresh best-effort from the API contract perspective: keep logs and metrics for failures, but do not reverse the business result once the database write has succeeded.

## What Changes

- Treat RBAC policy refresh and Redis policy notification failures after successful writes as operational side effects, not command return errors.
- Keep calling the existing policy refresh coordinator after all policy-affecting writes.
- Keep existing English error logs and metrics when local reload or Redis publish/version notification fails.
- Return successful command results when the PostgreSQL mutation succeeded, even if the post-write refresh notification failed.
- Apply the behavior consistently across permission writes, role active-state writes, user-role binding writes, and role-permission binding writes.
- Preserve validation, conflict, not-found, and store failure behavior before the database mutation succeeds.
- Do not add tables, migrations, outbox records, Redis Streams, MQ/eventbus, request-time Redis gates, or new HTTP response shapes.

## Capabilities

### Modified Capabilities

- RBAC online write APIs now report the durable business mutation result rather than the post-commit refresh side-effect result.
- RBAC policy refresh remains synchronous best-effort after writes, with observability through logs and metrics.

## Impact

- Affects permission command write paths under `user-service/internal/features/permission/application/command`.
- Affects role command write paths under `user-service/internal/features/role/application/command`.
- Affects tests that currently assert refresh failures are returned from command services.
- Does not change `PolicyRefreshCoordinator` internals unless implementation discovers a cleaner helper boundary.
- Does not change database schema, Redis key schema, Casbin subject format, route matching, response envelope, or authorization middleware request-time behavior.
