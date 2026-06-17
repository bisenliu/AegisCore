# Metrics Runtime

`common/runtime/observability/metrics` 是跨服务指标采集 runtime primitive。当前支持 Prometheus 独立 registry/provider、可选 Go runtime/process collector、低基数 label 约定和 Fx 友好的 provider 构造。

Provider 基于 `common/runtime/config.MetricsConfig`、服务名和部署环境创建。启用模式使用独立 `prometheus.Registry`，不会写入 Prometheus 默认全局 registry。禁用模式保持零副作用：不创建 registry、不注册 collector、不注册 Go runtime/process collector，调用 `Register` 也不会产生效果。

本 package 不挂载 `/metrics` HTTP 路由。服务侧后续如需暴露 Prometheus endpoint，应显式检查 `Provider.Enabled()`，再使用 `Provider.Gatherer()` 挂载自己的路由。

## 可以放置

- 跨服务通用的 Prometheus provider、registry、registerer、gatherer 和 runtime collector 构造逻辑。
- 不包含业务语义的 counter、histogram、gauge 等指标基础封装。
- 通用指标命名、label key、bucket 边界和错误记录约定。
- 与 HTTP middleware、scheduler、workerpool、datastore client 或外部调用 adapter 共享的 runtime 指标 helper。
- 面向 Fx 或 runtime 初始化的通用 provider wiring。

## 禁止放置

- 用户服务专属的业务指标、feature label、SLA 口径、dashboard、告警规则或部署清单。
- Gin controller、HTTP route、Ent、Redis、SQL 或服务持久化访问。
- 认证、用户、角色、权限等 feature 的业务编排、领域模型或 application port。
- OpenTelemetry Metrics 依赖。
- 为单个服务临时方便而扩张的 metrics facade、全局可变 registry 或通用大接口。

## 配置语义

- `observability.metrics.enabled: false`：返回 disabled provider，零副作用。
- `observability.metrics.enabled: true`：创建独立 Prometheus registry。
- `observability.metrics.include_runtime: true`：注册 Go runtime collector 和 process collector。
- `observability.metrics.include_runtime: false`：不注册 Go runtime/process collector。
- `observability.metrics.path`：只表达未来 HTTP metrics 暴露路径；本 package 不自动挂载该路径。

## Label 约定

通用 label key：

| Label | 来源 | 约束 |
|---|---|---|
| `service` | app name | 由 provider 作为 const label 注入 |
| `environment` | app environment | 由 provider 作为 const label 注入 |
| `method` | HTTP method 或 runtime action | 枚举值，例如 GET、POST |
| `route` | HTTP route template 或固定 runtime key | 必须是模板或枚举值，例如 `/api/v1/users/:id` |
| `status_class` | HTTP status class | 使用 `2xx`、`3xx`、`4xx`、`5xx` |
| `code` | 稳定错误码或结果码 | 必须来自有限集合，不得填入原始错误字符串 |

禁止作为 label 的值：

- user ID、role ID、permission ID、session ID、token ID、trace ID、span ID、request ID。
- IP、User-Agent、URL query、raw path、email、username、手机号。
- SQL、Redis key、JWT、Authorization header、Cookie、原始错误消息。
- 任意高基数或敏感值。

## 命名约定

- 指标名使用 Prometheus snake_case。
- AegisCore runtime primitive 指标使用 `aegiscore_` 前缀。
- 服务名和环境不写入 metric name，使用 `service` 和 `environment` label。

预留命名：

| Area | Metric | Type | Labels |
|---|---|---|---|
| HTTP | `aegiscore_http_requests_total` | counter | `service`,`environment`,`method`,`route`,`status_class`,`code` |
| HTTP | `aegiscore_http_request_duration_seconds` | histogram | `service`,`environment`,`method`,`route`,`status_class` |
| Scheduler | `aegiscore_scheduler_jobs_total` | counter | `service`,`environment`,`job`,`event`,`code` |
| Scheduler | `aegiscore_scheduler_job_duration_seconds` | histogram | `service`,`environment`,`job`,`status` |
| Workerpool | `aegiscore_workerpool_tasks_total` | counter | `service`,`environment`,`pool`,`event`,`code` |
| Workerpool | `aegiscore_workerpool_running` | gauge | `service`,`environment`,`pool` |
| Workerpool | `aegiscore_workerpool_queued` | gauge | `service`,`environment`,`pool` |

`job`、`pool`、`event` 和 `status` 也必须是固定枚举式低基数值。Scheduler job key 和 workerpool name 不得来自用户输入或业务实体 ID。

## 当前状态

当前 package 提供 `NewProvider`、`NewFxProvider`、`Provider.Register`、`Provider.Registerer`、`Provider.Gatherer`、`StatusClass` 和 label key 常量。Scheduler、workerpool 和 HTTP 指标采集 adapter 尚未接入；后续实现应继续保持本包无业务语义，并由服务侧显式挂载 `/metrics` 路由。
