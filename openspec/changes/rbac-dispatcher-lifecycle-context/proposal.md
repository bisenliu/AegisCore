## Why

当前 RBAC outbox dispatcher 在 `Start()` 内部直接以 `context.Background()` 创建后台运行根上下文，导致后台轮询、运行状态 metrics 与结构化日志无法统一继承 Fx lifecycle 的启动上下文。该行为削弱了生命周期边界的一致性，也让 dispatcher 的运行语义与 permission module 的 Fx wiring 不完全对齐。

## What Changes

- **BREAKING**：将 permission application 的 `OutboxDispatcherRunner` 启动接口从无参 `Start() error` 调整为 `Start(ctx context.Context) error`，不保留无参兼容分支或 adapter。
- 调整 dispatcher 实现，使 `Dispatcher.Start(ctx)` 使用传入的 Fx lifecycle context 派生后台运行 context，并保存对应 cancel。
- 调整 dispatcher 后台轮询、运行状态 metrics 与日志上下文，使其统一来自 dispatcher 生命周期上下文或其 logger-aware 派生上下文。
- 调整 permission Fx lifecycle wiring，在 `OnStart(ctx)` 中直接调用 `Runtime.Dispatcher.Start(ctx)`。
- 更新 dispatcher lifecycle 单元测试、permission module lifecycle mock/fake，验证重复 start/stop 幂等语义保持不变。
- 明确 `Stop(ctx)` 仍仅作为等待后台循环退出的期限控制，不替代 dispatcher 运行根上下文。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：收紧 RBAC outbox dispatcher 启动生命周期契约，要求 dispatcher 后台运行上下文由 Fx lifecycle root context 派生，禁止在 Start 路径使用 `context.Background()` 作为运行根上下文。

## Impact

- 影响代码：`user-service/internal/features/permission/application` 中 dispatcher runner 接口与 dispatcher 实现、permission feature Fx lifecycle wiring、相关测试 fake/mock。
- 影响行为：dispatcher 后台轮询、运行状态 metrics 和日志上下文将继承 `OnStart(ctx)` 传入的生命周期上下文；重复 `Start(ctx)` 不启动第二个 ticker，重复 `Stop(ctx)` 继续稳定返回。
- 不影响公开 HTTP API、OpenAPI、数据库 schema、outbox claim/Ack/Fail 持久化语义、轮询间隔、batch size、claim timeout、retry backoff 或 Redis watcher/subscriber/Casbin enforcer 上下文模型。
- 验证范围：`go test ./user-service/internal/features/permission/application ./user-service/internal/features/permission -run 'TestDispatcher|TestRegisterRBACLifecycle|TestPermissionModule'`。
