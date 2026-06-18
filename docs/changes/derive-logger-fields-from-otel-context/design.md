# Design

## Overview

本变更将日志关联字段的唯一运行时来源收敛为当前 `context.Context` 中的 OpenTelemetry span context。

```text
OTel span context
  -> common/runtime/logger.WithContext(ctx, base)
  -> zap logger with trace_id/span_id
  -> logger.Info(ctx, ...), Warn(ctx, ...), Error(ctx, ...)
```

`common/runtime/logger` 继续负责 Zap logger 构造、默认 logger、context logger 和命名 logger。它不负责创建 span、不读取 HTTP header、不理解 Gin route，也不生成 fallback trace ID。用户服务、middleware、provider、Ent SQL debug 和业务 use case 只需要继续传递已有 `ctx`。

## Current State

当前 `common/runtime/logger/context.go` 已经导入 `go.opentelemetry.io/otel/trace`，但仍保留自定义 trace-id context key：

- `TraceIDField = "trace_id"`。
- `WithTraceID(ctx, traceID)` 将字符串写入私有 context key。
- `TraceIDFromContext(ctx)` 先读取私有 context key，再读取 `trace.SpanContextFromContext(ctx)`。
- `WithContext(ctx, base)` 始终追加 `zap.String("trace_id", TraceIDFromContext(ctx))`。
- `FromContext(ctx)` 通过 `WithContext` 返回带 `trace_id` 的 logger。

这意味着：

- 手动 `WithTraceID` 会覆盖 OTel span context。
- 无有效 span context 时会写出空字符串 `trace_id` 字段。
- 没有 `span_id` 字段。
- `common/runtime/logger/logger_test.go`、`user-service/internal/providers/ent_test.go` 等测试仍通过 `WithTraceID` 构造日志字段。

仓库当前规则已经要求 HTTP trace 传播使用 W3C `traceparent` / `tracestate`，日志字段 `trace_id` 来源于当前 OTel span context。因此 logger 实现需要补齐该规则。

## Field Derivation

在 `common/runtime/logger` 中新增稳定字段名：

```go
const (
    TraceIDField = "trace_id"
    SpanIDField  = "span_id"
)
```

新增一个包内 helper，用于从 context 派生 zap 字段：

```go
func fieldsFromContext(ctx context.Context) []zap.Field {
    if ctx == nil {
        return nil
    }
    spanContext := trace.SpanContextFromContext(ctx)
    if !spanContext.IsValid() {
        return nil
    }
    return []zap.Field{
        zap.String(TraceIDField, spanContext.TraceID().String()),
        zap.String(SpanIDField, spanContext.SpanID().String()),
    }
}
```

实现细节：

- 使用 `spanContext.IsValid()` 作为整体有效性判断。
- 不再读取私有 context value。
- 不生成 UUID、随机 ID 或空字符串 fallback。
- `ctx == nil`、无 span、无效 span 时返回 `nil`，日志照常输出。
- `trace_id` 和 `span_id` 字段值使用 OTel 标准 hex string。

## Logger API Changes

`WithContext` 改为只在存在有效 OTel span context 时追加字段：

```go
func WithContext(ctx context.Context, base *zap.Logger) *zap.Logger {
    if base == nil {
        base = getDefault()
    }
    if fields := fieldsFromContext(ctx); len(fields) > 0 {
        return base.With(fields...)
    }
    return base
}
```

`FromContext` 保持现有使用方式：

- 如果 context 中存在 `logger.ToContext` 写入的 logger，则以它为 base。
- 否则使用 default logger。
- 最终统一调用 `WithContext(ctx, base)`。

`Debug`、`Info`、`Warn`、`Error` 保持签名和 caller skip 语义不变。

`SQL(base)` 保持只返回命名 logger，不自行追加 trace/span 字段。Ent SQL debug callback 已经拿到 `context.Context`，继续通过 `logger.WithContext(ctx, logger.SQL(base))` 在 SQL 日志里追加 OTel 字段。

## Removing Custom Trace ID Helpers

目标状态是删除：

- `traceIDContextKey`
- `WithTraceID`
- `TraceIDFromContext`

如果实现阶段发现一次删除造成大面积测试迁移成本过高，可先将 `WithTraceID` 和 `TraceIDFromContext` 标记为 deprecated，但同一 change 的任务必须继续完成直接调用方迁移，并在最终扫描中确保生产代码和测试不再依赖它们。完成迁移后优先删除，避免保留第二套日志关联入口。

不新增 `WithSpanID`、`TraceFieldsFromContext` 等供业务代码调用的公开 API。业务代码只传 `ctx` 给 logger helper。

## Common Middleware Impact

`common/http/middleware` 的 request logger 和 recovery 不需要直接依赖 OTel API。它们继续：

- 使用请求的 `c.Request.Context()`。
- 调用 `logger.WithContext(ctx, log)` 或 `logger.Info(ctx, ...)`。
- 追加 method/path/status/client_ip/user_id/latency_ms/panic/stack 等自身字段。

只要 Gin OTel middleware 已经将有效 server span 写入 request context，request logger 和 recovery 日志会自动获得 `trace_id` 与 `span_id`。

健康探针若被 OTel Gin filter 跳过，则 request context 没有有效业务 server span。此时健康探针日志如果被记录，不应出现伪造 trace/span 字段。

## User Service Provider Impact

`user-service/internal/providers/gin.go` 已经通过 OTel Gin middleware 创建 server span，并在 request logger 与 recovery 之前执行。该顺序应保持：

```text
otelgin.Middleware
renameHTTPServerSpan
Recovery
RequestLogger
CORS
```

本变更不需要 feature 层改动。Provider 层相关测试需要从“日志字段是某个手写 trace id”改为：

- `trace_id` 是有效 OTel trace ID。
- `span_id` 是有效 OTel span ID。
- 带 `traceparent` 的请求日志使用 parent trace ID。
- 无 `traceparent` 的请求日志生成新的有效 trace ID 与 span ID。
- 无有效 span context 的日志不包含 `trace_id` 和 `span_id`，或至少不包含空字符串字段。

## Ent SQL Debug Impact

`user-service/internal/providers/ent.go` 的 `entSQLDebugLogFunc` 当前通过：

```go
logger.WithContext(ctx, log).Info("ent sql debug", ...)
```

输出 SQL debug 日志。该实现可以保留。

测试应迁移为使用真实 OTel span context：

- 构造有效 `trace.SpanContext`。
- 写入 `context.Context`。
- 调用 SQL debug log func。
- 断言 SQL log 包含 `trace_id`、`span_id` 和 `statement`。

不要通过 `logger.WithTraceID` 构造 SQL debug 测试 context。

## Test Context Helper

测试可以新增测试内 helper，用 OTel API 构造固定 trace/span context：

```go
func contextWithSpanContext(t *testing.T, ctx context.Context, traceIDHex, spanIDHex string) context.Context {
    t.Helper()
    traceID, err := trace.TraceIDFromHex(traceIDHex)
    if err != nil {
        t.Fatalf("TraceIDFromHex: %v", err)
    }
    spanID, err := trace.SpanIDFromHex(spanIDHex)
    if err != nil {
        t.Fatalf("SpanIDFromHex: %v", err)
    }
    spanContext := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID: traceID,
        SpanID:  spanID,
        Remote:  true,
    })
    return trace.ContextWithSpanContext(ctx, spanContext)
}
```

该 helper 只用于测试，不进入生产代码。测试中固定 trace/span ID 便于精确断言。

## Invalid Span Behavior

无效 span context 的期望行为是“日志正常输出，不伪造字段”：

- `WithContext(context.Background(), log)` 返回可用 logger。
- `logger.Info(context.Background(), ...)` 可以输出消息。
- 日志 context map 中不包含 `trace_id` 和 `span_id`，或者文件日志中没有 `"trace_id":""` / `"span_id":""`。

选择“省略字段”而不是“写空字符串”的原因：

- 空字符串字段容易被日志检索误判为有 trace 字段但值缺失。
- 省略字段更准确表达当前 context 没有可关联的 OTel span。
- 避免下游索引中大量空值污染字段统计。

## Documentation Updates

需要更新当前规则文档中的日志描述：

- `AGENTS.md`：说明 `common/runtime/logger` context helper 会从 OTel span context 自动追加 `trace_id` 与 `span_id`。
- `docs/ARCHITECTURE.md`：日志基础设施说明补充 `span_id`，并明确无有效 span 时不伪造 ID。
- `docs/DEVELOPMENT.md`：本地调试说明从“日志字段 trace_id 优先来自 OTel”调整为“trace_id/span_id 均来自 OTel span context”。

历史 change 文档只在它们仍被作为当前行为参考、且会误导实现时做小范围更新。不要为了重写历史而大规模改动旧 proposal/design/tasks。

## Verification

实现后建议运行：

```bash
cd common && go test ./runtime/logger ./http/middleware
```

```bash
cd user-service && go test ./internal/providers
```

```bash
make test-common
```

```bash
make test-user-service
```

```bash
make architecture-lint
```

最后扫描确认没有遗留自定义 trace-id helper 依赖：

```bash
rg -n "WithTraceID|TraceIDFromContext|traceIDContextKey" common user-service
```

并确认没有重新引入旧 HTTP trace header：

```bash
rg -n "X-Trace-ID|HeaderTraceID" common user-service docs AGENTS.md
```
