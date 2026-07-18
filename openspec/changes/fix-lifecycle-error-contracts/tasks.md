## 1. Tracing Lifecycle

- [x] 1.1 重构 `common/runtime/observability/tracing.Provider`，支持 constructor 阶段创建未启动 provider，并在 `OnStart(ctx)` 中安装真实 SDK `TracerProvider`。
- [x] 1.2 调整 `common/runtime/observability/tracing/fx.go`，删除 constructor 阶段 exporter 初始化，将 exporter 创建迁移到 lifecycle `OnStart(ctx)`，并保持 `OnStop(ctx)` 使用 `provider.Shutdown(ctx)`。
- [x] 1.3 修改 `newOTLPExporter` 错误处理，使用标准 `%w` wrapping 保留 `otlptrace.New` 返回的底层 cause。
- [x] 1.4 更新 tracing 单元测试，删除 exporter 在 construction 阶段创建的旧断言，新增 `OnStart(ctx)` 触发 exporter 初始化和 provider stop 清理的断言。
- [x] 1.5 新增 tracing 启动预算测试，使用阻塞 exporter 验证 `app.Start` 受 `fx.StartTimeout` 或传入 context 取消控制。
- [x] 1.6 新增 exporter cause 保留测试，验证返回错误可通过 `errors.Is` 或 `errors.As` 匹配底层错误。

## 2. RBAC Lifecycle Contract

- [x] 2.1 将 Casbin engine 初始加载入口改为明确表达 fail-closed 降级启动语义的 API，例如 `InitializeFailClosed(ctx)`，并移除不再使用的严格启动 error 返回契约。
- [x] 2.2 调整 `user-service/internal/features/permission/fx.go` 的 `registerRBACLifecycle`，调用新的 fail-closed 初始化入口，删除不可达的 `Initialize` error branch，并保持初始化后启动 watcher。
- [x] 2.3 将 RBAC lifecycle `OnStop` 改为合并 `params.Watcher.Stop(ctx)` 与 `params.Closer.Close()` 的错误，并保证 watcher stop 失败时仍执行 cache close。
- [x] 2.4 更新 Casbin engine 测试，覆盖初始 load 失败时初始化入口不返回错误、`LastError` 保留 cause、授权 fail-closed、后续 reload 成功清除错误。
- [x] 2.5 新增或更新 permission lifecycle 测试，验证 watcher stop 和 cache close 同时失败时返回错误包含两个 cause。
- [x] 2.6 新增 RBAC 降级启动集成测试，覆盖初始 policy load 失败后 `app.Start` 成功、watcher 运行、readiness/startup 失败、后续 reload 成功后 readiness/startup 恢复、shutdown 完整清理。

## 3. Specs And Verification

- [x] 3.1 运行 `openspec status --change "fix-lifecycle-error-contracts"`，确认 proposal、design、specs、tasks 均完成且 change apply-ready。
- [x] 3.2 运行 `go test ./runtime/observability/tracing` 验证 tracing lifecycle 行为。
- [x] 3.3 运行 `go test ./internal/features/permission ./internal/features/permission/infrastructure/casbin ./internal/providers` 验证 RBAC lifecycle 和健康检查行为。
- [x] 3.4 运行 `make user-service-architecture-lint` 验证架构边界未回归。
- [x] 3.5 将本次预期代码、测试、OpenSpec artifact 和相关文档变更加到暂存区。
- [x] 3.6 运行 `make lint` 并确认通过。
- [x] 3.7 运行 `make verify` 并确认通过，若生成物或格式化产生 drift，先修复并重新执行验证。
