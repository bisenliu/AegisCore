# Design

## Context

`common/runtime/observability/metrics` 当前已经提供 Prometheus provider：

- `metrics.NewFxProvider` 从 `config.Config.App` 和 `config.Config.Observability.Metrics` 构造 provider。
- enabled provider 持有独立 `prometheus.Registry`、`Registerer` 和 `Gatherer`。
- disabled provider 零副作用，不创建 registry，不注册 collector。
- `include_runtime: true` 时注册 Go runtime/process collector。
- common package 明确不自动挂载 `/metrics` HTTP route。

用户服务当前 HTTP route graph 由 `user-service/internal/providers/routes.go` 适配依赖，再调用 `user-service/internal/router.RegisterUserServiceHTTPRoutes`。健康探针、OpenAPI、pprof 和 `/api/v1` 业务路由都由 router 层总装。JWT authentication 与 RBAC authorization 只包裹 `/api/v1` 的受保护业务组，因此 root-level metrics route 可以自然避开 feature 业务层与 RBAC。

当前 Gin engine provider 已经为成功健康探针跳过 access log，并通过 OTel Gin filter 跳过健康探针 tracing。metrics scrape 成功路径也需要类似降噪，但不应复用 `IsHealthProbePath` 的语义名，避免把 metrics 误表达为 health endpoint。

## Goals / Non-Goals

**Goals:**

- 在用户服务启用 metrics 时暴露 Prometheus scrape endpoint。
- 使用 common metrics provider 的 `Gatherer()`，不使用 Prometheus 默认全局 registry。
- 让 metrics path 由 `observability.metrics.path` 配置，默认 `/metrics`。
- 保持 route ownership 在服务级 provider/router，不进入 feature 层。
- metrics endpoint 不需要 JWT，不进入 RBAC authorization，不纳入 permission route diff。
- 成功 scrape 跳过 access log；失败仍能被日志观察到。
- 增加 focused tests，覆盖 enabled、disabled、自定义 path、RBAC bypass 和 route scanner 行为。

**Non-Goals:**

- 不增加业务指标或 HTTP request metrics middleware。
- 不接入 scheduler/workerpool 指标 adapter。
- 不改变 tracing provider 或 OTel exporter 行为。
- 不新增部署层监控资源或 Helm/Kubernetes artifact。
- 不将 metrics route 放入 RBAC baseline 或权限目录。

## Decisions

### Provider wiring

在 `user-service/internal/providers/fx.go` 中加入：

```go
commonmetrics.NewFxProvider
```

该 provider 来自 `common/runtime/observability/metrics`，只构造 provider，不挂载 route，也不创建后台资源。它与 tracing provider 同属服务 runtime wiring，适合放在 `providers.Module`。

`RegisterRouteParams` 新增 metrics provider 依赖：

```go
Metrics *commonmetrics.Provider
```

如果为了测试便捷需要可选依赖，可将其标记为 optional，但生产 module 应始终通过 common provider 注入。`RegisterRoutes` 将 metrics provider 和 `params.Config.Observability.Metrics` 传入 router。

### Router-owned metrics mount

在 `user-service/internal/router` 新增 metrics route helper，例如 `metrics.go`：

```go
type MetricsRouteParams struct {
    Config   config.MetricsConfig
    Provider *commonmetrics.Provider
}

func registerMetricsRoute(engine *gin.Engine, params MetricsRouteParams)
func IsMetricsPath(path string, cfg config.MetricsConfig) bool
```

`RegisterUserServiceHTTPRoutes` 的建议顺序：

```go
registerHealthRoutes(...)
RegisterOpenAPI(...)
registerPprofRoutes(...)
registerMetricsRoute(...)
registerV1Routes(...)
```

metrics route 挂载在根路径，不放进 `/api/v1`。它不会经过 `AuthWithTokenVersionValidator` 和 `permissionhttp.Authorize`。

### Path normalization and conflict checks

`common/runtime/config` 已校验 metrics enabled 时 path 必须以 `/` 开头。用户服务路由层仍应做防御性校验，避免将危险或冲突 path 注册到 Gin：

- disabled 或 provider disabled：不注册。
- path trim 后为空：不注册或返回 provider-time error，优先通过现有配置校验阻止。
- path 必须以 `/` 开头。
- path 不允许包含 `:`、`*` 等 Gin 参数或通配符。
- path 不允许等于 `/livez`、`/readyz`、`/startupz`。
- path 不允许以 `/api/v1` 开头，避免进入业务 API 命名空间。
- path 不允许以 `/openapi`、`/docs`、`/api-docs` 或 `/debug/pprof` 等已知服务级文档/诊断前缀冲突。

如果配置冲突，应在 route registration 阶段返回错误，而不是静默覆盖。当前 `RegisterRoutes` 没有返回 error；实现可以选择：

- 将 `RegisterRoutes` 改为 `func RegisterRoutes(...) error`，让 Fx invoke 返回错误并 fail fast。
- 或在 metrics provider/route helper 中 panic。

推荐改为返回 error。这符合 Fx invoke 错误传播，也避免隐藏错误。需要同步调整相关 tests。

### Prometheus handler

使用 Prometheus 官方 handler：

```go
handler := promhttp.HandlerFor(provider.Gatherer(), promhttp.HandlerOpts{})
engine.GET(path, gin.WrapH(handler))
```

`Provider.Gatherer()` 为 nil 或 provider disabled 时不注册 route。不要使用 `prometheus.DefaultGatherer`。不要在 route handler 中追加服务业务数据、配置值、DSN 或 request-specific labels。

### Access log and tracing behavior

当前 `NewGinEngine` 使用：

```go
RequestLoggerWithOptions(... Skip: skipSuccessfulHealthProbeLog)
otelgin.WithFilter(traceBusinessRequest)
```

本变更应把命名扩展为 runtime endpoint 语义，例如：

- `skipSuccessfulRuntimeEndpointLog`
- `traceBusinessRequest`
- `router.IsLowNoiseRuntimePath(path, cfg)` 或 providers 层闭包捕获 metrics config。

行为建议：

- `/livez`、`/readyz`、`/startupz` 成功请求继续跳过 access log 和 tracing。
- metrics path 成功请求跳过 access log。
- metrics tracing 可与 access log 保持一致跳过，也可仅跳过 access log；为减少 scrape 噪声，推荐成功 metrics scrape 同时跳过 tracing。
- 非 2xx/3xx/成功状态不跳过 access log，便于观察 scrape 失败。

因为 OTel filter 在 handler 前执行，无法基于最终状态决定 tracing skip；因此 metrics path tracing skip 只能按 path 判断。access log skip 可继续基于最终 status 判断。

### RBAC and route diff

metrics route 不在 `/api/v1` 下，按现有 `RouteCatalogScanner.isAuthorizableRoute` 会被过滤。测试仍需显式覆盖 `/metrics` 和自定义 metrics path，防止未来扫描规则扩大后误纳入 RBAC baseline。

`providers/routes_test.go` 应增加 public route 断言：无 Authorization header 请求 metrics route 不返回 401，且 `routeAuthorizer.calls == 0`。

### Configuration and docs

更新 `user-service/configs/config.yaml` 注释：

- `enabled: false` 默认不暴露 route。
- `enabled: true` 时在 `path` 挂载 Prometheus scrape endpoint。
- `include_runtime` 控制 Go runtime/process collector。

更新文档：

- `docs/ARCHITECTURE.md` runtime flow、HTTP flow 和 infrastructure 可观测性段落。
- `docs/DEVELOPMENT.md` 中“当前阶段用户服务不注册 `/metrics` 路由”的内容改为启用后注册。

不需要更新 OpenAPI。Prometheus scrape endpoint 是运维 runtime endpoint，不使用 API response envelope，也不进入业务 OpenAPI 文档。

## Risks / Trade-offs

- metrics route 是公开 root-level endpoint；本变更按需求不做 RBAC，网络侧保护由部署层负责。
- metrics scrape 可能暴露 runtime/process 指标，虽然不含业务敏感数据，但仍应避免公网直接暴露。
- 如果配置 path 与现有 route 冲突，fail fast 会阻止服务启动。这比静默覆盖或注册不可达 route 更安全。
- 成功 scrape 跳过 access log 会减少日志噪声，但也意味着常规成功 scrape 需要通过 Prometheus target health 或 metrics 自身观察。
- 未新增 HTTP request metrics middleware，因此首个实现只暴露 provider 已注册的 runtime/process 指标和未来 collector。

## Test Plan

- Provider/module test：metrics provider 被 `providers.Module` 注入，enabled/disabled 不影响非 metrics 路由。
- Router test：disabled 时 configured metrics path 未注册。
- Router test：enabled 默认 `/metrics` 返回 HTTP 200、Prometheus text content type 和至少一个 metric family。
- Router test：custom path 生效，默认 `/metrics` 不额外注册。
- Router test：metrics path 配置冲突时 route registration 返回错误。
- Gin provider test：成功 metrics scrape 不产生 access log；失败或未命中仍产生 access log。
- Route registration test：metrics route 无 token 可访问，不触发 RBAC authorizer。
- Permission scanner test：`/metrics` 和 custom metrics path 不出现在 authorizable routes。
- Regression test：`/livez`、`/readyz`、`/startupz` 路径和响应不变。
- Verification：运行 `make test-user-service` 和 `make architecture-lint`。
