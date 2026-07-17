## Why

当前 permission infrastructure 的 Casbin initial load、RBAC watcher 和 user-role localcache 仍通过构造器或 infrastructure 内部绑定 Fx lifecycle，导致对象构造、资源启动和资源释放边界耦合。`remove-fx-from-permission-adapters` 完成后，需要把这些运行期资源统一改为显式生命周期契约，使 fail-closed 行为、启动顺序、停止超时和资源所有权可以由 composition 层清晰编排和测试。

## What Changes

- **BREAKING** 删除 permission infrastructure 的 `RegisterInitialLoad(fx.Lifecycle, ...)` 入口，改为可由调用方直接执行的 initial load 初始化方法；初始化失败仍记录 reload 状态、readiness 可观测性和 fail-closed 授权语义。
- **BREAKING** 调整 RBAC watcher 构造语义：`NewWatcher` 只构造对象，不启动 goroutine；调用方必须显式调用幂等 `Start` 与 `Stop(context.Context)` 管理长期循环。
- **BREAKING** 调整 user-role resolver/cache 生命周期：启用和 disabled 模式均必须返回或实现幂等 `Close` 契约，同时继续暴露 resolver 与 stats。
- 修改 Fx composition：只负责登记和编排显式 `Initialize`、`Start`、`Stop`、`Close` 方法，不保留 permission infrastructure 内的 Fx lifecycle adapter。
- 补充启动失败、Stop timeout、重复启停、重复关闭和 fail-closed 测试，证明构造与启动分离后行为不退化。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确 RBAC policy initial load、watcher 和 user-role localcache 的显式生命周期契约、启动顺序、fail-closed 语义和资源所有权。

## Impact

- 影响 `user-service/internal/features/permission/infrastructure` 中 policy loader/engine initial load、policy watcher、user-role resolver/cache 相关构造器和生命周期实现。
- 影响 user-service 的 permission/RBAC Fx module composition，Fx lifecycle hook 只能留在组合层登记显式方法，不能下沉到 permission infrastructure。
- 不改变权限、角色、用户角色、Casbin policy 派生规则、Pub/Sub payload、Redis policy version 补偿、readiness 定义或 metrics 标签。
- 不改变数据库 schema、OpenAPI、HTTP API、部署资产或共享 `common` 契约。
- 验证影响包括 permission 包测试、OpenSpec 校验、架构 lint、仓库 lint 和 verify。
