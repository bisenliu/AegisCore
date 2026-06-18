## Why

RBAC write APIs currently persist permission, role, user-role, and role-permission changes before triggering policy refresh as a best-effort side effect. If local Casbin reload or Redis policy version publishing fails, the failure is only logged and recorded in metrics while the management API can still return success.

This creates a safety and observability gap: callers may believe an authorization-changing operation is active across the service, while the writing instance or other replicas can continue authorizing with stale in-memory policy. Redis Pub/Sub plus periodic version checks compensate for missed messages only when a newer Redis policy version exists; they do not cover local reload failures or Redis version increment failures that are swallowed before callers can react.

## What Changes

- Make RBAC policy refresh notification return an error to command services instead of being fire-and-forget.
- Treat local policy reload failure as a write-path failure for online RBAC management operations.
- Treat Redis policy version increment or publish failure as a write-path failure so the API response does not falsely claim the change is fully applied.
- Keep PostgreSQL as the source of truth; do not roll back already committed RBAC rows in this change.
- Preserve existing local Casbin reload and Redis policy version/Pub/Sub synchronization flow when refresh succeeds.
- Preserve existing metrics and English logs, and add/adjust tests so refresh failure propagation is explicitly covered.
- Apply the contract consistently to permission writes, role active-state writes, user-role binding writes, and role-permission binding writes.

## Capabilities

### New Capabilities

- `rbac-policy-refresh-ack`: Online RBAC write operations surface local reload and distributed policy notification failures to callers instead of silently accepting stale authorization risk.

### Modified Capabilities

- RBAC multi-instance synchronization becomes command-visible for online management writes.
- Permission and role command services now require successful policy refresh acknowledgement after policy-affecting data mutations.

## Impact

- Affects `user-service/internal/features/permission/application/policy_sync.go`.
- Affects permission command helpers and write use cases under `user-service/internal/features/permission/application/command`.
- Affects role command helpers and write use cases under `user-service/internal/features/role/application/command`.
- Affects command service tests and policy refresh coordinator tests.
- Does not add OpenSpec/OPSX artifacts, database tables, Redis Streams, outbox, generic eventbus, request-time Redis authorization, or new HTTP response shapes.
