# Add local OTel tracing provider

## What

在 `common/runtime/observability/tracing` 中落地跨服务无业务语义的 OpenTelemetry tracing runtime primitive。

本变更新增本地可用的 tracing provider 构造能力：

- 基于 `common/runtime/config.TracingConfig` 和服务标识创建 OpenTelemetry SDK `TracerProvider`。
- 支持 `observability.tracing.exporter: none`，该模式不创建 OTLP exporter、不连接 Collector，但仍使用 SDK provider、sampler、resource 和标准 W3C 上下文传播。
- 设置 resource attributes：`service.name`、`deployment.environment`，并预留 `service.version` 和 `service.instance.id`。
- 设置 W3C `TraceContext` 与 `Baggage` propagator。
- 使用 parent-based sampler，root span 采样率由 `observability.tracing.sample_ratio` 控制；本地默认 `1.0`。
- 提供 Fx 生命周期友好的 shutdown 方法或 provider，确保服务关闭时可统一释放 tracing runtime 资源。
- 更新文档，明确 `exporter: none` 只提供标准 trace/span ID、本进程上下文传播和未来日志关联基础，不提供 trace 可视化。

## Why

仓库已经在 `common/runtime/config` 中建立了 `observability.tracing` 配置契约，但 `common/runtime/observability/tracing` 仍只有边界 README，没有真实 provider。后续 Gin middleware、日志 trace/span 关联、外部调用 instrumentation 和可选 OTLP exporter 都需要一个稳定、跨服务、无业务语义的 tracing 基础能力。

先落地 `exporter: none` 的 SDK provider 可以带来：

- 本地开发不需要部署 OTLP Collector、Jaeger、Tempo 或其他后端。
- 即使不导出 span，也能生成标准 OpenTelemetry `trace_id` 和 `span_id`，为日志关联和跨边界传播打基础。
- 采样策略、resource attributes、propagator 和 shutdown 语义先稳定下来，后续接入 exporter 或 middleware 时不需要重复设计。
- 将 tracing runtime 能力保留在 `common`，避免 user-service 或 feature 层各自散落 OTel 初始化逻辑。

## Scope

包括：

- 新增 `common/runtime/observability/tracing` 实现代码。
- 定义 provider 构造输入，至少包含：
  - `config.TracingConfig`
  - service name
  - deployment environment
  - 可选 service version
  - 可选 service instance ID
- 构造 OpenTelemetry SDK `TracerProvider`。
- `exporter: none` 时不创建 span exporter，使用 SDK provider 与 sampler 正常生成 span context。
- 使用 parent-based sampler，root sampler 使用配置中的 ratio。
- 创建 OpenTelemetry resource，至少包含 `service.name` 和 `deployment.environment`。
- 配置全局或可注入的 W3C `TraceContext` + `Baggage` propagator。
- 提供 `Shutdown(ctx)`，并可提供 Fx lifecycle provider/module 以便服务侧后续接入。
- 增加单元测试，覆盖：
  - 无 exporter provider 可创建和关闭。
  - 使用 provider 创建 span 时有有效 trace ID 和 span ID。
  - 采样率传入 sampler 后行为符合预期。
  - resource attributes 包含服务名和部署环境。
  - shutdown 可重复或安全调用。
- 更新 `common/runtime/observability/tracing/README.md`。
- 更新 `docs/ARCHITECTURE.md` 和 `docs/DEVELOPMENT.md`，说明本地 tracing provider 行为与限制。

不包括：

- 不接入 Gin middleware。
- 不改造 logger，也不新增日志字段迁移。
- 不接入 Redis、PostgreSQL、Ent 或外部 HTTP/gRPC/events instrumentation。
- 不部署 Collector、Jaeger、Tempo 或任何 tracing 后端。
- 不实现 metrics exporter、`/metrics` 路由、dashboard 或告警。
- 不保留或新增 `X-Trace-ID` 兼容逻辑。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不修改 HTTP API、认证、RBAC、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。

## Acceptance Criteria

- 在无 OTLP endpoint、无 Collector 的环境下，`exporter: none` 的 tracer provider 可正常创建和关闭。
- 使用该 provider 创建 span 时，span context 中存在有效 OpenTelemetry trace ID 和 span ID。
- sampler 使用 parent-based 策略，root span 采样率来自配置。
- resource attributes 至少包含 `service.name` 和 `deployment.environment`。
- W3C `TraceContext` 与 `Baggage` propagator 被正确配置或可由调用方明确注入。
- `common/runtime/observability/tracing` 单元测试覆盖无 exporter、采样率、resource attributes 和 shutdown。
- `common` 模块测试通过。
- 文档明确 `exporter: none` 只提供日志关联和标准上下文传播基础，不提供 trace 可视化。
- 未新增 `openspec/` 或 `docs/opsx/` 目录。
