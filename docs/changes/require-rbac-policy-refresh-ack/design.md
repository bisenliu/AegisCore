# Design

## Overview

Online RBAC management writes must no longer hide policy refresh failures. The command path should persist the requested RBAC data mutation, synchronously invoke the permission feature policy refresh coordinator, and return an error if refresh acknowledgement fails.

The important contract change is:

```text
RBAC write succeeds in PostgreSQL
  -> local Casbin policy reload must succeed
  -> Redis policy version increment and Pub/Sub publish must succeed
  -> command returns success

Any refresh step fails
  -> command returns an error
  -> logs and metrics record the failure
  -> committed PostgreSQL data remains the source of truth
```

This keeps request-time authorization unchanged: protected requests still authorize only through the in-memory Casbin enforcer and do not read Redis.

## Current State

`permissionapplication.PolicyChangeNotifier` exposes `NotifyPolicyChanged(ctx, change)` without an error return. Permission and role command services call their local `notifyPolicyChanged` or `notifyUserRoleChanged` helpers after successful store mutations, then immediately return successful results.

`PolicyRefreshCoordinator.NotifyPolicyChanged` currently:

- Reloads local Casbin policy for policy-wide changes.
- Invalidates user-role cache for user-role changes.
- Publishes a Redis policy version and Pub/Sub message.
- Logs and records metrics on reload or publish failures.
- Returns without surfacing those failures to the caller.

Existing tests codify this best-effort behavior: reload failure skips publish, publish failure does not mark the version applied, and neither case can fail the command because the notifier returns no error.

## Goals / Non-Goals

**Goals:**

- Make policy refresh acknowledgement visible to online RBAC write commands.
- Fail the management API response when local reload, Redis version increment, or Redis publish fails.
- Preserve existing successful refresh ordering: local reload before distributed publication.
- Preserve policy version compensation for missed Pub/Sub messages.
- Keep metrics and logs as the primary operational signal for refresh failures.
- Update tests so reload and publish failures are asserted at both coordinator and command-service boundaries.

**Non-Goals:**

- No transactional rollback of committed PostgreSQL writes after refresh failure.
- No outbox, retry worker, Redis Stream, Kafka, RabbitMQ, NATS, or generic eventbus.
- No request-time Redis version gate in RBAC authorization middleware.
- No change to Casbin subject format, route template matching, permission catalog schema, or HTTP response envelope structure.
- No new shared package or common runtime primitive.

## Decisions

### Return errors from the policy change notifier

Change `PolicyChangeNotifier` to:

```go
type PolicyChangeNotifier interface {
    NotifyPolicyChanged(ctx context.Context, change PolicyChange) error
}
```

`PolicyRefreshCoordinator.NotifyPolicyChanged` should return wrapped errors when local reload or distributed publish fails. Metrics and English logs should remain at the failure sites, but the error return becomes the command-facing contract.

The coordinator still handles nil receiver as a no-op success, preserving optional wiring behavior in tests and Fx edge cases.

### Command helpers propagate refresh failures

Permission and role command helpers should return errors:

```go
func (s *permissionCommandService) notifyPolicyChanged(ctx context.Context, reason string) error
func (s *roleCommandService) notifyPolicyChanged(ctx context.Context, reason string) error
func (s *roleCommandService) notifyUserRoleChanged(ctx context.Context, reason string, userID uuid.UUID, roleID uuid.UUID) error
```

Every policy-affecting command should check the helper result after the successful store mutation. On failure, log an English message with stable fields where useful, then return an error to the caller.

Affected permission paths:

- Create permission.
- Update permission.
- Enable permission.
- Disable permission.

Affected role paths:

- Update role.
- Enable or disable role through active-state update.
- Add, replace, or remove user-role bindings.
- Add, replace, or remove role-permission bindings.

Role creation does not currently trigger policy refresh and does not need to change unless it becomes policy-affecting in a future change.

### Refresh failure maps to system failure

Refresh failures are operational/system failures rather than validation, conflict, or not-found failures. Existing HTTP error mapping can rely on the default `contracterrors.FromError(err)` behavior if it maps unknown wrapped errors to internal server error.

If the current default mapping is ambiguous, add a permission or role application/domain sentinel error that maps to internal server error without exposing low-level Redis, SQL, or Casbin details in the HTTP response. Logs may include wrapped errors and stack traces according to existing logger rules.

### Keep committed data and stale-policy risk explicit

The store mutation has already committed before refresh runs. This change intentionally does not try to roll back PostgreSQL because:

- Existing stores are not organized around a transaction that spans external Redis publication.
- A rollback after successful write but failed publication would require deeper consistency design.
- PostgreSQL remains the source of truth, and future refresh attempts can converge policy from it.

The user-visible difference is that callers no longer receive a false success. Operators can retry the management operation or trigger another online RBAC change once the dependency issue is resolved.

### Preserve existing Redis compensation semantics

When Redis version increment and publish succeed, other instances still rely on Pub/Sub and periodic version checks. Missed Pub/Sub messages remain compensated by the existing version mismatch check.

When Redis version increment fails, no newer version exists for other instances to detect. Returning an error to the API caller is therefore required for observability and safety.

When publish fails after a successful version increment, a newer version may still be visible to periodic checks. Returning an error is still required because the caller should not be told the change was fully synchronized.

## Error And Logging Guidance

- Wrap errors with operation context, for example `refresh rbac policy after permission_created`.
- Keep log messages in English.
- Use stable snake_case fields such as `reason`, `permission_id`, `role_id`, and `user_id`.
- Do not include tokens, Authorization headers, Cookies, raw request bodies, SQL, Redis keys, or full HTTP payloads in logs or metrics labels.
- Metrics labels must remain fixed low-cardinality enums.

## Test Plan

Coordinator tests:

- Success reloads locally, publishes policy version, marks tracker applied, and returns nil.
- Local reload failure returns an error, records reload failure metric, skips publish, and does not mark tracker applied.
- Publish failure returns an error, records publish failure metric, and does not mark tracker applied.
- User-role change invalidates the intended local cache, publishes, marks tracker applied, and returns nil.

Permission command tests:

- Successful create/update/enable/disable still trigger refresh and return results.
- Refresh failure after create/update/enable/disable returns an error.
- Validation and store failures still short-circuit before refresh.

Role command tests:

- Successful policy-affecting writes still trigger refresh or user-role invalidation notification.
- Refresh failure after role active-state, user-role binding, or role-permission binding mutations returns an error.
- Pre-mutation validation and store failures still short-circuit before refresh.

Validation:

- Run focused package tests for permission application and role application command packages.
- Run `make test-user-service` or `go test ./...` under `user-service/`.
- Run `make architecture-lint` if imports or boundaries change.
