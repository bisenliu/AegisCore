## 1. 接口与调用方更新

- [x] 1.1 将 permission application 中 `OutboxDispatcherRunner` 接口签名从 `Start() error` 调整为 `Start(ctx context.Context) error`，并移除无参 Start 兼容形态。
- [x] 1.2 更新所有编译期调用方，使 permission Fx lifecycle `OnStart(ctx)` 直接调用 `Runtime.Dispatcher.Start(ctx)`。
- [x] 1.3 更新 permission module lifecycle 相关 fake/mock，使测试替身记录并校验传入的 Start context。

## 2. Dispatcher 实现

- [x] 2.1 将 `Dispatcher.Start` 签名调整为 `Start(ctx context.Context) error`，并使用传入 ctx 派生后台运行 context 与保存 cancel。
- [x] 2.2 移除 dispatcher Start 路径中的 `context.WithCancel(context.Background())` 或等价运行根 context 创建。
- [x] 2.3 确保后台轮询 loop、结构化日志和 `DispatcherRunningObserved(true/false)` 使用 Start 派生的运行 context 或 logger-aware 派生 context。
- [x] 2.4 保持重复 `Start(ctx)` 幂等，不覆盖运行中实例的 cancel，不启动第二个 ticker 或 worker。
- [x] 2.5 保持 `Stop(ctx)` 只用于取消内部运行 context 后等待后台 loop 退出的期限控制，不关闭共享 Ent、PostgreSQL 或 Redis client。

## 3. 测试更新

- [x] 3.1 更新 dispatcher lifecycle 单元测试，覆盖 `Start(ctx)` 使用传入 lifecycle context 派生运行 context。
- [x] 3.2 更新 dispatcher 幂等测试，覆盖重复 `Start(ctx)` 不启动第二个 loop，重复 `Stop(ctx)` 稳定返回。
- [x] 3.3 更新 dispatcher metrics/log context 相关测试，覆盖 `DispatcherRunningObserved(true/false)` 使用运行 context。
- [x] 3.4 更新 permission module lifecycle 测试，覆盖 Fx `OnStart(ctx)` 将同一个 lifecycle ctx 传给 dispatcher runner。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./user-service/internal/features/permission/application ./user-service/internal/features/permission -run 'TestDispatcher|TestRegisterRBACLifecycle|TestPermissionModule'`，确认相关测试通过。
- [x] 4.2 运行代码搜索确认 dispatcher Start 路径不再出现 `context.WithCancel(context.Background())`，且无无参 `Start()` 兼容接口或 adapter 残留。
- [x] 4.3 如果实现触及架构边界、文档或 OpenSpec 主规格，运行 `make user-service-architecture-lint`。
- [x] 4.4 将本次预期代码、OpenSpec artifacts 和必要文档变更加到暂存区。
- [x] 4.5 运行 `make lint`，未通过时修复后重新运行。
- [x] 4.6 运行 `make verify`，未通过时修复后重新运行，并确认没有未暂存的预期变更导致 drift 检查失败。
