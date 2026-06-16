# Record HTTP errors on OTel spans

## What

将 HTTP panic、5xx 响应和可识别的应用错误记录到当前 OpenTelemetry span，提升日志、响应信封与 trace 的错误关联能力。

本变更会在现有 Gin HTTP 边界补充 span 错误语义：

- panic recovery 在保持现有内部错误响应信封和 panic 日志行为的同时，调用当前 span 的 `RecordError`，并将 span status 标记为 error。
- HTTP 请求完成时，如果最终状态码为 5xx，将当前 span status 标记为 error。
- `common/contract/errors.Error` 映射出的应用错误在共享响应输出边界写入低基数字段，例如 `error_code`、`http_status`。
- 对 5xx 应用错误记录 error event；对普通 4xx 业务拒绝只记录受控属性，不把它们统一标记为系统异常。
- 测试覆盖 panic、5xx 和普通 4xx 路径，确认 span status、错误事件和日志行为符合预期。

## Why

用户服务已经通过 OTel Gin middleware 为 HTTP 请求创建 server span，并通过 `common/runtime/logger` 从当前 span context 自动派生 `trace_id` 和 `span_id`。现在日志可以关联到 trace，但 trace 本身还缺少足够的错误语义：panic、5xx 和应用错误响应主要体现在日志和响应信封中，span status 与 span event 不一定能表达同一失败。

这会带来几个问题：

- trace UI 或 span recorder 中无法直接区分正常 4xx 业务拒绝和真正的服务端失败。
- panic recovery 虽然返回统一内部错误信封并写 error 日志，但当前 span 未必记录错误事件。
- 5xx 响应如果不是 panic 触发，可能只在 access log 中体现为 error 级别。
- 应用错误码、HTTP status 和 span 之间缺少稳定、低基数字段，排障时需要在日志和响应之间手工拼线索。

本变更把 HTTP 错误语义补齐到 span 上，但不改变客户端响应、不改变日志敏感字段规则，也不引入新的错误上报后端。

## Scope

包括：

- 修改 `common/http/middleware/recovery.go`：
  - panic 时调用当前 span 的 `RecordError`。
  - panic 时将 span status 设置为 error。
  - 继续返回现有 `contracterrors.InternalError(nil)` 失败信封。
  - 继续记录现有 panic 日志，不改变日志级别。
- 在 HTTP request logger 或轻量共享 middleware 边界补充 5xx span status 标记：
  - 优先复用 `common/http/middleware.RequestLoggerWithOptions`，因为它已经在 `c.Next()` 后读取最终状态码。
  - 如果实现阶段发现 request logger 语义不适合承载 span 标记，则新增无业务语义的轻量 middleware，专门处理 span status。
- 修改共享响应输出边界，优先选择 `common/http/response.WriteError` / `Fail`：
  - 将可识别 `common/contract/errors.Error` 的 `error_code`、`http_status` 写入当前 span attribute。
  - 对 5xx 应用错误记录 sanitized error event，并设置 span status error。
  - 对普通 4xx 应用错误保留属性，不将 span status 标记为 error。
  - `ValidationFailedWithErrors` 这类字段级 400 校验失败可只写 `error_code` 和 `http_status`，不记录字段明细到 span。
- 增加或调整测试，覆盖：
  - panic 请求返回现有内部错误信封，span status 为 error，记录 error event。
  - 非 panic 5xx 响应会标记 span error status。
  - 由 `contracterrors.Error` 输出的 4xx 应用错误携带低基数字段，但不被误判为系统异常。
  - span attribute 和 event 不包含 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 参数或 stacktrace 原文。
- 运行 `make test-common` 和 `make test-user-service`。

不包括：

- 不引入 Sentry、Rollbar、Datadog 或其他错误上报后端。
- 不实现 OTLP exporter、Collector 部署、trace UI、dashboard 或告警。
- 不新增 metrics 指标。
- 不改变客户端响应消息、错误码、响应信封结构或 HTTP status。
- 不把预期业务拒绝统一标记为系统异常。
- 不记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 参数或 stacktrace 原文到 span。
- 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- panic 请求仍返回现有内部错误响应信封，且当前 span 记录 error event，span status 为 error。
- 非 panic 5xx 请求在当前 span 上标记 error status。
- 普通 4xx 业务拒绝不会被误判为系统异常，不应统一设置 span status error。
- `common/contract/errors.Error` 映射出的应用错误在可行范围内写入 `error_code`、`http_status` 等低基数字段。
- 错误相关 span attribute 和 event 不包含 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 参数或 stacktrace 原文。
- 现有统一 response envelope、HTTP status、错误码和日志安全规则保持不变。
- `make test-common` 通过。
- `make test-user-service` 通过。
