# Tracing Boundary

`common/runtime/observability/tracing` 是未来跨服务链路追踪 runtime primitive 的边界。只有当至少两个服务需要同一套稳定、无业务语义的 tracing 基础能力时，才应在这里新增真实代码。

## 可以放置

- 跨服务通用的 trace context 提取、注入和传播 helper。
- 通用 tracer provider、sampler、exporter 和 resource 属性的构造逻辑。
- 与日志、HTTP middleware 或外部调用 adapter 共享的稳定 trace metadata 常量。
- 不包含业务字段的 span 命名、attribute key 和 error recording 基础约定。
- 面向 Fx 或 runtime 初始化的通用 provider wiring。

## 禁止放置

- 尚无真实跨服务需求的 OpenTelemetry、Jaeger、Zipkin 或其他 tracing 依赖。
- 用户服务专属的 span 名称、业务 attribute、feature event 或 use case 编排。
- Gin controller、HTTP route、Ent、Redis、SQL 或服务持久化访问。
- 认证、用户、角色、权限等 feature 的业务状态、领域事件或应用 command/query。
- 为单个服务临时方便而扩张的 tracing facade 或通用大接口。

## 当前状态

当前没有真实跨服务 tracing runtime primitive。本目录只保留未来边界说明；新增实现前应先明确调用方、初始化方式、配置来源、exporter 行为、采样策略和测试方式。
