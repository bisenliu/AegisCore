## Why

当前 permission Casbin Engine 的共享 reload flight 使用 `context.Background()` 作为根 context，单个等待方取消不会影响共享 reload 的既有语义虽然正确，但服务关闭时无法通过 RBAC/engine 生命周期根 context 取消正在阻塞的 loader。这会增加 shutdown、trace 关联和资源泄漏排查成本。

本变更将共享 reload flight 的根 context 明确收敛到 engine lifecycle root，使服务生命周期可以控制后台 reload，同时继续保留 waiter context 只控制等待行为的语义边界。

## What Changes

- 为 permission Casbin Engine 增加明确的 lifecycle root context 管理方式，并在 RBAC lifecycle 启动时注入或启动该 root。
- 修改 `startFlightLocked`，使 shared flight context 从 engine lifecycle root 派生，而不是从 `context.Background()` 派生。
- 明确 `ReloadToRevision(ctx)` 等调用方的 waiter context 只控制当前调用等待，不作为 shared flight root，也不能取消其他 waiter 共享的 reload 工作。
- 增加 engine lifecycle root 取消时取消正在执行的 shared reload loader 的能力。
- 更新 Casbin engine 并发 reload、waiter cancel、sole waiter cancel、初始化和 fault injection 相关测试。
- 移除 background-only flight 兼容路径，不保留旧 root 策略分支。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 调整 Casbin policy reload 主动资源生命周期要求，要求 shared reload flight 由 engine lifecycle root 控制，并保持 waiter context 与 shared flight context 的职责隔离。

## Impact

- 影响代码：`user-service/internal/features/permission/infrastructure/casbin` 下的 engine lifecycle、reload flight 和相关测试。
- 影响装配：permission/RBAC provider 或 Fx lifecycle wiring 需要在 RBAC runtime 启动和停止时管理 engine lifecycle root。
- 影响测试：需要覆盖 waiter 取消不取消共享 reload、唯一 waiter 取消不取消共享 reload、engine lifecycle root 取消会取消 loader、初始化仍使用目标 revision 0 且失败时保持 fail-closed。
- 不影响 HTTP API、OpenAPI、数据库 schema、Casbin model、policy loader 查询语义、permission rule 映射、user-role resolver cache 策略或 Enforce fail-closed 行为。
