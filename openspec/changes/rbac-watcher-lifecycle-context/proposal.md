## Why

当前 RBAC policy Redis watcher 在 `Start()` 内部直接以 `context.Background()` 创建后台运行根上下文，导致后台消息处理、周期 revision check、日志和 metrics 无法统一继承 Fx lifecycle 的启动上下文。该行为削弱了 permission runtime 生命周期边界的一致性，也让 watcher 与 outbox dispatcher 的启动语义不一致。

## What Changes

- **BREAKING**：将 permission feature 内 `policyWatcherRunner` 启动接口从无参 `Start() error` 调整为 `Start(ctx context.Context) error`，不保留无参兼容分支或 adapter。
- 调整 Redis watcher 实现，使 `Watcher.Start(ctx)` 使用传入的 Fx lifecycle context 派生后台运行 context，并保存对应 cancel。
- 调整 watcher 后台消息处理、周期 revision check、日志和 metrics 上下文，使其统一来自 watcher 生命周期上下文或其 logger-aware 派生上下文。
- 调整 permission Fx lifecycle wiring，在 `OnStart(ctx)` 中直接调用 `Runtime.Watcher.Start(ctx)`。
- 更新 watcher lifecycle 单元测试、permission module lifecycle fake/mock，验证重复 start/stop、Stop 超时和订阅断开后周期补偿语义保持不变。
- 明确 `Stop(ctx)` 仍仅作为等待后台循环退出的期限控制，不替代 watcher 运行根上下文。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：收紧 RBAC policy watcher 启动生命周期契约，要求 watcher 后台运行上下文由 Fx lifecycle root context 派生，禁止在 Start 路径使用 `context.Background()` 作为运行根上下文。

## Impact

- 影响代码：`user-service/internal/features/permission` 中 watcher runner 接口与 Fx lifecycle wiring、`user-service/internal/features/permission/infrastructure/redis` 中 watcher 实现、相关测试 fake/mock。
- 影响行为：watcher 后台消息处理、周期 revision check、reload、日志和 metrics 将继承 `OnStart(ctx)` 传入的生命周期上下文；重复 `Start(ctx)` 不启动第二个 ticker，重复 `Stop(ctx)` 继续稳定返回。
- 不影响公开 HTTP API、OpenAPI、数据库 schema、Redis Pub/Sub subscriber 启动签名、Casbin enforcer reload flight 上下文模型、消息串行处理顺序、revision check 周期或缓存失效语义。
- 验证范围：`go test ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission -run 'TestWatcher|TestRegisterRBACLifecycle|TestPermissionModule'`。
