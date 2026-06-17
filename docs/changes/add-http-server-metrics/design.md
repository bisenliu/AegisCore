# Design

## Context

现有可观测性基础已经提供：

- `common/runtime/observability/metrics.Provider`：独立 Prometheus registry/registerer/gatherer，禁用模式零副作用。
- `common/runtime/observability/metrics`：统一低基数 label 常量和 label key 校验。
- 用户服务已通过 `user-service/internal/router/metrics.go` 在 metrics enabled 时暴露配置化 Prometheus scrape endpoint，默认 `/metrics`。
- `user-service/internal/providers/gin.go` 统一创建 Gin engine，并已安装 OTel Gin tracing、span rename、panic recovery、request logger 和 CORS。
- `router.IsLowNoiseRuntimePath` 已能识别健康探针和启用后的 metrics path，用于成功请求 access log/tracing 降噪。

当前缺口是 HTTP request-level RED 指标。该能力属于跨服务 HTTP runtime primitive，不属于 user/auth/role/permission 任一 feature，也不应该放进 service-specific route handler 或 feature controller。

## Goals / Non-Goals

**Goals:**

- 提供无业务语义的 Gin HTTP server metrics middleware。
- 使用 Prometheus counter、histogram 和 gauge 采集请求量、错误/状态维度、延迟和 in-flight。
- 使用 Gin route template 作为 route label，未匹配请求使用固定 fallback。
- 支持过滤或标记低噪声 runtime endpoint，避免健康探针和 scrape 污染业务 RED 面板。
- 在用户服务 Gin middleware 链中接入，并保持 request logger 行为不变。
- 在 common 和 user-service 中分别补足 focused tests。

**Non-Goals:**

- 不新增 tracing span、trace attribute 或 exporter。
- 不新增业务、datastore、dependency、scheduler、workerpool 指标。
- 不改变现有 `/metrics` endpoint 暴露方式。
- 不改变认证、授权、路由注册、OpenAPI、日志字段或 HTTP response envelope。
- 不新增 dashboard、alert、ServiceMonitor、PodMonitor、Helm chart 或 Kubernetes artifact。

## Package Placement

新增代码放在 `common/http/middleware`，建议文件：

```text
common/http/middleware/metrics.go
common/http/middleware/metrics_test.go
```

放在 `common/http/middleware` 的理由：

- middleware 依赖 Gin request lifecycle 和 `c.FullPath()`，属于 HTTP/Gin 边界适配。
- 它只接收 common metrics provider/registerer，不承载服务业务语义。
- 它可以被未来其他 Gin 服务复用。

不放在 `common/runtime/observability/metrics` 的理由：

- runtime metrics package 应保持 exporter/provider、label contract 和 collector helper 的边界。
- Gin middleware 属于 HTTP adapter；让 runtime package 反向理解 Gin 会扩大依赖面。

## Public API

建议新增 API：

```go
type HTTPMetricsOptions struct {
    Provider              *metrics.Provider
    Skip                  func(*gin.Context) bool
    RouteFallback         string
    DurationBuckets       []float64
    IncludeStatusCode     bool
    IncludeApplicationCode bool
}

func HTTPServerMetrics(options HTTPMetricsOptions) gin.HandlerFunc
```

默认行为：

- `Provider == nil` 或 provider disabled 时返回零副作用 middleware。
- `RouteFallback` 为空时使用 `__unmatched__`。
- `DurationBuckets` 为空时使用 Prometheus 默认 duration buckets 或一组 HTTP 合理 buckets。
- 默认使用 `status_class` label；如启用 `status_code`，值必须是 HTTP numeric status 的有限集合。
- `Skip` 返回 true 时不记录 counter/duration，也不影响 in-flight，适合过滤成功健康探针和成功 metrics scrape。

如果实现希望 API 更小，可以先只暴露：

```go
type HTTPMetricsOptions struct {
    Provider      *metrics.Provider
    Skip          func(*gin.Context) bool
    RouteFallback string
}
```

后续再按需要增加 bucket 或 label 选项。

## Metric Model

指标使用 Prometheus client collectors：

```text
http_server_requests_total
http_server_request_duration_seconds
http_server_in_flight_requests
```

建议 label：

| Metric | Type | Labels |
|---|---|---|
| `http_server_requests_total` | counter vec | `method`, `route`, `status_class` 或 `status_code`，可选 `code` |
| `http_server_request_duration_seconds` | histogram vec | `method`, `route`, `status_class` 或 `status_code` |
| `http_server_in_flight_requests` | gauge vec | `method`, `route` |

说明：

- `service` 和 `environment` 由 metrics provider 的 wrapped registerer 注入，不在 middleware collector 中重复定义。
- `method` 使用标准 HTTP method；异常空值使用固定 `UNKNOWN`。
- `route` 优先使用 `c.FullPath()`，为空时使用 fallback。
- `status_class` 使用 `2xx`、`3xx`、`4xx`、`5xx`、`unknown` 这类固定值。
- `status_code` 如启用，使用 `strconv.Itoa(c.Writer.Status())`，状态码空间有限。
- `code` 如实现，必须只来自稳定应用错误码；不能使用原始错误字符串或响应 message。

禁止 label：

- 用户、角色、权限、session、token、cursor、trace/span/request ID。
- IP、User-Agent、raw URL、query string、email、username、手机号。
- SQL、Redis key、Authorization header、Cookie、原始错误消息。

## Middleware Lifecycle

推荐伪代码：

```go
func HTTPServerMetrics(options HTTPMetricsOptions) gin.HandlerFunc {
    recorder := newHTTPMetricsRecorder(options)
    if recorder == nil {
        return func(c *gin.Context) { c.Next() }
    }

    return func(c *gin.Context) {
        if options.Skip != nil && options.Skip(c) {
            c.Next()
            return
        }

        method := normalizeMethod(c.Request.Method)
        route := routeFallback
        start := time.Now()

        recorder.inFlight.WithLabelValues(method, route).Inc()
        defer func() {
            route = routeTemplateOrFallback(c, routeFallback)
            status := c.Writer.Status()
            statusClass := metrics.StatusClass(status)

            recorder.inFlight.WithLabelValues(method, routeFallback).Dec()
            recorder.requests.WithLabelValues(method, route, statusClass).Inc()
            recorder.duration.WithLabelValues(method, route, statusClass).Observe(time.Since(start).Seconds())
        }()

        c.Next()
    }
}
```

in-flight route label 有一个细节：Gin route template 在 handler 匹配后才可靠。为避免同一请求在进入时不知道 template，可以采用以下方案之一：

1. in-flight 只按 `method` 记录，不带 `route` label。
2. in-flight 进入时先使用 fallback，defer 时按 fallback decrement，并对完成指标使用真实 template。
3. 将 middleware 放在 route group 层而不是全局层，使 `FullPath()` 更早可用。

本需求要求 label 至少包括 `method`、`route`、`status_code` 或 `status_class`，但 in-flight 没有最终 status。推荐方案 2：in-flight 使用 `method`、`route`，进入时为 fallback，退出时只 decrement 同一组 label；完成类指标记录真实 route template。这能避免 gauge 泄漏并保持稳定 label。若实现发现 Gin 在全局 middleware 执行时已能提供 `FullPath()`，可以直接使用 template，但必须通过测试覆盖。

## Filtering Runtime Endpoints

用户服务已有低噪声 runtime path 判断：

```go
router.IsLowNoiseRuntimePath(path, metricsCfg)
```

HTTP metrics middleware 需要过滤成功健康探针和成功 metrics scrape。由于请求开始时还不知道最终 status，common middleware 的 `Skip` 只能基于请求前信息过滤。如果要只过滤成功请求，需要在 middleware 内支持完成后决策：

- 简单实现：对 runtime path 全量过滤。这会让失败健康探针和失败 metrics scrape 不进入 HTTP RED 指标，但仍由 access log 记录失败。
- 精细实现：新增 `SkipResult func(*gin.Context) bool`，在 `c.Next()` 后根据 status 判断是否记录。用户服务传入 `status < 400 && router.IsLowNoiseRuntimePath(...)`。

推荐精细实现，避免失败 runtime endpoint 完全不可见：

```go
type HTTPMetricsOptions struct {
    Provider   *metrics.Provider
    SkipResult func(*gin.Context) bool
}
```

这样与现有 access log skip 语义一致：成功 runtime endpoint 降噪，失败仍可见。

## User-Service Wiring

`user-service/internal/providers/gin.go` 的 `GinParams` 增加 metrics provider：

```go
Metrics *commonmetrics.Provider
```

middleware 链建议顺序：

```go
engine.Use(
    otelgin.Middleware(...),
    renameHTTPServerSpan(),
    commonmw.Recovery(params.Log),
    commonmw.HTTPServerMetrics(...),
    commonmw.RequestLoggerWithOptions(...),
    commonmw.CORS(),
)
```

排序说明：

- OTel middleware 保持最外层，继续创建 server span。
- Recovery 保持在 metrics 内侧或外侧都需要能保证 panic 后 `c.Next()` 返回并记录 5xx。推荐 metrics 放在 recovery 之后，使 recovery 先处理 panic 并设置响应状态，再由 metrics defer 记录最终状态。
- Request logger 行为不改变，仍按现有 skip 规则记录或跳过。
- CORS 保持现有位置，避免改变现有行为。

如果测试表明 panic 记录不到 5xx，需要调整 metrics 与 recovery 顺序，并用 panic test 固定行为。

接入选项：

```go
commonmw.HTTPServerMetrics(commonmw.HTTPMetricsOptions{
    Provider: params.Metrics,
    SkipResult: func(c *gin.Context) bool {
        return c.Writer.Status() < http.StatusBadRequest &&
            router.IsLowNoiseRuntimePath(c.Request.URL.Path, params.Config.Observability.Metrics)
    },
})
```

## Error Code Label

本变更不强制实现 `code` label。原因：

- 当前统一 error envelope 写出在 response helper 中完成，middleware 不应解析响应体。
- 从 Gin context 中提取稳定应用错误码需要先确认现有 response/error helper 是否有稳定 context key。
- 错误码 label 一旦设计不当容易引入 raw error 或 message 高基数。

第一阶段推荐仅记录 `status_class` 或 `status_code`。后续如要加入 `code`，应由 `common/http/response` 或错误处理层显式写入稳定 context key，middleware 只读取有限集合。

## Registration And Duplicate Handling

middleware 构造时通过 provider 注册 collectors：

- disabled provider：不创建 collectors，不注册。
- enabled provider：使用 `Provider.Register`，重复注册视为成功。
- collector 名称固定，不使用服务名或环境拼接 metric name。

测试中如一个 provider 多次构造 middleware，不能因 AlreadyRegistered 导致 panic。实现可在 recorder 内复用 collector，或依赖 `Provider.Register` 的重复注册保护。

## Documentation

更新：

- `common/runtime/observability/metrics/README.md`：补充 HTTP RED middleware、metric name、label contract 和禁止高基数 label。
- `docs/ARCHITECTURE.md`：说明用户服务 Gin provider 接入 HTTP metrics middleware，业务 route 使用 template label，runtime endpoint 成功请求降噪。

不需要更新 OpenAPI，HTTP metrics 是运行时观测能力，不属于业务 API 契约。

## Risks / Trade-offs

- 全局 middleware 中 route template 的可用时机需要测试确认，尤其是 404、405 和 panic。
- in-flight 如果使用 route fallback，会减少按业务 route 观察当前并发的精度，但能避免 gauge label 泄漏；完成类 RED 指标仍按真实 template 记录。
- 成功健康探针和 metrics scrape 被过滤后不会出现在 HTTP RED 业务面板，需要通过专门探针状态或 Prometheus target health 观察。
- 如果后续新增 `code` label，必须先建立稳定低基数错误码上下文传递，不能解析响应 body。

## Test Plan

- common middleware test：metrics disabled 时不注册 collector，scrape 不出现 HTTP server metrics。
- common middleware test：业务 route 请求后 counter 增加，duration histogram 有样本。
- common middleware test：5xx 请求记录为 `5xx` 或对应 `status_code`。
- common middleware test：route label 为 Gin template `/api/v1/users/:id`。
- common middleware test：未匹配 route 使用 fallback，不包含 raw path。
- common middleware test：in-flight 在阻塞 handler 执行中增加，handler 完成后归零。
- common middleware test：`SkipResult` 对成功 runtime endpoint 生效，失败 runtime endpoint 仍记录。
- user-service provider test：metrics enabled 时 Gin engine scrape 可看到 HTTP server metrics。
- user-service provider test：成功 `/livez`、`/readyz`、`/startupz` 和 `/metrics` 不污染业务 HTTP metrics。
- user-service provider test：request logger skip 行为保持不变。
- 验证：运行 `make test-common` 和 `make test-user-service`。
