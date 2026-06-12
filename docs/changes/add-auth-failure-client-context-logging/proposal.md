# Add auth failure client context logging

## What

为认证失败相关日志补充稳定的客户端上下文字段，重点覆盖 `client_ip` 和 `user_agent`，并顺带收敛 HTTP access log 字段命名：

- 认证中间件中的认证失败日志增加 `client_ip`、`user_agent`、`method`、`path` 等请求上下文。
- 用户名密码登录失败路径通过 HTTP transport 注入安全审计上下文，使 application 层已有登录失败日志能够携带 `client_ip` 和 `user_agent`。
- HTTP request logging 将延迟字段标准化为 `latency_ms`，保留 `method`、`path`、`status`、`client_ip`、`user_id` 和 trace-id。
- 认证失败日志继续使用英文消息和稳定英文 snake_case 字段名。
- 不记录密码、token、Authorization header、Cookie、原始请求体或其他敏感认证材料。

本变更聚焦日志字段和安全审计上下文，不改变认证流程、HTTP API、响应 envelope、错误码、数据库 schema、Redis key 或配置结构。

## Why

认证失败是安全审计和运维排障的关键事件。当前 HTTP access log 已记录 `client_ip`，但认证中间件的失败日志没有直接携带 `client_ip` 和 `user_agent`；用户名密码登录失败日志位于 application 层，也缺少 transport 层客户端上下文。排查暴力破解、异常客户端、网关转发链路和用户反馈时，需要在 access log 与业务日志之间反复关联 trace-id，效率较低。

补充这些字段可以带来：

- 更快定位登录失败来源 IP、客户端类型和代理链路问题。
- 支持后续风控、告警和安全审计按客户端维度聚合。
- 减少只依赖 access log 反查的排障成本。
- 保持认证拒绝使用 `Info` 或 `Warn`，避免把预期客户端拒绝误报为系统错误。

## Scope

包括：

- 在 `common/http/middleware` 增加一个无业务语义的请求日志字段 helper，用于复用 `client_ip`、`user_agent`、`method` 和 `path` 提取逻辑。
- 更新 `common/http/middleware/auth.go`，为 missing header、invalid bearer、token validation failed、token version mismatch 等认证失败日志补充客户端上下文字段。
- 更新 `common/http/middleware/logging.go`，将 access log 的 `latency` duration 字段调整为 `latency_ms` 数值字段。
- 在 auth HTTP controller 登录入口把 `client_ip` 和 `user_agent` 作为审计上下文写入 request context。
- 在 auth application 登录失败日志中从 context 读取审计字段并输出，但不让 use case 依赖 Gin、HTTP DTO 或 request 对象。
- 更新相关单元测试断言，覆盖认证失败日志字段和 request logging 字段。
- 必要时更新 `docs/ARCHITECTURE.md` 与 `docs/DEVELOPMENT.md` 中的日志字段说明，使长期规则与实现一致。

不包括：

- 不新增 `openspec/` 或 `docs/opsx/`。
- 不引入新的日志库、SIEM、metrics、tracing exporter、WAF 或风控系统。
- 不记录明文密码、token、Authorization header、Cookie、Refresh Token、请求体或完整敏感用户标识。
- 不修改认证错误响应内容、HTTP status、错误码或业务拒绝语义。
- 不把 transport DTO 传入 application use case，也不让 application 层导入 Gin、HTTP binder 或 response envelope。
- 不新增数据库表、Redis key、Ent schema、migration 或配置项。

## Acceptance Criteria

- 认证失败日志包含 `client_ip` 和 `user_agent`，并在适用场景包含 `method`、`path`、`user_id`、`token_version` 等已有安全上下文。
- 用户名密码登录失败日志可以携带 `client_ip` 和 `user_agent`，但 application 层不依赖 Gin 或 HTTP request 类型。
- HTTP access log 使用 `latency_ms`，并继续包含 trace-id、`method`、`path`、`status`、`client_ip` 和 `user_id`。
- 认证失败仍按场景使用 `Info`、`Warn` 或 `Error`：预期认证拒绝不使用 `Error`，系统配置或依赖失败使用 `Error`。
- 日志消息保持英文，字段名保持稳定英文 snake_case。
- 测试覆盖认证中间件失败日志字段、登录失败审计字段和 access log `latency_ms`。
- `gofmt` 后相关 Go 测试通过，至少运行 `go test ./http/middleware` 和 `go test ./internal/features/auth/...`。
