## Why

当前 HTTP request ID 已写入请求 `context.Context`，但通用日志 helper 只自动提取 `trace_id` 和 `span_id`，导致参数校验等应用日志缺少 `request_id`，而 access log 依赖 HTTP middleware 单独拼装该字段。同一请求内的日志关联行为不一致，增加了跨日志定位请求的成本，也造成 request ID 上下文与日志字段提取职责分散。

## What Changes

- 将 request ID 的日志关联上下文能力统一归属到 `common/runtime/logger`，使基于 `logger.WithContext`、`logger.FromContext` 和 `logger.Info|Warn|Error` 记录的请求内日志自动包含 `request_id`。
- 调整 HTTP Request ID middleware，通过 logger 提供的上下文 API 写入和读取 request ID，并继续保持 `X-Request-ID` 透传、生成和响应头行为不变。
- 删除 access log 对 `request_id` 的重复手工拼装，由通用 logger 上下文字段提取统一负责，避免重复字段和不同日志路径行为漂移。
- **BREAKING**：删除 `common/http/middleware` 中公开的 `RequestIDField`、`WithRequestID` 和 `RequestIDFromContext`，调用方必须迁移到 `common/runtime/logger` 中对应的公开符号，不提供别名、转发函数或弃用兼容期。
- 更新日志与 request ID 相关规格和测试，覆盖有效 span、无有效 span、参数校验失败和 access log 场景。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`：将 `request_id` 纳入通用日志 helper 的关联上下文字段，并明确请求内应用日志与 HTTP access log 使用同一 request ID 上下文来源。

## Impact

- 受影响代码：`common/runtime/logger/`、`common/http/middleware/request_id.go`、`common/http/middleware/log_fields.go`、`common/http/binding/` 相关测试，以及引用被删除 middleware API 的 user-service provider 测试。
- 共享契约：公开 Go API 的包归属发生不兼容变化，仓库内所有调用方必须原子迁移；不保留兼容入口。
- 观测行为：HTTP 请求处理期间通过通用 logger 记录的应用日志将新增稳定 `request_id` 字段；`trace_id`、`span_id`、logger name、日志 level、message 和 access log 请求字段保持不变。
- HTTP API：`X-Request-ID` 的输入校验、生成、透传和响应头行为不变；不修改业务 endpoint、响应 envelope 或状态码。
- 数据与交付：不涉及数据库 schema、Atlas migration、OpenAPI 生成物、依赖升级、部署清单、metrics 标签、dashboard 或安全授权边界。
