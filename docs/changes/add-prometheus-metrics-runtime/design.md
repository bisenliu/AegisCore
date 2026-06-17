# Design

## Overview

本变更把 `common/runtime/observability/metrics` 从边界 README 推进为真实 Prometheus runtime primitive。

```text
common/runtime/config.MetricsConfig
  -> common/runtime/observability/metrics.Options
  -> independent prometheus.Registry
  -> optional Go runtime/process collectors
  -> shared label and naming conventions
  -> optional Fx lifecycle/provider wiring
```

核心原则：

- `common` 只提供跨服务、无业务语义的 metrics runtime 能力。
- Prometheus registry 使用独立实例，不使用默认全局 registry。
- 禁用模式零副作用。
- `/metrics` HTTP route 由服务侧显式挂载，本变更不改 user-service 路由图。
- 指标 label 必须低基数、稳定、可聚合。

## Current State

已有基础：

- `common/runtime/config.MetricsConfig` 已包含 `enabled`、`path` 和 `include_runtime`。
- `user-service/configs/config.yaml` 当前默认 metrics disabled，path 为 `/metrics`，include runtime 为 true。
- `common/runtime/observability/metrics/README.md` 当前只声明未来边界，没有 Go 实现。
- `common/runtime/scheduler` 已定义 `Metrics` 接口，当前默认 `NopMetrics`。
- `common/runtime/workerpool` 已提供 `Stats()` 快照，字段包含 submitted、rejected、started、completed、failed、panicked、queued、running、free、waiting 和 closed。
- `common/runtime/observability/tracing` 已建立本地 OTel tracing provider，但本变更不引入 OpenTelemetry Metrics。

约束：

- metrics package 不导入 Gin、Ent、Redis、SQL、user-service 或 feature 包。
- 不放 auth/user/role/permission 业务指标、SLA 口径、dashboard、告警或部署清单。
- 不把 `scheduler` 和 `workerpool` 反向耦合到 Prometheus；适配应放在 metrics package 或服务侧 wiring。
- 代码注释使用中文；如新增日志，日志消息必须使用英文。本变更首选不输出日志。
- 不新增 `openspec/` 或 `docs/opsx/`。

## Package Shape

建议文件：

```text
common/runtime/observability/metrics/
  README.md
  provider.go
  labels.go
  register.go
  runtime.go
  fx.go
  provider_test.go
  labels_test.go
```

可选后续文件：

```text
  scheduler.go
  workerpool.go
  http.go
```

第一阶段优先实现 registry/provider、runtime collector 和 label 约束。scheduler、workerpool 和 HTTP adapter 可以在本变更中定义命名约定与适配边界；若实现具体 collector，必须保持无业务语义，并只消费既有稳定接口或显式传入的 snapshot 函数。

## Public API

推荐公开 API：

```go
type Options struct {
    Config      config.MetricsConfig
    ServiceName string
    Environment string
}

type Provider struct {
    // unexported fields
}

func NewProvider(opts Options) (*Provider, error)
func (p *Provider) Enabled() bool
func (p *Provider) Registerer() prometheus.Registerer
func (p *Provider) Gatherer() prometheus.Gatherer
func (p *Provider) Register(collector prometheus.Collector) error
func (p *Provider) MustRegister(collector prometheus.Collector) error
```

说明：

- `Options.Config` 复用 `common/runtime/config.MetricsConfig`。
- `ServiceName` 来自 `config.AppConfig.Name`。
- `Environment` 来自 `config.AppConfig.Environment`。
- `Provider.Registerer()` 和 `Provider.Gatherer()` 暴露 Prometheus 标准接口，便于服务侧后续用 `promhttp.HandlerFor(provider.Gatherer(), ...)` 挂载路由。
- 禁用模式下 `Enabled()` 返回 false；`Registerer()` 可返回 no-op registerer 或 nil，但调用方应通过 `Enabled()` 判断是否挂载路由。
- `Register` 应安全处理 `prometheus.AlreadyRegisteredError`，返回 nil 或返回已注册 collector 信息，避免重复 provider wiring 导致启动失败。

禁用模式建议：

```go
if !opts.Config.Enabled {
    return &Provider{enabled: false}, nil
}
```

禁用 provider 不创建 `prometheus.Registry`，也不注册任何 collector。若实现上为了简化返回空 registry，也必须测试确认不会注册 runtime/process collector；更推荐真正 no-op。

启用模式建议：

```go
registry := prometheus.NewRegistry()
provider := &Provider{
    enabled: true,
    registry: registry,
    registerer: prometheus.WrapRegistererWith(prometheus.Labels{
        LabelService: serviceName,
        LabelEnvironment: environment,
    }, registry),
    gatherer: registry,
}
```

是否使用 `WrapRegistererWith`：

- 优点：所有通过 provider 注册的 collector 自动拥有 `service` 与 `environment` const labels。
- 风险：如果调用方注册的 collector 已经定义同名 label，会冲突。
- 推荐：provider 文档明确 `service` 与 `environment` 由 provider 注入，业务 collector 不得重复定义。

## Dependencies

在 `common/go.mod` 中新增 Prometheus client：

```bash
cd common
go get github.com/prometheus/client_golang@latest
go mod tidy
```

实现时应固定到 `go get` 解析出的稳定版本，并提交 `common/go.mod` 与 `common/go.sum`。不要引入 OpenTelemetry Metrics 或 Prometheus exporter 之外的 dashboard/deployment 依赖。

## Runtime Collectors

`observability.metrics.include_runtime` 控制 Go runtime/process collector：

- true：注册 Go runtime collector 和 process collector。
- false：不注册 runtime/process collector。

Prometheus client 可用 collector：

```go
collectors.NewGoCollector()
collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
```

实现细节：

- runtime/process collector 只在 enabled provider 上注册。
- 使用 `Register` 包装重复注册保护。
- 如果 process collector 在某些平台不可用，应返回清晰错误或在测试中使用 Prometheus 官方 collector 行为；不要 silently 忽略非重复注册错误。
- 不注册默认 Go collector 到全局 registry。

测试可以通过 `provider.Gatherer().Gather()` 断言 runtime metric family 是否存在，例如 `go_goroutines`。禁用模式或 include_runtime false 时不应出现这些 runtime family。

## Label Contract

推荐 label key 常量：

```go
const (
    LabelService     = "service"
    LabelEnvironment = "environment"
    LabelMethod      = "method"
    LabelRoute       = "route"
    LabelStatusClass = "status_class"
    LabelCode        = "code"
)
```

允许的 label 语义：

| Label | 来源 | 约束 |
|---|---|---|
| `service` | app name | provider 注入，固定服务名 |
| `environment` | app environment | provider 注入，固定环境名 |
| `method` | HTTP method 或 runtime action | 枚举值，例如 GET/POST/PUT/DELETE |
| `route` | Gin route template 或固定 runtime key | 必须是模板或枚举值，例如 `/api/v1/users/:id` |
| `status_class` | HTTP status class | 使用 `2xx`、`3xx`、`4xx`、`5xx` |
| `code` | 稳定错误码或结果码 | 必须来自有限集合，不得填入原始错误字符串 |

禁止作为 label：

- user ID、role ID、permission ID、session ID、token ID、trace ID、span ID、request ID。
- IP、User-Agent、URL query、raw path、email、username、手机号。
- SQL、Redis key、JWT、Authorization header、Cookie、原始错误消息。
- 任意高基数或敏感值。

建议 helper：

```go
func StatusClass(status int) string
func ValidateLowCardinalityLabelKey(key string) error
```

`StatusClass` 返回 `unknown` 或 `0xx` 之类稳定值处理非法状态码，具体实现需在测试中固定。`ValidateLowCardinalityLabelKey` 可以先只校验 common package 自有 helper 使用的 key 是否在允许集合中，避免提供过大的通用 validation facade。

## Metric Naming

命名建议：

- 统一使用 Prometheus snake_case。
- runtime primitive 指标以 `aegiscore_` namespace 开头，或由 provider 统一设置 namespace。
- 不把服务名写入 metric name，服务名放入 `service` label。
- 不把环境写入 metric name，环境放入 `environment` label。

建议预留命名：

| Area | Metric | Type | Labels |
|---|---|---|---|
| HTTP | `aegiscore_http_requests_total` | counter | `service`,`environment`,`method`,`route`,`status_class`,`code` |
| HTTP | `aegiscore_http_request_duration_seconds` | histogram | `service`,`environment`,`method`,`route`,`status_class` |
| Scheduler | `aegiscore_scheduler_jobs_total` | counter | `service`,`environment`,`job`,`event`,`code` |
| Scheduler | `aegiscore_scheduler_job_duration_seconds` | histogram | `service`,`environment`,`job`,`status` |
| Workerpool | `aegiscore_workerpool_tasks_total` | counter | `service`,`environment`,`pool`,`event`,`code` |
| Workerpool | `aegiscore_workerpool_running` | gauge | `service`,`environment`,`pool` |
| Workerpool | `aegiscore_workerpool_queued` | gauge | `service`,`environment`,`pool` |

`job`、`pool`、`event`、`status` 也必须是固定枚举式值。Scheduler job key 必须是代码配置的稳定任务名，不得来自用户输入或业务实体 ID。

第一阶段如果不实现 HTTP/scheduler/workerpool collectors，也必须把这些命名和 label 限制写入 README，作为后续实现边界。

## Scheduler And Workerpool Adapters

Scheduler 当前已有消费侧接口：

```go
type Metrics interface {
    JobRegistered(jobKey string)
    JobTriggered(jobKey string)
    JobStarted(jobKey string)
    JobCompleted(jobKey string, duration time.Duration)
    JobFailed(jobKey string, duration time.Duration)
    JobSkipped(jobKey, reason string)
    JobLockRenewFailed(jobKey string)
}
```

Prometheus adapter 可选设计：

```go
func NewSchedulerMetrics(provider *Provider, opts SchedulerOptions) scheduler.Metrics
```

要求：

- adapter 放在 metrics package 或服务侧 observability wiring，不放回 `common/runtime/scheduler`。
- 只把 `jobKey` 当低基数枚举使用。
- `reason` 必须来自 scheduler 固定原因值，例如 `local_overlap`、`global_concurrency_limit`、`lock_error`、`lock_busy`。
- provider disabled 时返回 `scheduler.NopMetrics{}`。

Workerpool 当前暴露 `Stats()`。Prometheus adapter 可选设计：

```go
type WorkerpoolStatsSource interface {
    Stats() workerpool.Stats
}

func NewWorkerpoolCollector(source WorkerpoolStatsSource) prometheus.Collector
```

要求：

- collector 只读取 stats snapshot，不改变 pool 生命周期。
- `pool` label 使用 `Stats.Name`，必须是固定配置名。
- 不在 workerpool package 中 import Prometheus，避免 runtime primitive 反向依赖具体 exporter。

## HTTP Metrics Boundary

HTTP metrics 的通用命名和 label 约定属于 common metrics 文档；具体 Gin middleware 或 route instrumentation 不在本变更中实现。

后续如果实现 HTTP middleware：

- Gin route 必须使用 route template，不能用 raw path。
- status 使用 `status_class` 聚合，具体 code 如需 label 必须确认基数有限。
- 不记录 query、client IP、User-Agent、Authorization header、Cookie 或请求体。
- middleware 可以位于 `common/http/middleware` 或服务侧 provider，但业务错误码映射仍由服务侧控制。

## Fx Wiring

推荐窄 Fx provider：

```go
type FxParams struct {
    fx.In

    Config *config.Config
}

func NewFxProvider(params FxParams) (*Provider, error)
```

行为：

- 从 `params.Config.App.Name`、`params.Config.App.Environment` 和 `params.Config.Observability.Metrics` 构造 provider。
- metrics provider 本身没有需要关闭的后台资源，因此无需 OnStop；保留 Fx 入口主要是为了服务侧 graph 统一注入。
- 不在本变更中自动接入 user-service `AppModule` 或 route provider。
- 不因 `observability.metrics.enabled: true` 自动注册 `/metrics` route。

如果实现阶段发现当前没有服务侧消费点，可以只提供 `NewProvider` 和可测试 API，把 Fx provider 延后到服务侧 wiring 变更；但 proposal 的目标是提供 Fx 友好的 wiring，因此建议至少提供不带副作用的 provider 函数。

## Error Handling

构造错误：

- service name 为空：`metrics service name is required`
- environment 为空：`metrics deployment environment is required`
- metrics path 校验仍由 `common/runtime/config` 持有，provider 不重复处理 path。
- collector 注册遇到非重复注册错误：返回包装错误，错误消息不包含敏感 runtime 数据。

重复注册：

- `prometheus.AlreadyRegisteredError` 应视为成功。
- 如果需要返回已有 collector，API 可以内部使用 existing collector；对调用方第一阶段无需暴露。

禁用模式：

- `Register` 对 disabled provider 返回 nil，保持零副作用。
- `Gatherer()` 对 disabled provider 可返回 nil；服务侧必须先检查 `Enabled()` 再挂载 promhttp handler。
- 测试固定该语义，避免后续误挂 route。

## Documentation Updates

`common/runtime/observability/metrics/README.md`：

- 从“当前没有真实 runtime primitive”更新为“当前支持 Prometheus registry/provider 和可选 runtime/process collector”。
- 说明 enabled、disabled、include_runtime 行为。
- 说明 provider 不使用默认全局 registry，不自动挂载 `/metrics`。
- 写清 label key、低基数限制和禁止的敏感/高基数 label。
- 写清 HTTP、scheduler、workerpool 指标命名约定。

`docs/ARCHITECTURE.md`：

- 更新 Common Organization 中 observability metrics 当前状态。
- 说明 `common/runtime/observability/metrics` 拥有 Prometheus registry/provider、runtime collector、label/naming 约定和 Fx 友好 wiring。
- 明确用户服务业务指标、dashboard、告警和部署清单仍禁止放入 common。
- 明确 scheduler/workerpool 的 Prometheus 接入通过 adapter 或服务侧 wiring 完成，不反向污染 runtime primitive。

`docs/DEVELOPMENT.md`：

- 更新 observability 配置章节，说明 metrics enabled/path/include_runtime 与 provider 行为。
- 说明当前仍不自动挂载 `/metrics`，服务侧路由接入需单独变更。
- 给出本地验证建议：`make test-common`。

## Testing Strategy

`common/runtime/observability/metrics/provider_test.go`：

- `TestNewProviderDisabledHasNoSideEffects`
  - enabled false。
  - 断言 `Enabled()` false。
  - 调用 `Register` 不报错。
  - 不创建 runtime/process collector，`Gatherer()` 语义按实现固定。
- `TestNewProviderEnabledCreatesRegistry`
  - enabled true。
  - 断言 `Enabled()` true。
  - 断言 `Registerer()` 与 `Gatherer()` 非 nil。
  - 注册测试 counter 后 gather 能看到 metric family。
  - 断言 const labels 包含 service 和 environment。
- `TestRuntimeCollectorsRespectConfig`
  - include_runtime true 时 gather 包含 `go_goroutines` 等 runtime family。
  - include_runtime false 时不包含 runtime family。
- `TestRegisterIgnoresAlreadyRegistered`
  - 同一 collector 注册两次返回 nil。
  - 不吞掉非重复注册错误。
- `TestNewProviderRejectsMissingServiceIdentity`
  - service name 或 environment 为空时报错。

`common/runtime/observability/metrics/labels_test.go`：

- `TestStatusClass`
  - 覆盖 200、204、302、404、500 和非法状态码。
- `TestLabelKeyConstants`
  - 确认公共 label 常量值稳定。

如果实现 scheduler 或 workerpool adapter，补充：

- disabled provider 返回 nop adapter。
- scheduler completed/failed/skipped 更新 counter/histogram。
- workerpool collector 从 snapshot 输出 gauge/counter，不修改 pool 状态。

验证命令：

```bash
cd common && go test ./runtime/observability/metrics
cd common && go test ./...
make test-common
```

如更新架构边界文档或 lint 规则，额外运行：

```bash
make architecture-lint
```

## Risks And Mitigation

风险：配置中启用 metrics 后被误解为用户服务已经暴露 `/metrics`。

缓解：README、架构文档和开发文档明确 provider 与 route mounting 分离；本变更不改 user-service 路由。

风险：使用默认 Prometheus global registry 带来跨测试污染和重复注册 panic。

缓解：始终使用独立 `prometheus.Registry`，并通过 `Register` 包装 `AlreadyRegisteredError`。

风险：label 误用导致高基数或敏感数据进入 Prometheus。

缓解：在 metrics package 中定义 label key 常量和 README 禁止清单；HTTP route 必须使用模板，错误使用稳定 code，不允许 raw error、user ID、token 或 request ID。

风险：scheduler/workerpool 为了接入 Prometheus 反向 import metrics package。

缓解：adapter 放在 metrics package 或服务侧 wiring；scheduler 保持 `Metrics` 接口，workerpool 保持 `Stats()` snapshot。

风险：runtime/process collector 重复注册导致服务启动失败。

缓解：provider 使用独立 registry，并在注册 wrapper 中将同一 collector 的重复注册视为成功。
