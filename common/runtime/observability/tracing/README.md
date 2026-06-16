# Tracing Boundary

`common/runtime/observability/tracing` 是跨服务链路追踪 runtime primitive 的边界。当前支持基于 OpenTelemetry SDK 的本地 tracer provider、parent-based sampler、resource attributes、W3C TraceContext/Baggage propagator 和 Fx 生命周期关闭。

`exporter: none` 是本地默认模式：它会创建 SDK provider，并生成标准 OTel trace ID 与 span ID，但不会创建 OTLP exporter、不会连接 Collector，也不会把 span 导出到 Jaeger、Tempo 或其他 trace UI。该模式只提供日志关联和标准上下文传播基础，不提供 trace 可视化。

## 可以放置

- 跨服务通用的 trace context 提取、注入和传播 helper。
- 通用 tracer provider、sampler、未来 exporter 和 resource 属性的构造逻辑。
- 与日志、HTTP middleware 或外部调用 adapter 共享的稳定 trace metadata 常量。
- 不包含业务字段的 span 命名、attribute key 和 error recording 基础约定。
- 面向 Fx 或 runtime 初始化的通用 provider wiring。

## 禁止放置

- 用户服务专属的 span 名称、业务 attribute、feature event 或 use case 编排。
- Gin controller、HTTP route、Ent、Redis、SQL 或服务持久化访问。
- 认证、用户、角色、权限等 feature 的业务状态、领域事件或应用 command/query。
- Collector、Jaeger、Tempo、Zipkin、Grafana dashboard、告警规则或部署清单。
- 为单个服务临时方便而扩张的 tracing facade 或通用大接口。

## 当前状态

当前 package 提供 `NewProvider` 和 `NewFxProvider`。构造函数不会隐式安装 OpenTelemetry global tracer provider 或 global propagator，调用方需要显式使用返回的 provider 和 propagator；未来如果服务侧需要全局安装，应通过单独 wiring 明确接入。

当前只实现 `exporter: none`。`exporter: otlp` 会返回未实现错误，避免生产环境误以为 span 已经导出。后续接入 OTLP exporter、Gin tracing middleware、logger trace/span 字段或外部调用 instrumentation 时，应继续保持本包无业务语义，并单独说明 exporter endpoint、认证和脱敏规则。
