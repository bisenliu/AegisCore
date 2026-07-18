## 1. 共享 logger adapter

- [x] 1.1 在 `common/runtime/logger/fx.go` 中新增 `NewFxEventLogger(log *zap.Logger) fxevent.Logger`，使用 `log.Named("fx")` 创建 `fxevent.ZapLogger`。
- [x] 1.2 为 Fx event logger 设置常规事件 debug 级别和错误事件 error 级别，并保持 `LogEvent` 路径只依赖本地 zap adapter。
- [x] 1.3 在 `common/runtime/logger/fx_test.go` 增加单元测试，验证 event logger 使用命名 logger、debug/error 级别输出，并且不替换进程级默认 logger。

## 2. user-service 顶层接入

- [x] 2.1 在 `user-service/internal/bootstrap/app.go` 的 `AppOptions` 中通过 `fx.WithLogger(logger.NewFxEventLogger)` 启用自定义 Fx event logger。
- [x] 2.2 保持 `logger.NewLogger` 仍由 `fx.Provide` 注入并负责 Stop hook 同步，不新增 App 级额外 logger 生命周期。
- [x] 2.3 在 `user-service/internal/bootstrap` 相关测试中覆盖默认 `AppOptions` 会让 Fx 事件进入注入的 zap logger，同时保留需要静默输出的测试可继续使用 `fx.NopLogger` 覆盖。

## 3. 验证与交付

- [x] 3.1 运行相关 Go 测试：`go test ./common/runtime/logger ./user-service/internal/bootstrap`。
- [x] 3.2 运行架构校验：`make user-service-architecture-lint`。
- [x] 3.3 检查不需要更新 OpenAPI、Ent、Atlas migration 或部署观测生成物，并确认 diff 只包含本 change 预期文件。
- [x] 3.4 将本次预期代码、测试和 OpenSpec artifacts 加入暂存区。
- [x] 3.5 运行 `make lint`，通过后再勾选本任务。
- [x] 3.6 运行 `make verify`，通过后再勾选本任务。

## 4. tracing provider 初始化时序补充

- [x] 4.1 更新 `common/runtime/observability/tracing.NewFxProvider`，在 provider 构造阶段创建可用的底层 `TracerProvider()`。
- [x] 4.2 保留 Fx lifecycle rollback 语义，注册 no-op `OnStart` 和 `OnStop: provider.Shutdown`，确保后续启动失败时释放 tracing provider。
- [x] 4.3 更新 tracing provider 单元测试，覆盖构造后立即可用、exporter 构造时机、rollback shutdown 和 disabled tracing never-sample 行为。
- [x] 4.4 运行相关测试：`go test ./common/runtime/observability/tracing ./user-service/internal/providers ./user-service/internal/bootstrap`。
- [x] 4.5 运行 `make user-service-run` 验证不再因 `redis tracing provider is required` 在 Fx 构图阶段失败。
- [x] 4.6 运行 `make lint` 和 `make verify`，通过后再勾选本任务。
