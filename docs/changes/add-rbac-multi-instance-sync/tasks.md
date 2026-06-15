## 1. Policy Sync Foundations

- [x] 1.1 Add a permission application-level policy refresh notifier/coordinator abstraction that can reload the local Casbin engine and notify distributed policy version changes.
- [x] 1.2 Add a feature-local Redis policy sync package under `user-service/internal/features/permission/infrastructure/redis` with key/channel naming, message DTOs, version parsing, and Redis client integration.
- [x] 1.3 Track the locally applied policy version in a concurrency-safe component so notification and periodic check paths can skip stale or duplicate versions.

## 2. Redis Version And Pub/Sub Flow

- [x] 2.1 Implement Redis policy version increment and Pub/Sub publish after successful local policy reload.
- [x] 2.2 Implement Redis Pub/Sub subscription that receives policy refresh messages and triggers full Casbin reload for newer versions.
- [x] 2.3 Implement periodic Redis policy version checks that detect newer Redis versions and trigger compensation reloads.
- [x] 2.4 Register watcher startup and shutdown through Fx lifecycle with context cancellation and clean Pub/Sub close handling.

## 3. Command Path Integration

- [x] 3.1 Wire permission command services to trigger policy refresh notification after successful permission create, update, enable, or disable operations that can affect loaded policy.
- [x] 3.2 Wire role command services to trigger policy refresh notification after successful role active-state updates, user-role binding changes, and role-permission binding changes.
- [x] 3.3 Ensure Redis coordination failures are logged but do not roll back successful RBAC data mutations or successful local policy reloads.

## 4. Observability And Safety

- [x] 4.1 Add English logs with stable snake_case fields for version increments, published messages, received messages, version mismatch detection, refresh success, refresh failure, and Redis coordination failures.
- [x] 4.2 Ensure request-time authorization paths do not import or call Redis policy sync components and still authorize only through the in-memory Casbin enforcer.
- [x] 4.3 Preserve UUID-based Casbin subjects (`user:<user_uuid>` and `role:<role_uuid>`) and avoid any dependency on `roles.code`.

## 5. Tests And Validation

- [x] 5.1 Add unit tests for policy refresh coordinator behavior, including local reload success, local reload failure, Redis increment failure, and publish failure.
- [x] 5.2 Add tests for Redis watcher behavior covering newer Pub/Sub versions, stale Pub/Sub versions, periodic version mismatch compensation, and reload failure preserving the last good policy.
- [x] 5.3 Add or update command service tests to verify policy refresh is triggered only after successful policy-affecting role and permission mutations.
- [x] 5.4 Run `go test ./...` under `user-service/` and fix any failures.
