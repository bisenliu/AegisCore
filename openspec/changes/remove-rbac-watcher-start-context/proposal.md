## Why

RBAC policy watcher 的 `Start(context.Context)` 当前不使用传入的 Fx `OnStart` context，而是创建内部 `context.WithCancel(context.Background())` 并由 `Stop` 统一关闭后台循环。这个设计本身可以正确关闭 goroutine，但方法签名会误导维护者以为启动 context 参与长期生命周期控制。

## What Changes

- **BREAKING** 移除 `permission/infrastructure/redis.Watcher.Start` 的 `context.Context` 入参，明确 watcher 后台循环只由内部 cancel 和 `Stop(ctx)` 管理。
- 更新 Fx lifecycle hook，使 `OnStart` 明确丢弃启动 context，仅触发 watcher 启动。
- 更新 watcher 相关测试，覆盖 `Start()` 无参调用和 `Stop(ctx)` 关闭语义。
- 不保留兼容包装方法，不新增旧签名适配层。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `rbac-access-control`: 明确 Redis policy watcher 的启动与停止生命周期契约，避免将 Fx `OnStart` context 误用为长期后台循环的控制信号。

## Impact

- 影响代码：`user-service/internal/features/permission/infrastructure/redis/watcher.go` 及其测试。
- 影响能力：RBAC policy sync 的 watcher 生命周期表达更清晰，运行时关闭行为保持由 `Stop(ctx)` 驱动。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署资产、观测指标名称或外部共享契约。
