# Tracing Boundary

`common/runtime/observability/tracing` 是跨服务链路追踪 runtime primitive 的边界。当前支持基于 OpenTelemetry SDK 的 tracer provider、parent-based sampler、resource attributes、W3C TraceContext/Baggage propagator、OTLP trace exporter 和 Fx 生命周期关闭。

`observability.tracing.enabled: false` 是本地默认模式：它使用 `NeverSample`，不创建 OTLP exporter、不连接 Collector，也不向 Jaeger、Tempo 或其他 trace UI 导出 span。

启用 tracing 后会基于 `observability.tracing.otlp_endpoint` 创建 OTLP gRPC exporter，并通过 SDK batch processor 导出 span。生产类环境中 `insecure: true` 会在配置校验阶段被拒绝；endpoint 不应包含 token、账号密码、Cookie 或其他敏感凭据。配置层不提供 exporter 选择字段。

## 可以放置

- 跨服务通用的 trace context 提取、注入和传播 helper。
- 通用 tracer provider、sampler、exporter 和 resource 属性的构造逻辑。
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

当前实现只有 enabled/disabled 与 OTLP 传输语义。禁用 tracing 时会使用 `NeverSample` 且不创建 OTLP exporter，避免关闭状态仍连接 Collector。后续接入 exporter 认证、logger trace/span 字段或外部调用 instrumentation 时，应继续保持本包无业务语义，并单独说明认证和脱敏规则。
