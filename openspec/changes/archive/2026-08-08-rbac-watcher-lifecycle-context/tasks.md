## 1. 接口与调用方更新

- [x] 1.1 将 permission feature 内 `policyWatcherRunner` 接口签名从 `Start() error` 调整为 `Start(ctx context.Context) error`，并移除无参 Start 兼容形态。
- [x] 1.2 更新所有编译期调用方，使 permission Fx lifecycle `OnStart(ctx)` 直接调用 `Runtime.Watcher.Start(ctx)`。
- [x] 1.3 更新 permission module lifecycle 相关 fake/mock，使测试替身记录并校验传入的 Start context。

## 2. Watcher 实现

- [x] 2.1 将 Redis watcher `Start` 签名调整为 `Start(ctx context.Context) error`，并使用传入 ctx 派生后台运行 context 与保存 cancel。
- [x] 2.2 移除 watcher Start 路径中的 `context.WithCancel(context.Background())` 或等价运行根 context 创建。
- [x] 2.3 确保后台消息处理、周期 revision check、结构化日志和 watcher metrics 使用 Start 派生的运行 context 或 logger-aware 派生 context。
- [x] 2.4 保持重复 `Start(ctx)` 幂等，不覆盖运行中实例的 cancel，不启动第二个 ticker 或 worker。
- [x] 2.5 保持 `Stop(ctx)` 只用于取消内部运行 context 后等待后台 loop 退出的期限控制，不关闭共享 Redis client。

## 3. 测试更新

- [x] 3.1 更新 watcher lifecycle 单元测试，覆盖 `Start(ctx)` 使用传入 lifecycle context 派生运行 context。
- [x] 3.2 更新 watcher 幂等测试，覆盖重复 `Start(ctx)` 不启动第二个 loop，重复 `Stop(ctx)` 稳定返回。
- [x] 3.3 更新 Stop 超时和订阅断开后周期补偿相关测试，确保既有行为保持。
- [x] 3.4 更新 permission module lifecycle 测试，覆盖 Fx `OnStart(ctx)` 将同一个 lifecycle ctx 传给 watcher runner。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission -run 'TestWatcher|TestRegisterRBACLifecycle|TestPermissionModule'`，确认相关测试通过。
- [x] 4.2 运行代码搜索确认 watcher Start 路径不再出现 `context.WithCancel(context.Background())`，且无无参 `Start()` 兼容接口或 adapter 残留。
- [x] 4.3 如 OpenSpec 或架构文档变更触发架构检查，运行 `make user-service-architecture-lint`。
