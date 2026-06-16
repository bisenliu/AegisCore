# Design

## Overview

本变更把用户服务 HTTP 入站请求链路从自定义 `X-Trace-ID` 中间件迁移到 OpenTelemetry Gin middleware。

```text
W3C traceparent/tracestate
  -> otelgin.Middleware
  -> OpenTelemetry server span
  -> c.Request.Context()
  -> handler / auth / RBAC / application
```

`common/runtime/observability/tracing` 继续只拥有跨服务、无业务语义的 OTel SDK provider、sampler、resource、propagator 和 Fx lifecycle。用户服务自己的 Gin middleware 顺序、健康探针过滤、route template span name 和日志兼容调整都留在 `user-service/internal/providers` 或现有 `common/http/middleware` 的无业务语义 middleware 中。

## Current State

已有基础：

- `common/runtime/observability/tracing` 已实现 `NewProvider` 与 `NewFxProvider`。
- tracing provider 支持 `exporter: none`，可生成标准 OTel trace/span context，但不会导出 span。
- `user-service/configs/config.yaml` 本地默认 tracing enabled、sample ratio `1.0`、exporter `none`。
- `common/http/middleware/trace_id.go` 当前读取或生成 `X-Trace-ID`，写入 Gin context、Go context、响应头和 logger context。
- `user-service/internal/providers/gin.go` 当前 middleware 顺序为 `TraceID`、`Recovery`、`RequestLogger`、`CORS`。
- `common/http/middleware/request_logger.go` 和 `recovery.go` 当前通过 `common/runtime/logger` context helper 读取 `trace_id` 字段。
- `user-service/internal/providers/routes_test.go`、`user-service/tests/e2e/harness_test.go` 和 `common/http/middleware/trace_id_test.go` 仍断言 `X-Trace-ID`。

约束：

- 不新增 `openspec/` 或 `docs/opsx/`。
- `common/http/middleware` 只能承载无业务语义的 Gin middleware。
- `common/runtime/observability/tracing` 不能引入 Gin、user-service、feature 包或业务 span 名称。
- 用户服务 provider 可持有服务名、健康探针路径和 Gin route template 语义。
- 不改变 auth、RBAC、CORS、健康探针和业务 handler 行为。

## Dependency Plan

`user-service` 需要新增 OTel Gin instrumentation 依赖：

```text
go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
```

由于 `user-service` 直接使用 `otelgin.Middleware`，该依赖应进入 `user-service/go.mod`。OpenTelemetry API/SDK 版本应与 `common/go.mod` 中现有 `go.opentelemetry.io/otel v1.24.0`、`go.opentelemetry.io/otel/sdk v1.24.0` 保持兼容。实现阶段使用 `go get` 或 `go mod tidy` 让 module 解析合适版本，不手写 `go.sum`。

## Fx Wiring

`common/runtime/observability/tracing.NewFxProvider` 当前能从 `*config.Config` 构造 `*tracing.Provider` 并注册 shutdown。用户服务 provider module 应显式提供它：

```go
fx.Provide(
    tracing.NewFxProvider,
    ProvidePostgresPools,
    ProvideRedisClients,
    NewJWTService,
    ProvideEntClients,
    ProvideHealthChecks,
    NewGinEngine,
)
```

`GinParams` 增加 tracing provider 输入：

```go
type GinParams struct {
    fx.In

    Config  *config.Config
    Log     *zap.Logger
    Tracing *tracing.Provider
}
```

`NewGinEngine` 使用 `params.Tracing` 配置 `otelgin.Middleware`。如果 `params.Tracing` 为 nil，应返回明确错误，避免服务在 tracing 配置缺失时悄悄回退到私有 trace-id 语义。

`exporter: otlp` 当前仍由 tracing provider 返回未实现错误，本变更不改变该行为。

## Middleware Order

推荐顺序：

```go
engine.Use(
    otelgin.Middleware(...),
    commonmw.Recovery(params.Log),
    commonmw.RequestLoggerWithOptions(params.Log, commonmw.RequestLoggerOptions{Skip: skipSuccessfulHealthProbeLog}),
    commonmw.CORS(),
)
```

理由：

- OTel middleware 尽早包裹请求，确保后续 recovery、logger、auth、RBAC 和 handler 都能看到 server span context。
- Recovery 保持在业务 handler 外层，panic 时仍能输出统一失败 envelope。
- Request logger 继续在请求结束后记录状态、路径、延迟和用户 ID。
- CORS 行为不改变。

健康探针 filter 由 `otelgin.WithFilter` 表达，`router.IsHealthProbePath(c.Request.URL.Path)` 返回 true 时不创建 OTel server span。日志跳过仍保留现有 `skipSuccessfulHealthProbeLog`，两者职责独立。

## Span Naming

使用稳定 span name formatter：

```go
func userServiceHTTPSpanName(c *gin.Context) string {
    path := c.FullPath()
    if path == "" {
        path = c.Request.URL.Path
    }
    return c.Request.Method + " " + path
}
```

已匹配路由使用 Gin route template，例如：

```text
GET /api/v1/users/:user_id
```

未匹配路由或 Gin 尚未解析 template 的路径降级为 URL path，例如：

```text
GET /not-found
```

formatter 放在 `user-service/internal/providers/gin.go` 或同包小文件中，因为它依赖用户服务 route graph 语义，不进入 `common/runtime/observability/tracing`。

## W3C Propagation

OTel Gin middleware 必须使用标准传播协议：

- 提取：读取 `traceparent` 和 `tracestate`。
- 注入：本变更不新增响应 trace header，也不写 `X-Trace-ID`。

如果当前 `otelgin` 版本支持直接传入 propagator，则使用 `params.Tracing.TextMapPropagator()`。如果该版本只读取 OTel global propagator，则在用户服务 tracing provider wiring 中显式调用 `otel.SetTextMapPropagator(params.Tracing.TextMapPropagator())`，并在测试中小心恢复 global state，避免污染其他测试。

同理，优先通过 `otelgin.WithTracerProvider(params.Tracing.TracerProvider())` 显式传入 provider；只有版本限制时才使用 global tracer provider。

## Logger And Recovery Transition

本变更不要求立即把日志字段升级为 `span_id`，也不新增响应 header。后续 `derive-logger-fields-from-otel-context` 变更会完成 `span_id` 字段、删除自定义 trace-id context key，并把 logger 字段来源收敛到 OTel span context。本阶段需要处理的是现有 logger helper 对自定义 `trace_id` context 的依赖：

- `commonmw.RequestLogger` 不应要求 `TraceID()` 先把 `trace_id` 写入 context。
- `commonmw.Recovery` 不应要求 `TraceID()` 先把 `trace_id` 写入 context。
- 日志仍可保留字段名 `trace_id`，但值应优先从 `trace.SpanContextFromContext(ctx).TraceID().String()` 获取。
- 当 OTel span context 无效时，日志字段可以为空，或省略该字段；不要生成私有 fallback trace ID。

本阶段可在 `common/runtime/logger` 增加无业务语义 helper：

```go
func WithOTelTraceID(ctx context.Context) context.Context
func TraceIDFromContext(ctx context.Context) string
```

或者直接调整 `TraceIDFromContext`：先读取现有 context value，再读取 OTel span context。这样能兼容少量非 HTTP 测试中手动 `logger.WithTraceID` 的用法，同时让 HTTP 请求自然从 OTel context 得到 `trace_id`。该兼容路径只属于过渡状态，后续 `derive-logger-fields-from-otel-context` 会迁移调用方并删除 `WithTraceID` / `TraceIDFromContext`。

`common/http/middleware/trace_id.go` 删除后，`common/http/middleware` 中仍可以保留 `ContextKeyLogger`，但应迁移到 request logger 或独立 constants 文件，避免常量随 trace-id middleware 删除而消失。

## Removing X-Trace-ID Contract

需要清理：

- `common/http/middleware/trace_id.go`
- `common/http/middleware/trace_id_test.go` 中专门验证 `TraceID` 的测试。
- `common/http/middleware/cors.go` 默认 allowed/exposed headers 中的 `HeaderTraceID`。
- `user-service/internal/providers/gin.go` 对 `commonmw.TraceID()` 的调用。
- `user-service/internal/providers/routes_test.go` 中所有 `X-Trace-ID` 请求头、响应头和固定日志 trace id 断言。
- `user-service/tests/e2e/harness_test.go` 中统一设置和断言 `commonmw.HeaderTraceID` 的逻辑。
- 文档中“HTTP trace header 是 `X-Trace-ID`”或“响应头包含 trace-id”的当前契约描述。

不要新增替代响应头。trace 传播只通过 W3C 请求头进入上下文；响应是否暴露 trace ID 交给未来单独设计。

## Tests

### Provider/Gin Unit Tests

在 `user-service/internal/providers` 增加或调整测试：

- 构造本地 `tracing.Provider`，传入 `NewGinEngine`。
- 注册测试路由，在 handler 中读取：

```go
spanContext := trace.SpanContextFromContext(c.Request.Context())
```

并断言 trace ID 和 span ID 有效。

- 发送带 `traceparent` 的请求，断言 handler 中 trace ID 等于 header 中 parent trace ID。
- 发送不带 `traceparent` 的请求，断言创建新的有效 trace ID。
- 对 `/livez`、`/readyz`、`/startupz` 请求验证 filter 不创建业务 server span。可用 `sdktrace.NewTracerProvider` 搭配 in-memory span recorder，或者在 handler/probe 后确认没有记录对应 span。
- 对 `/api/v1/users/:user_id` 验证 span name 为 `GET /api/v1/users/:user_id`。
- 对未匹配路由验证 span name fallback 使用 URL path。
- 验证响应头中没有 `X-Trace-ID`。

### Common Middleware Tests

更新 `common/http/middleware` 测试：

- 删除 `TraceID` 专属测试。
- Request logger 测试使用 OTel span context，不再安装 `TraceID()`。
- Recovery 测试使用 OTel span context 或明确断言没有自定义 trace header 也能记录 panic。
- CORS 测试不再默认暴露 `X-Trace-ID`。

### Route And E2E Tests

更新：

- `user-service/internal/providers/routes_test.go`
- `user-service/tests/e2e/harness_test.go`
- 其他 `rg "X-Trace-ID|HeaderTraceID|TraceID\\("` 命中的测试

测试应转向：

- 不再设置 `X-Trace-ID`。
- 不再断言响应 `X-Trace-ID`。
- 如需要验证 tracing，断言 OTel span context 有效或 `traceparent` 被采纳。
- 保留 auth、RBAC、panic envelope、健康探针日志跳过等原有行为断言。

## Documentation Updates

`docs/ARCHITECTURE.md`：

- HTTP middleware 链描述改为 OTel Gin tracing、panic recovery、request logging、CORS。
- Observability 当前状态改为用户服务已接入 Gin tracing middleware，但仍不实现 OTLP exporter、metrics exporter 或外部 client instrumentation。
- 删除 “HTTP trace header 为 `X-Trace-ID`” 当前契约，改为 W3C Trace Context。
- HTTP access log 字段仍可保留 `trace_id`，但来源改为当前 OTel span context。

`docs/DEVELOPMENT.md`：

- 本地 tracing 说明改为 `traceparent` / `tracestate`。
- 说明 `exporter: none` 下不会看到 trace UI，但 handler context 会有 OTel span context。
- 删除调试时依赖 `X-Trace-ID` 响应头的说法。

Historical change docs 可以只更新仍被当作当前契约引用的明显错误描述，例如 `docs/changes/add-user-service-http-flow-integration-tests/design.md` 中的响应 trace-id 断言，避免大规模重写历史记录。

## Verification

实现后运行：

```bash
cd user-service && go test ./internal/providers ./tests/e2e
```

```bash
make test-user-service
```

```bash
make architecture-lint
```

如修改 `common/http/middleware` 或 `common/runtime/logger`，还应运行：

```bash
cd common && go test ./http/middleware ./runtime/logger
```

最后扫描确认没有残留当前契约：

```bash
rg -n "X-Trace-ID|HeaderTraceID|TraceID\\(" common user-service docs AGENTS.md
```

允许保留的命中仅应是历史 change 文档中明确描述过去行为的内容；当前架构、开发文档、生产代码和测试不应再依赖它。
