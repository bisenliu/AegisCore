## 1. Casbin Engine 生命周期

- [x] 1.1 检查 `user-service/internal/features/permission/infrastructure/casbin` 中 `Engine` 的 lifecycle root 字段、`Start(ctx)` 和 `Stop(ctx)` 行为，确保 root context 由 engine 拥有且停止后不会再启动新的 reload flight。
- [x] 1.2 修改或确认 `startFlightLocked` 只通过 `context.WithCancel(e.lifecycleCtx)` 创建 shared flight context，并移除任何 `context.Background()` 或 waiter context 作为 shared flight root 的兼容路径。
- [x] 1.3 检查 `ReloadToRevision(ctx)` 和 `RefreshToRevision(ctx)` 的 waiter cancel 分支，确保单个 waiter 取消只移除该 waiter，仍有 waiter 时不取消 shared flight。
- [x] 1.4 检查全部 waiter 取消后的清理逻辑，确保当前 shared flight 被取消、engine 保持 fail-closed，后续新 waiter 可创建 fresh flight。

## 2. RBAC 装配与初始化

- [x] 2.1 检查 `user-service/internal/features/permission/fx_lifecycle.go` 的 RBAC lifecycle hook，确保启动顺序为 engine lifecycle root、`InitializeFailClosed(ctx)`、watcher、dispatcher。
- [x] 2.2 检查启动回滚和停止顺序，确保 dispatcher 停止后取消 runtime root，并停止 watcher 与 engine lifecycle，且不关闭共享 Redis、Ent 或 PostgreSQL 资源。
- [x] 2.3 确认 `InitializeFailClosed(ctx)` 仍调用 target revision 0，且初始化失败、取消或未达到目标 revision 时不阻断服务启动并保持 fail-closed。

## 3. 测试覆盖

- [x] 3.1 更新或补齐 Casbin engine 测试，覆盖一个 waiter 取消时其他 waiter 仍等待同一 shared reload 并成功完成。
- [x] 3.2 更新或补齐唯一 waiter 取消后 shared flight 取消、后续新 waiter 启动 fresh flight 并成功完成的测试。
- [x] 3.3 更新或补齐 engine lifecycle root 取消会取消正在阻塞 loader、记录失败并保持 fail-closed 的测试。
- [x] 3.4 更新或补齐并发 reload、强制 refresh、100 个并发 target coalescing 和 fault injection 收敛测试，确保最高 target 语义不倒退。
- [x] 3.5 更新或补齐 `InitializeFailClosed(ctx)` 使用 target revision 0 和 caller/startup context 取消语义的测试。
- [x] 3.6 更新或补齐 permission lifecycle wiring 测试，覆盖 engine、watcher、dispatcher 的启动、停止和启动失败回滚顺序。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./user-service/internal/features/permission/infrastructure/casbin -run 'TestEngineWaiterCancellation|TestEngineNewWaiter|TestEngineCoalesces|TestEngineFaultInjection|TestEngineInitialize'` 并修复失败。
- [x] 4.2 运行 permission lifecycle 相关测试，例如 `go test ./user-service/internal/features/permission -run 'TestRegisterRBACLifecycle|TestStopRBACLifecycle'` 并修复失败。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认 OpenSpec 和架构边界没有 drift。
- [x] 4.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [x] 4.5 运行 `make lint` 并修复失败。
- [x] 4.6 运行 `make verify` 并修复失败；若验证未运行或失败，不得标记本 change 完成。
