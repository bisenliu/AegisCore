# Metrics Boundary

`common/runtime/observability/metrics` 是未来跨服务指标采集 runtime primitive 的边界。只有当至少两个服务需要同一套稳定、无业务语义的 metrics 基础能力时，才应在这里新增真实代码。

## 可以放置

- 跨服务通用的 meter/provider、registry、exporter 和 resource 属性构造逻辑。
- 不包含业务语义的 counter、histogram、gauge 等指标基础封装。
- 通用指标命名、label key、bucket 边界和错误记录约定。
- 与 HTTP middleware、datastore client 或外部调用 adapter 共享的 runtime 指标 helper。
- 面向 Fx 或 runtime 初始化的通用 provider wiring。

## 禁止放置

- 尚无真实跨服务需求的 Prometheus、OpenTelemetry Metrics 或其他 metrics 依赖。
- 用户服务专属的业务指标、feature label、SLA 口径或 dashboard 配置。
- Gin controller、HTTP route、Ent、Redis、SQL 或服务持久化访问。
- 认证、用户、角色、权限等 feature 的业务编排、领域模型或 application port。
- 为单个服务临时方便而扩张的 metrics facade、全局可变 registry 或通用大接口。

## 当前状态

当前没有真实跨服务 metrics runtime primitive。本目录只保留未来边界说明；新增实现前应先明确调用方、初始化方式、配置来源、exporter 行为、指标命名、label 基数控制和测试方式。
