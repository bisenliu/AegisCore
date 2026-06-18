# Design

## Overview

RBAC management commands should distinguish between the durable data mutation and the post-mutation policy refresh side effect.

The target contract is:

```text
RBAC write validation/store mutation fails
  -> command returns an error
  -> HTTP returns failure

RBAC write commits successfully
  -> command attempts local policy refresh and Redis policy notification
  -> refresh/notification success or failure is logged and recorded in metrics
  -> command returns the business result
```

This makes API responses match PostgreSQL source-of-truth state while retaining the existing short eventual-consistency model for in-memory Casbin policy.

## Current State

`PolicyRefreshCoordinator.NotifyPolicyChanged` returns an error when local reload fails or Redis policy version/Pub/Sub publication fails. Permission and role command services call `notifyPolicyChanged` or `notifyUserRoleChanged` after successful store mutations and currently return those errors to callers.

The affected write paths include:

- Permission create.
- Permission update.
- Permission enable or disable.
- Role update.
- Role enable or disable through active-state update.
- User-role add, replace, and remove.
- Role-permission add, replace, and remove.

In these paths the store mutation has already succeeded before the notification helper runs. Some stores use single-statement auto-commit writes, while replace operations explicitly commit an Ent transaction before returning to the command layer.

## Goals / Non-Goals

**Goals:**

- Stop returning post-write refresh failures from RBAC command services.
- Keep local reload and Redis notification attempts on every existing policy-affecting write path.
- Keep current logging and metrics on refresh failures.
- Make all same-class RBAC write paths follow the same API success semantics.
- Update tests to assert that post-write refresh failures are swallowed after the store mutation succeeds.
- Preserve pre-write validation and store mutation errors exactly as command failures.

**Non-Goals:**

- No durable outbox, pending-change table, migration, or new Ent schema.
- No PostgreSQL policy version table.
- No Redis Stream, Kafka, RabbitMQ, NATS, generic eventbus, or background retry worker.
- No rollback attempt after a committed RBAC source-data mutation.
- No request-time Redis version check in RBAC authorization middleware.
- No change to Casbin policy loading, subject schema, route template matching, or HTTP envelope format.

## Decisions

### Keep notifier errors internal to command helpers

The simplest implementation is to introduce helper methods that log notification failures and return no error to command callers.

Example shape:

```go
func (s *roleCommandService) notifyPolicyChangedBestEffort(ctx context.Context, reason string, fields ...zap.Field) {
    if err := s.notifyPolicyChanged(ctx, reason); err != nil {
        logger.Error(ctx, "refresh rbac policy after role change failed", logger.StackTrace(append(fields, zap.Error(err))...)...)
    }
}
```

The exact helper shape can remain feature-local. The important behavior is that command methods do not return the refresh error once their store mutation has already succeeded.

### Preserve coordinator failure semantics

`PolicyRefreshCoordinator.NotifyPolicyChanged` may continue returning errors. That return value remains useful for tests, future callers that require acknowledgement, and local observability. This change only alters the online command service contract after successful store writes.

If implementation finds duplicated logging easier to remove by adding a feature-local best-effort adapter or wrapper, that wrapper should stay inside the permission/role application boundary and must not introduce shared runtime primitives.

### Apply the rule to all same-class RBAC write paths

The change must cover:

- `CreatePermission`, `UpdatePermission`, `EnablePermission`, `DisablePermission`.
- `UpdateRole`, `SetRoleActive`.
- `AddUserRole`, `ReplaceUserRoles`, `RemoveUserRole`.
- `AddRolePermission`, `ReplaceRolePermissions`, `RemoveRolePermission`.

Role creation stays unchanged unless implementation discovers it already triggers a policy refresh or has a real policy-affecting side effect.

### Keep refresh failures observable

Failure handling must keep English logs with stable fields such as `reason`, `permission_id`, `role_id`, `user_id`, and `active` where applicable.

Metrics currently emitted by the policy refresh coordinator should remain the primary low-cardinality signal for reload or publish failures. Do not add entity IDs, policy versions, Redis keys, SQL, raw paths, or raw error strings as metric labels.

### Accept bounded eventual consistency

Without adding a table, durable outbox, or PostgreSQL policy version, the system cannot guarantee that every failed refresh will be retried to completion. This change intentionally chooses clearer API semantics over strong refresh durability.

Operationally:

- If local reload fails, the writing instance may use stale policy until another successful reload path occurs.
- If Redis version increment fails, other instances cannot detect that change through the existing version-compensation mechanism.
- If Redis publish fails after version increment succeeds, periodic version checks can still compensate.

These risks already exist after a committed write; this change makes them observable without falsely reporting the business mutation as failed.

## Error And Logging Guidance

- Keep log messages in English.
- Use stable snake_case fields.
- Log refresh failures at error level because they indicate an operational dependency or policy reload problem.
- Do not include passwords, tokens, Authorization headers, Cookies, raw request bodies, SQL, Redis keys, or full HTTP payloads.
- Do not expose low-level Redis, Casbin, or SQL details in HTTP responses. Because commands return success after committed writes, refresh errors should not reach response mapping on these paths.

## Test Plan

Permission command tests:

- Refresh failure after create returns the created permission result.
- Refresh failure after update returns the updated permission result.
- Refresh failure after enable or disable returns the updated permission result.
- Validation failures, duplicate conflicts, not-found failures, and store failures still return errors and do not trigger refresh.

Role command tests:

- Refresh failure after role update returns the updated role result.
- Refresh failure after role active-state change returns the updated role result.
- User-role notification failure after add, replace, or remove returns the role list result.
- Role-permission refresh failure after add, replace, or remove returns the permission list result.
- Pre-mutation validation and store failures still return errors and do not trigger refresh.

Validation:

- Run focused tests for permission command and role command packages.
- Run broader `make test-user-service` if implementation touches shared interfaces or coordinator behavior.
- Run `make architecture-lint` if imports or package boundaries change.
