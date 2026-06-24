## Why

当前 HTTP 日志已经能从 OpenTelemetry span context 记录 `trace_id` 和 `span_id`，但服务没有生成或透传调用方可见的请求标识。客户端、网关和工单排障无法稳定拿到一次请求的关联 ID；在 tracing 被过滤、未导出或调用方未接入 W3C TraceContext 时，也缺少轻量的日志关联字段。

## What Changes

- 新增共享 Gin middleware，用于读取入站 `X-Request-ID`、在缺失时生成请求 ID，并在响应头回传同名 header。
- 将请求 ID 写入 request context，使 access log 使用稳定 `request_id` 字段输出该值。
- 在 user-service 的 Gin engine 中安装该 middleware，使业务 API 和运行时端点获得统一行为。
- 更新 runtime observability 规格，明确请求 ID 的生成、透传、响应回传、日志关联和 metrics 低基数约束。
- 不改变现有 `traceparent` / `tracestate` 传播行为，不改变 HTTP response envelope，不引入数据库、OpenAPI 或部署资产变更。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `runtime-observability`: HTTP 日志与错误可观测性增加 `X-Request-ID` 生成、透传、响应回传和 `request_id` 日志字段要求。

## Impact

- 影响 `common/http/middleware/`：新增无业务语义 request ID middleware、context helper 和测试，并扩展请求日志字段。
- 影响 `user-service/internal/providers/gin.go`：在全局 Gin middleware 链中安装 request ID middleware。
- 影响 `user-service/internal/providers/gin_test.go` 与 `common/http/middleware/logging_test.go`：补充 header 透传、生成、响应回传和日志字段测试。
- 影响 `openspec/changes/add-request-id-middleware/specs/runtime-observability/spec.md`：新增规格 delta。
- 不影响数据库 schema、Ent/Atlas migration、OpenAPI 生成物、RBAC 授权模型、Prometheus metrics label 或部署配置。
