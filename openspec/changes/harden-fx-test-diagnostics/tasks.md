## 1. Fx 测试 logger 治理

- [x] 1.1 替换 `user-service/internal/features/role/fx_test.go` 中正向 `fxtest.New` 测试的 `fx.NopLogger`，保留默认测试 logger 并确认 start/stop 断言不变。
- [x] 1.2 替换 `user-service/internal/features/permission/fx_test.go` 中正向 `fxtest.New` 测试和 `newPermissionModuleTestApp` helper 的 `fx.NopLogger`，负向 `fx.New` 构图测试改用 `fxtest.WithTestLogger(t)`。
- [x] 1.3 替换 `user-service/internal/features/auth/fx_test.go` 中 auth module helper 的 `fx.NopLogger`，确认预期 `app.Err()` 的负向测试仍使用 `fx.New` 并配置测试 logger。
- [x] 1.4 替换 `user-service/internal/bootstrap/validation_test.go`、`http_test.go` 和 `lifecycle_test.go` 中不必要的 `fx.NopLogger`，保留需要断言 `app.Err()` 或 panic recovery 的测试形态。
- [x] 1.5 运行 `rg 'fx\.NopLogger' user-service/internal/features user-service/internal/bootstrap common/runtime/observability common/runtime/workerpool`，确认剩余使用点均有明确负向测试理由或已被移除。

## 2. 阻塞关闭路径硬超时

- [x] 2.1 为 `user-service/internal/features/permission/infrastructure/redis/watcher_test.go` 的 watcher stop deadline 测试增加 goroutine/select 测试级 guard，并保留重复 Stop 和共享资源不关闭断言。
- [x] 2.2 为 `user-service/internal/features/permission/fx_test.go` 中 RBAC watcher 启动失败回滚和 `stopRBACLifecycle` 错误聚合测试增加带 deadline 的 Start/Stop 调用或 goroutine/select guard。
- [x] 2.3 为 `user-service/internal/features/auth/infrastructure/redis/session_delete_all_test.go` 中 session purge pool drain、caller timeout 和重复停止路径增加测试级 guard，避免 worker task 或 Stop 阻塞测试进程。
- [x] 2.4 为 `user-service/internal/bootstrap/pprof_test.go` 和 `http_test.go` 中 pprof/HTTP shutdown、repeated stop、drain timeout 相关等待点补齐测试级 guard，确保 context 忽略时快速失败。
- [x] 2.5 为 `common/runtime/observability/tracing/provider_test.go` 中 tracing exporter start/shutdown 相关阻塞路径补齐带 deadline context 或测试级 guard。
- [x] 2.6 对仍使用 `fxtest.NewLifecycle` 且覆盖阻塞 hook 或 deadline 语义的测试启用 `fxtest.EnforceTimeout(true)`，不把该选项误用于 `fxtest.New` App 测试。

## 3. 目标验证

- [x] 3.1 运行 `go test ./user-service/internal/features/role ./user-service/internal/features/permission ./user-service/internal/features/auth ./user-service/internal/bootstrap`，确认 Fx 模块、bootstrap 和 lifecycle 测试通过。
- [x] 3.2 运行 `go test ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/auth/infrastructure/redis ./common/runtime/workerpool ./common/runtime/observability/tracing`，确认 watcher、worker pool 和 tracing timeout 测试通过。
- [x] 3.3 运行 `make user-service-architecture-lint`，确认测试变更未破坏架构边界。
- [x] 3.4 检查 `git diff`，确认没有 API、OpenAPI、Ent schema、migration、部署资产或生产运行时代码的非预期变更。

## 4. 最终门禁

- [x] 4.1 将本 change 的预期代码、规格和文档变更加到暂存区，避免最终 verify 的 diff 检查被预期变更阻塞。
- [x] 4.2 运行 `make lint` 并修复所有问题。
- [x] 4.3 运行 `make verify` 并确认完整验证通过且无生成物 drift。
- [x] 4.4 验证通过后更新本 `tasks.md` 对应 checkbox 为完成状态。
