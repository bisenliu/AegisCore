## 1. Permission Command Integration

- [x] 1.1 Adjust permission command post-write refresh handling so `CreatePermission` logs refresh failure but still returns the created permission result.
- [x] 1.2 Adjust `UpdatePermission` so refresh failure is logged but the updated permission result is returned.
- [x] 1.3 Adjust `EnablePermission` and `DisablePermission` so refresh failure is logged but the updated permission result is returned.
- [x] 1.4 Preserve validation, duplicate, not-found, and store failure short-circuit behavior before refresh is attempted.

## 2. Role Command Integration

- [x] 2.1 Adjust `UpdateRole` so refresh failure is logged but the updated role result is returned.
- [x] 2.2 Adjust `SetRoleActive` so refresh failure is logged but the updated role result is returned.
- [x] 2.3 Adjust `AddUserRole`, `ReplaceUserRoles`, and `RemoveUserRole` so user-role notification failure is logged but the role list result is returned.
- [x] 2.4 Adjust `AddRolePermission`, `ReplaceRolePermissions`, and `RemoveRolePermission` so policy refresh failure is logged but the permission list result is returned.
- [x] 2.5 Keep `CreateRole` unchanged unless implementation discovers an existing policy-refresh call.

## 3. Observability And Error Semantics

- [x] 3.1 Keep existing refresh failure logs in English with stable snake_case fields.
- [x] 3.2 Keep existing policy reload and publish failure metrics emitted by the permission refresh coordinator.
- [x] 3.3 Ensure refresh errors no longer reach HTTP error mapping for successful post-write RBAC command paths.
- [x] 3.4 Do not add tables, migrations, outbox records, Redis Streams, MQ/eventbus, background retry workers, or request-time Redis checks.

## 4. Tests

- [x] 4.1 Update permission command tests so notifier errors after successful create/update/enable/disable no longer fail the command result.
- [x] 4.2 Update role command tests so notifier errors after successful role update/active-state changes no longer fail the command result.
- [x] 4.3 Update role binding tests so notifier errors after successful user-role and role-permission mutations no longer fail the command result.
- [x] 4.4 Keep or add tests proving validation and store failures still short-circuit before refresh.

## 5. Validation

- [x] 5.1 Run focused permission command tests.
- [x] 5.2 Run focused role command tests.
- [x] 5.3 Run `make test-user-service` if implementation changes shared interfaces, coordinator behavior, or package boundaries.
- [x] 5.4 Run `make architecture-lint` if imports or boundaries change.
