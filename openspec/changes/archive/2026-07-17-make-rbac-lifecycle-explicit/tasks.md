## 1. 生命周期契约实现

- [x] 1.1 定位 `user-service/internal/features/permission/infrastructure` 中 initial load、RBAC watcher、user-role resolver/cache 的现有 Fx lifecycle 或构造副作用入口，确认替换范围。
- [x] 1.2 删除 `RegisterInitialLoad(fx.Lifecycle, ...)` 及 permission infrastructure 生产代码中的 Fx/Dig imports，提供由调用方显式执行的 initial load 初始化入口，并保留 reload 状态、readiness 可观测性和 fail-closed 语义。
- [x] 1.3 调整 RBAC watcher，使 `NewWatcher` 只构造对象，`Start()` 幂等启动内部可取消长期循环，`Stop(context.Context)` 幂等取消并在 deadline 内等待退出。
- [x] 1.4 调整 user-role resolver/cache，使启用和 disabled 模式都返回或实现幂等 `Close` 契约，并继续暴露 resolver 与 stats。
- [x] 1.5 确认 watcher/cache 的停止或关闭不会关闭调用方注入的 Redis、Ent 或 PostgreSQL 共享资源。

## 2. Composition 接线

- [x] 2.1 更新正式 permission/RBAC Fx module composition，在组合层登记 initial load 显式调用、watcher `Start`/`Stop` 和 cache `Close`。
- [x] 2.2 确认启动顺序为 initial load 优先于 watcher start，停止顺序能在服务关闭时停止 watcher 并关闭 cache。
- [x] 2.3 确认 composition 层只保留必要 Fx hook，不在 permission infrastructure 中保留旧 lifecycle adapter。

## 3. 测试覆盖

- [x] 3.1 补充 initial load 失败测试，证明失败会记录状态并使后续授权继续 fail-closed。
- [x] 3.2 补充 watcher 测试，证明 `NewWatcher` 构造时不启动 goroutine、`Start`/`Stop` 可重复调用且无非预期副作用。
- [x] 3.3 补充 `Stop(context.Context)` deadline 测试，证明超时返回 context 相关错误并保持后续重复停止安全。
- [x] 3.4 补充 user-role cache/resolver 测试，覆盖启用和 disabled 模式的重复 `Close`、stats 暴露和关闭后不关闭共享资源。
- [x] 3.5 运行 `cd user-service && go test ./internal/features/permission/... -count=1` 并修复失败。

## 4. 规格与架构验证

- [x] 4.1 运行 `rg -n 'go\.uber\.org/(fx|dig)|fx\.(Lifecycle|Hook|In|Out)' user-service/internal/features/permission/infrastructure --glob '*.go' --glob '!**/*_test.go'`，确认无输出。
- [x] 4.2 运行 `openspec validate make-rbac-lifecycle-explicit` 并修复规格问题。
- [x] 4.3 运行 `make user-service-architecture-lint` 并修复架构边界问题。

## 5. 收尾验证

- [x] 5.1 检查 diff，确认未修改数据库 schema、OpenAPI 生成物、HTTP API、部署资产或无关共享契约。
- [x] 5.2 暂存本次预期代码和文档变更，避免最终 verify 的 git diff 检查被预期变更阻塞。
- [x] 5.3 运行 `make lint` 并修复失败。
- [x] 5.4 运行 `make verify` 并修复失败。
