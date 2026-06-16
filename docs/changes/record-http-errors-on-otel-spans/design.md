# Design

## Overview

本变更在共享 HTTP 边界补齐 OpenTelemetry span 错误语义，不改变 feature controller、application use case 或响应信封契约。

```text
Gin request context
  -> otelgin server span
  -> common/http/middleware.Recovery
  -> feature controller / response helper
  -> common/http/response.WriteError / Fail
  -> common/http/middleware.RequestLoggerWithOptions
  -> span error event / attributes / status
```

实现重点是将错误记录集中在 `common/http/middleware` 和 `common/http/response`，避免在 user/auth/role/permission controller 中逐个追加 OTel 逻辑。

## Current State

当前 HTTP 链路已经具备以下基础：

- `user-service/internal/providers/gin.go` 先安装 `otelgin.Middleware`，再安装 span rename、panic recovery、request logger 和 CORS。
- `common/runtime/logger` 已从当前 OTel span context 自动派生 `trace_id` 和 `span_id`。
- `common/http/middleware.Recovery` 在 panic 时写 error 日志、返回 `contracterrors.InternalError(nil)` 失败信封并 `Abort`。
- `common/http/middleware.RequestLoggerWithOptions` 在 `c.Next()` 后读取最终响应状态码，并按状态码选择 access log 级别：5xx 为 error，4xx 为 warn。
- `common/http/response.Fail` 会通过 `contracterrors.FromError` 将任意 error 映射为 `*contracterrors.Error`。
- `common/http/response.WriteError` 是已归一化应用错误写出失败信封的共享边界。

当前缺口是：日志与响应已经能表达错误，但 span 本身缺少一致的 error status、低基数错误属性和 sanitized error event。

## Span Semantics

本变更采用以下规则：

| 场景 | span status | span event | span attributes |
|---|---|---|---|
| panic | error | 记录 sanitized panic error | `http.status_code` 由 OTel/Gin 或现有 HTTP 语义保留；可补充 `error.type=panic` |
| 5xx 应用错误 | error | 记录 sanitized application error | `error_code`、`http_status` |
| 5xx 非应用错误响应 | error | 可不新增 error event | `http_status` |
| 4xx 应用错误 | 不设置 error status | 不记录 error event | `error_code`、`http_status` |
| 2xx/3xx | 不改动 | 不记录 | 不改动 |

普通 4xx 包括绑定失败、校验失败、未认证、token 无效、权限拒绝、资源不存在和业务冲突等预期拒绝。它们可以通过属性帮助排查，但不应被当成服务端异常。

## Helper Location

在 `common/http/middleware` 内新增包内 helper，或新增窄文件，例如 `span_error.go`：

```go
func recordPanicOnSpan(ctx context.Context, recovered any)
func markServerErrorStatus(ctx context.Context, status int)
func annotateAppErrorOnSpan(ctx context.Context, err *contracterrors.Error)
```

如果 `common/http/response` 需要直接调用应用错误标注 helper，可以将无业务语义的 helper 放到 `common/http/response` 包内，或在 `common/http/middleware` 与 `common/http/response` 之间选择一个不引入 import cycle 的位置。优先避免新增公开 API；只有测试或跨包调用确实需要时，再暴露小写不可行的最小公开函数，并保持注释为中文。

需要使用的 OTel API：

- `trace.SpanFromContext(ctx)`
- `span.RecordError(err, trace.WithAttributes(...))`
- `span.SetStatus(codes.Error, "...")`
- `span.SetAttributes(attribute.String(...), attribute.Int(...))`

`common` 已依赖 `go.opentelemetry.io/otel` 和 `go.opentelemetry.io/otel/trace`。实现时如需使用 `codes` 和 `attribute`，继续使用同一 OTel 版本下的官方 API，不引入 exporter 依赖。

## Panic Recovery

`common/http/middleware.Recovery` 是 panic 的唯一共享恢复边界。panic 处理顺序应保持清晰：

1. 从 `c.Request.Context()` 取得当前 span。
2. 将 panic 以 sanitized error 形式记录到 span。
3. 将 span status 设置为 error。
4. 写现有 panic 日志，保留 stacktrace 在日志中。
5. 通过 `response.Fail(c, contracterrors.InternalError(nil))` 写现有内部错误信封。
6. `c.Abort()`。

panic span event 不记录 `debug.Stack()` 原文。可将 `recovered` 转为短字符串或只记录 panic 类型，例如：

```go
err := fmt.Errorf("panic recovered")
span.RecordError(err,
    trace.WithAttributes(
        attribute.String("error.type", "panic"),
    ),
)
span.SetStatus(codes.Error, "panic recovered")
```

不将 panic payload 原文写入 span attribute，避免 panic 中包含 token、SQL、DSN 或请求数据。日志现有 `panic` 和 `stack` 字段保持不变；本变更不扩大日志内容。

## 5xx Status Marking

`RequestLoggerWithOptions` 已经位于 `c.Next()` 之后，能够读取最终 `c.Writer.Status()`，且当前会对 5xx 输出 error 级别 access log。因此优先在该函数中调用：

```go
if status >= 500 {
    markServerErrorStatus(c.Request.Context(), status)
}
```

标记语义：

- span 无效时 no-op。
- status 小于 500 时 no-op。
- 设置 `http_status` 或复用 `http.status_code` 时应避免和 OTel semantic convention 冲突；如果使用自定义字段，保持低基数命名 `http_status`。
- status description 使用短固定文本，例如 `"http server error"`，不要包含 path、query、用户 ID 或错误消息。

如果实现阶段发现 request logger skip 规则会跳过需要标记的 5xx，例如健康探针失败不应被 skip，可以先在 skip 之前执行 span status 标记。当前 `skipSuccessfulHealthProbeLog` 只跳过 `<400` 的健康探针，因此失败探针仍会被记录。

若后续确认 request logger 不适合承载 span status 标记，可新增 `ServerErrorSpanStatus` 这类轻量 middleware，并在 user-service Gin provider 中放在 request logger 附近。第一选择仍是复用已有 request logger，减少中间件数量和 provider 改动。

## Application Error Annotation

`common/http/response.WriteError` 是已归一化 `*contracterrors.Error` 的共享输出边界。这里可以在写 JSON 前标注当前 span：

```go
func WriteError(c *gin.Context, err *contracterrors.Error) {
    if err == nil {
        err = contracterrors.InternalError(nil)
    }
    annotateAppErrorOnSpan(c.Request.Context(), err)
    JSON(c, statusCode(err), errorEnvelope(err))
}
```

`Fail` 不需要重复标注；它先调用 `contracterrors.FromError(err)`，再走 `WriteError` 即可。如果实现阶段 `Fail` 保持直接 `JSON`，则必须确保同一应用错误只标注一次，避免重复 event。

推荐字段：

- `error_code`：`contracterrors.Code` 的整数值或稳定字符串形式。优先使用整数，和响应信封一致。
- `http_status`：最终 HTTP status。

5xx 应用错误可记录 sanitized error event：

```go
span.RecordError(errors.New(contracterrors.MessageInternalError),
    trace.WithAttributes(
        attribute.Int("error_code", int(appErr.Code)),
        attribute.Int("http_status", status),
    ),
)
span.SetStatus(codes.Error, "application error")
```

4xx 应用错误只设置 attributes，不调用 `RecordError`，不设置 `codes.Error`。这能让 trace 仍保留业务拒绝上下文，同时避免把预期调用方错误算作服务端异常。

不要把 `appErr.Message`、`appErr.Cause.Error()`、validation field details 或原始 `err.Error()` 写入 span。应用错误消息虽然面向客户端，但仍可能因未来调用方传入格式化内容而包含高基数或敏感片段；span 只保留稳定低基数字段。

## Validation Error Path

`ValidationFailedWithErrors` 当前直接写 400 信封，不构造 `contracterrors.Error`。可选择两种方式之一：

- 保持不标注字段级校验失败，因为它是普通 400 且字段明细可能高基数。
- 只标注 `error_code=CodeValidationFailed` 和 `http_status=400`，不记录字段列表或字段消息。

优先选择第二种，便于 trace 中识别校验失败类别，同时仍不记录敏感字段或原始请求体。

## Sensitive Data Rules

span attribute 和 event 必须保持低基数、无敏感内容：

- 不记录 password。
- 不记录 access token、refresh token、JWT、Authorization header 或 Cookie。
- 不记录原始请求体、query 原文或表单字段。
- 不记录 DSN、SQL 参数、Redis key 原文或外部 endpoint 凭据。
- 不记录 stacktrace 原文。
- 不记录完整 error message、panic payload 原文或 wrapped cause 原文。

允许记录：

- 固定枚举式 `error.type`，例如 `panic`、`application_error`。
- 稳定整数 `error_code`。
- 稳定整数 `http_status`。
- 固定短文本 span status description，例如 `panic recovered`、`http server error`、`application error`。

## Tests

Common middleware 测试应覆盖：

- panic recovery：
  - 使用 in-memory span recorder 或 test span exporter 创建真实 recording span。
  - 请求 panic endpoint。
  - 断言响应仍为 500 且 body 包含现有失败信封。
  - 断言 panic 日志仍写出且包含 trace/span 日志字段。
  - 断言 span status 为 error。
  - 断言 span events 中有 error event，但不包含 stacktrace 原文。
- 5xx request logger：
  - handler 返回 500。
  - 断言 access log 仍为 error 级别。
  - 断言 span status 为 error。
- 4xx request logger：
  - handler 返回 404 或通过 response helper 写 400。
  - 断言 access log 仍为 warn 级别。
  - 断言 span status 未被设置为 error。

Common response 测试应覆盖：

- `WriteError` 处理 5xx 应用错误时写入 `error_code`、`http_status`，设置 error status，并记录 sanitized error event。
- `WriteError` 处理 4xx 应用错误时写入 `error_code`、`http_status`，不设置 error status。
- `Fail` 处理 unknown error 时映射为 internal error，且 span 按 5xx 标记。
- `ValidationFailedWithErrors` 不记录字段明细、请求体或敏感名称到 span。

User-service provider 测试应覆盖：

- 实际 `NewGinEngine` middleware 顺序下，panic 路径的 span status 与现有 envelope/log 行为同时成立。
- 实际业务路由产生的普通 4xx 不被标记为系统异常。
- 如果现有 tests 已有 panic recovery 和 request log 覆盖，优先扩展它们，避免新建过宽集成测试。

## Verification

实现后建议运行：

```bash
cd common && go test ./http/middleware ./http/response
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

最后扫描确认没有敏感字段被作为 span attribute/event 写入：

```bash
rg -n "RecordError|SetAttributes|attribute\\." common user-service
```

并确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

## Risks

风险：重复设置 span status 或重复记录 error event。
缓解：让 `WriteError` 负责应用错误标注，让 request logger 只兜底标记最终 5xx status；不要在 feature controller 中手动记录。

风险：4xx 被误标为 span error，导致业务拒绝被外部 tracing 误判为系统异常。
缓解：在 helper 中用 `status >= 500` 作为设置 `codes.Error` 和 `RecordError` 的门槛，并增加 4xx 测试。

风险：span event 泄漏敏感错误消息或 stacktrace。
缓解：只记录固定短文本和低基数字段，不写 `err.Error()`、panic payload、stacktrace、请求体或 headers。

风险：引入 OTel semantic convention 字段名冲突。
缓解：优先使用当前 OTel API 已有 HTTP 属性，新增自定义字段保持 `error_code`、`http_status` 这种窄字段；如实现阶段选择 semantic convention 常量，确保版本和现有 `go.mod` 兼容。
