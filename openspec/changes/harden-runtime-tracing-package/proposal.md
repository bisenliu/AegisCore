## Why

`common/runtime/observability/tracing` 当前把配置校验、resource、OTLP exporter、动态 tracer facade 和 Fx lifecycle 混在同一实现面中，导致启动时机、关闭所有权和重复启动行为不够清晰。随着 Redis、Gin、Ent 等 instrumentation 在 constructor 阶段需要稳定引用 tracing facade，需要把立即构造与 Fx 延迟启动的契约明确下来，避免 exporter 提前连接、重复 Start 泄漏旧 provider 或 Shutdown 后悬挂已关闭 provider。

## What Changes

- 拆分 `common/runtime/observability/tracing` 包内文件职责：`provider.go` 承载 provider facade 与公开 constructor，`resource.go` 承载 OTel resource 构造，`exporter.go` 承载 OTLP exporter 构造，`dynamic.go` 承载动态 tracer provider，`lifecycle.go` 承载启动与关闭状态机，`fx.go` 仅保留 Fx adapter。
- 明确 tracing provider 的两类构造所有权：普通 constructor 可立即启动并由调用方显式 `Shutdown`；Fx adapter 只能在 `OnStart` 延迟启动并由 Fx `OnStop` 或 rollback 关闭。
- 收窄仅供包内 lifecycle 使用的 `Start` 暴露面；如果需要改变导出 API，将在设计中声明 breaking impact 与调用方迁移路径。
- 新增 `doc.go` 与 `example_test.go`，覆盖 disabled provider、span 创建、propagator inject/extract 和显式 `Shutdown`，且不连接真实 OTLP endpoint。
- 新增或调整测试，覆盖重复启动、非法关闭、启动前动态 tracer 安全 no-op、启动后切换真实 provider、关闭后恢复 no-op，以及 race、go vet 和 executable examples。
- 不安装或替换 OpenTelemetry global provider，不新增 exporter protocol、采样策略或 HTTP、Redis、SQL、Ent instrumentation 的业务接线。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 明确 tracing provider 构造、启动、动态 tracer facade、关闭和 Fx rollback 的稳定行为。
- `shared-platform-primitives`: 明确 `common/runtime/observability/tracing` 作为共享 runtime primitive 的公开 API 边界、生命周期所有权和包内实现职责。

## Impact

- 影响代码：`common/runtime/observability/tracing/` 内 provider、resource、exporter、dynamic tracer、lifecycle、Fx adapter、文档和示例测试。
- 影响规格：更新 `runtime-observability` 与 `shared-platform-primitives` 的 OpenSpec delta。
- 影响公开 API：可能收窄 `Start` 的导出暴露面；若确认导出 API 变化，需在设计和任务中列出当前调用方、迁移方式和 breaking impact。
- 不影响 HTTP API、OpenAPI、数据库 schema、部署资产、metrics label、日志字段、Redis/SQL/Ent 业务 instrumentation 接线或进程级 OpenTelemetry global provider。
- 验证影响：需要运行 tracing 包普通测试、race 测试、go vet、executable examples，并在交付前运行 `make common-test`、`make user-service-architecture-lint`、`make lint` 和 `make verify`。
