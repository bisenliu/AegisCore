# Derive logger fields from OTel context

## What

将 `common/runtime/logger` 的请求关联字段来源收敛到 OpenTelemetry span context。

本变更会让所有通过 `common/runtime/logger` context helper 输出的日志，在 `context.Context` 中存在有效 OTel span context 时自动携带：

- `trace_id`：保留既有字段名，值改为当前 OTel trace ID。
- `span_id`：新增字段，值为当前 OTel span ID。

同时移除 logger 对自定义 trace-id context key 的依赖，清理 `WithTraceID`、`TraceIDFromContext` 及其直接调用方。业务代码继续使用 `logger.Info(ctx, ...)`、`logger.Warn(ctx, ...)`、`logger.Error(ctx, ...)`，不直接读取 OTel API 来拼日志字段。

## Why

用户服务已经将 HTTP trace 传播规则迁移到 W3C `traceparent` / `tracestate`，并由 OTel Gin middleware 在请求 context 中创建 server span。当前 logger helper 仍保留自定义 trace-id context key 的兼容路径，且只输出 `trace_id`，没有输出 `span_id`。

这会带来几个问题：

- `trace_id` 的来源可能仍被手动 context value 覆盖，和真实 OTel span context 不一致。
- 只有 trace ID 无法定位同一 trace 内的具体 span，排查跨 middleware、provider、Ent SQL debug 和业务 use case 日志时粒度不足。
- 测试仍可以通过 `logger.WithTraceID` 人工制造日志关联字段，掩盖 handler context 未携带有效 OTel span 的问题。
- 自定义 trace-id helper 继续存在，会让后续开发误以为仍有一套非 OTel 日志关联机制。

本变更把日志关联语义统一到 OTel context，降低长期维护成本，也让日志与未来 OTLP exporter、跨服务 tracing 和外部 client instrumentation 自然对齐。

## Scope

包括：

- 修改 `common/runtime/logger.WithContext` 和 `FromContext` 的字段派生逻辑，从 `trace.SpanContextFromContext(ctx)` 提取 `trace_id` 和 `span_id`。
- 保留 `trace_id` 字段名，新增 `span_id` 字段常量。
- 当 span context 有效时同时输出 `trace_id` 与 `span_id`。
- 当 span context 无效时仍正常输出日志，但不生成、不伪造、不写入空字符串形式的 trace/span 字段。
- 删除或废弃 `WithTraceID`、`TraceIDFromContext`，并迁移仓库内直接调用方。
- 更新 logger 单元测试，覆盖有效 OTel context、无效 OTel context、context logger、default logger 和 caller skip。
- 更新 `common/http/middleware` 相关测试，确保 request logger、panic recovery 通过 OTel context 输出 `trace_id` 与 `span_id`。
- 更新 `user-service/internal/providers` 相关测试，特别是 request log、panic recovery 和 Ent SQL debug 的日志字段断言。
- 如文档仍描述 logger 只携带 `trace_id` 或来自旧 helper，更新为 OTel span context 来源。

不包括：

- 不保留 `X-Trace-ID` 兼容路径。
- 不新增 `Trace-Id`、`X-Trace-ID` 或其他响应头。
- 不改变日志等级策略、日志文件轮转、日志编码格式、日志目录或命名 logger 规则。
- 不改变 HTTP access log 的既有业务字段集合，除通过 logger helper 自动追加 `span_id` 外不调整 method/path/status/user_id/client_ip/latency_ms 语义。
- 不在业务代码中直接依赖 OTel API 读取日志字段。
- 不新增日志采样。
- 不实现 OTLP exporter、metrics exporter、dashboard 或告警。
- 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- OTel span context 有效时，通过 `logger.WithContext`、`logger.FromContext`、`logger.Info(ctx, ...)`、`logger.Warn(ctx, ...)`、`logger.Error(ctx, ...)` 输出的日志包含 `trace_id` 和 `span_id`。
- `trace_id` 的值来自 `trace.SpanContextFromContext(ctx).TraceID().String()`。
- `span_id` 的值来自 `trace.SpanContextFromContext(ctx).SpanID().String()`。
- OTel span context 无效时，日志仍能正常输出，且不伪造 trace ID 或 span ID。
- `WithTraceID`、`TraceIDFromContext` 不再被生产代码或测试直接依赖；若实现阶段选择先 deprecated，任务中必须包含同一变更内的调用方迁移和删除检查。
- HTTP request logger、panic recovery 和 Ent SQL debug 日志能通过 OTel context 自动携带 `trace_id` 与 `span_id`。
- 现有业务代码继续通过 `logger.Info(ctx, ...)`、`logger.Warn(ctx, ...)`、`logger.Error(ctx, ...)` 使用日志，不需要在业务层手动追加 trace/span 字段。
- `common` 受影响测试通过。
- `user-service` 受影响测试通过。
- `make architecture-lint` 通过。
