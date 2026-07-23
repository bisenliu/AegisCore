## Why

当前 `registerRBACLifecycle` 注入 `UserRoleCacheCloser`，再通过 type assertion 判断对象是否额外具备 `Start(context.Context) error`。这会把启动能力隐藏在关闭接口中，使 RBAC user-role resolver 的生命周期语义不准确，也降低了 Fx 装配和测试对启动失败路径的可见性。

## What Changes

- 将 permission user role resolver 的生命周期依赖改为显式接口，统一表达 `Start(context.Context) error` 与 `Close() error`。
- 调整 `UserRoleResolverResult`，除 resolver 和 stats 外显式输出 lifecycle 视图。
- 调整 `RegisterRBACLifecycleParams`，直接依赖显式 resolver lifecycle，移除对 closer 的启动能力 type assertion。
- 调整 RBAC lifecycle 启动顺序，使 user-role resolver start 失败时直接阻止后续 policy 初始化和 watcher 启动。
- 更新相关 Fx 测试，覆盖显式 lifecycle 注入、启动失败和停止清理语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确 RBAC user-role resolver/cache lifecycle 必须通过显式接口装配和启动，避免依赖关闭接口中的隐式启动能力。

## Impact

- 影响代码：`user-service/internal/features/permission/fx_authorization.go`、`user-service/internal/features/permission/fx_lifecycle.go`、`user-service/internal/features/permission/fx_test.go`。
- 不影响公开 HTTP API、OpenAPI、数据库 schema、migration、部署资产或共享 `common` 契约。
- 影响 RBAC 启停行为的内部装配契约和测试覆盖；启动失败路径将更明确地 fail-fast 于 Fx lifecycle hook。
