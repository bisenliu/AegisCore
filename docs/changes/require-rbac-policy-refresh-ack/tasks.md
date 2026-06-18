## 1. Policy Refresh Contract

- [x] 1.1 Change `permissionapplication.PolicyChangeNotifier.NotifyPolicyChanged` to return `error`.
- [x] 1.2 Update `PolicyRefreshCoordinator.NotifyPolicyChanged` to return nil on success and wrapped errors on local reload or Redis publish failure.
- [x] 1.3 Preserve existing reload, invalidate, publish, tracker, metrics, and log ordering on successful refresh.
- [x] 1.4 Preserve nil coordinator/no-op behavior as a successful notification.

## 2. Permission Command Integration

- [x] 2.1 Change permission command `notifyPolicyChanged` helper to return an error.
- [x] 2.2 Propagate refresh failures from `CreatePermission` after successful store create.
- [x] 2.3 Propagate refresh failures from `UpdatePermission` after successful store update.
- [x] 2.4 Propagate refresh failures from enable and disable permission after successful active-state update.
- [x] 2.5 Ensure validation failures, duplicate conflicts, not-found failures, and store failures still short-circuit before refresh.

## 3. Role Command Integration

- [x] 3.1 Change role command `notifyPolicyChanged` and `notifyUserRoleChanged` helpers to return errors.
- [x] 3.2 Propagate refresh failures from role active-state updates after successful store mutation.
- [x] 3.3 Propagate user-role notification failures from add, replace, and remove user-role binding writes.
- [x] 3.4 Propagate policy refresh failures from add, replace, and remove role-permission binding writes.
- [x] 3.5 Keep role creation unchanged unless implementation discovers a real policy-affecting refresh requirement.

## 4. Error Mapping And Observability

- [x] 4.1 Verify wrapped refresh errors map to internal server error through existing HTTP error mapping.
- [x] 4.2 Add a stable sentinel error and mapper branch only if default mapping does not produce the intended internal server error.
- [x] 4.3 Keep failure logs in English with stable snake_case fields and without sensitive values.
- [x] 4.4 Keep metrics labels fixed and low-cardinality; do not add entity IDs, policy versions, Redis keys, SQL, or raw error strings as labels.

## 5. Tests

- [x] 5.1 Update policy refresh coordinator tests to assert returned errors on reload and publish failures.
- [x] 5.2 Update permission command tests to cover refresh failure propagation for create, update, enable, and disable.
- [x] 5.3 Update role command tests to cover refresh failure propagation for role active-state, user-role binding, and role-permission binding writes.
- [x] 5.4 Ensure existing short-circuit tests still prove failed validation or store mutations do not trigger refresh.

## 6. Validation

- [x] 6.1 Run focused tests for `user-service/internal/features/permission/application` and permission command packages.
- [x] 6.2 Run focused tests for `user-service/internal/features/role/application/command`.
- [x] 6.3 Run `make test-user-service`.
- [x] 6.4 Run `make architecture-lint` if imports, interfaces, or package boundaries change.
