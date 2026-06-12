# Design

## Overview

本变更将认证失败日志拆成两类观测面：

```text
HTTP access log
  -> 每个请求统一记录 method/path/status/latency_ms/client_ip/user_id/trace_id

security event log
  -> 认证拒绝、token 失败、登录失败等安全事件记录 client_ip/user_agent 和原因上下文
```

这样可以保持 access log 字段稳定，又让认证失败日志具备足够安全审计信息。实现上复用 request context，而不是让 application use case 直接接收 HTTP DTO 或 Gin context。

## Current State

当前相关实现：

- `common/http/middleware/logging.go` 已记录 `method`、`path`、`status`、`latency`、`client_ip` 和 `user_id`。
- `common/http/middleware/auth.go` 已记录 missing authorization header、invalid bearer、token validation failed 和 token version mismatch，但缺少 `client_ip` 和 `user_agent`。
- `user-service/internal/features/auth/application/command/credentials.go` 已记录 `login user not found`、`login password mismatch` 和 `login user status rejected`，但这些日志在 application 层，无法直接读取 HTTP request。
- `docs/ARCHITECTURE.md` 当前同时提到 `trace-id` 和 snake_case 字段规则，需要在实现时统一为稳定字段名说明。

## Field Standard

HTTP access log 标准字段：

| 字段 | 类型 | 来源 | 说明 |
|---|---|---|---|
| `trace_id` | string | `common/runtime/logger` context helper | 请求关联 ID |
| `user_id` | string | auth context，未认证为 `anonymous` | 认证用户 ID |
| `client_ip` | string | `gin.Context.ClientIP()` | 受 trusted proxies 配置影响的客户端 IP |
| `method` | string | `c.Request.Method` | HTTP method |
| `path` | string | 优先 route pattern，降级 URL path | 请求路径 |
| `status` | int | `c.Writer.Status()` | HTTP status |
| `latency_ms` | int64 | `time.Since(start).Milliseconds()` | 请求耗时毫秒 |

认证失败安全日志额外字段：

| 字段 | 类型 | 来源 | 说明 |
|---|---|---|---|
| `user_agent` | string | `User-Agent` header | 客户端标识，允许为空 |
| `reason` | string | 枚举或日志消息 | 失败原因分类，避免泄露底层敏感细节 |
| `username` | string | 登录请求规范化后的 username | 仅在现有日志已经记录 username 的路径保留 |
| `user_id` | string | token claims 或 credential | 已知用户时记录 |
| `token_version` | int64 | token claims 或 credential | 已知 token version 时记录 |

安全日志明确禁止字段：

- `password`
- `access_token`
- `refresh_token`
- `authorization`
- `cookie`
- 原始请求体
- 未脱敏的其他高敏凭据

## Request Metadata Helper

在 `common/http/middleware` 内新增无业务语义 helper，集中构造 HTTP 日志字段：

```go
func requestLogFields(c *gin.Context) []zap.Field
func authFailureLogFields(c *gin.Context) []zap.Field
```

建议职责：

- `requestLogFields` 返回 access log 字段，不包含 `user_agent`，避免每条请求日志变得过宽。
- `authFailureLogFields` 返回认证失败安全事件字段，包含 `client_ip`、`user_agent`、`method` 和 `path`。
- `path` 优先使用 `c.FullPath()`，当路由未匹配或为空时降级为 `c.Request.URL.Path`。
- `client_ip` 继续使用 Gin 的 `ClientIP()`，由服务级 trusted proxy 配置控制可信代理解析。

该 helper 放在 `common/http/middleware`，因为它只依赖 Gin request metadata，不含 user-service feature 业务语义。

## Auth Middleware Logging

更新 `common/http/middleware/auth.go`：

- missing authorization header：
  - 保持 `Info`。
  - 增加 `client_ip`、`user_agent`、`method`、`path`。
- invalid authorization header format、empty bearer token：
  - 保持 `Warn`。
  - 增加客户端上下文字段。
- token validation failed：
  - JWT secret 缺失等系统配置问题仍为 `Error`。
  - 过期、格式错误、subject 不允许等认证拒绝仍为 `Warn`。
  - 增加客户端上下文字段，但不记录 token 原文。
- token version mismatch：
  - 保持 `Warn`。
  - 保留 `user_id`、`current_token_version`、`token_version`。
  - 增加客户端上下文字段。
- token version validation failed：
  - 保持 `Error`。
  - 增加客户端上下文字段。

## Login Failure Logging

用户名密码登录失败发生在 auth application 的 credential verifier 中。为了保留分层边界，transport/http 不把 Gin context 或 HTTP DTO 传入 use case，而是把审计元数据放入 request context。

建议在 auth application command 包内定义 transport-neutral context helper：

```go
type ClientContext struct {
	ClientIP  string
	UserAgent string
}

func WithClientContext(ctx context.Context, meta ClientContext) context.Context
func ClientContextFromContext(ctx context.Context) (ClientContext, bool)
```

放置位置：

- 推荐放在 `user-service/internal/features/auth/application/command/client_context.go`，因为当前只有 auth 登录 use case 消费该上下文。
- 如果后续多个 auth command 需要同一语义，再提升到 auth application 根包。
- 不放入 `common`，因为目前它服务 auth 安全审计，不是跨服务稳定 runtime primitive。

HTTP controller 只在调用 `Login` 前注入：

```go
ctx := authcommand.WithClientContext(c.Request.Context(), authcommand.ClientContext{
    ClientIP: c.ClientIP(),
    UserAgent: c.GetHeader("User-Agent"),
})
tokens, err := ctl.login.Login(ctx, ...)
```

application 登录日志使用 helper 追加字段：

```go
func clientContextFields(ctx context.Context) []zap.Field
```

适用日志：

- `login user not found`
- `login password mismatch`
- `login user status rejected`
- 可选覆盖 `login user` 和 `login user authenticated`，但最小实现优先覆盖失败日志。

## Privacy And Security

本变更只记录安全审计需要的低敏上下文。注意事项：

- `User-Agent` 可被客户端伪造，只作为排障和聚合信号，不作为可信身份依据。
- `client_ip` 依赖 trusted proxy 配置；生产部署必须正确配置可信代理，否则不能把 forwarded header 视为真实客户端地址。
- 登录用户名当前已有日志记录，本变更不扩大用户名记录范围。后续如需强化隐私，可单独设计 `login_identifier_hash` 替代明文 username。
- 不记录 token、密码、Authorization header 或 Cookie，避免日志系统成为凭据泄漏面。

## Testing Strategy

需要更新或新增测试：

- `common/http/middleware/auth_test.go`
  - 认证失败日志断言包含 `client_ip` 和 `user_agent`。
  - token version mismatch 保留版本字段并包含客户端上下文。
  - JWT secret 缺失仍为 `Error`。
- `common/http/middleware/trace_id_test.go`
  - request log 字段从 `latency` 改为 `latency_ms`。
  - `client_ip` 现有断言保持通过。
- `user-service/internal/providers/routes_test.go`
  - request log 字段从 `latency` 改为 `latency_ms`。
- `user-service/internal/features/auth/transport/http/controller_test.go` 或 command 测试
  - 登录失败时 application 日志包含通过 controller 注入的 `client_ip` 和 `user_agent`。
  - 若现有测试不捕获 logger，可优先测试 context helper 的写入读取，再在 controller 层补一个窄日志断言。

验证命令：

```bash
cd common && go test ./http/middleware
cd user-service && go test ./internal/features/auth/... ./internal/providers
```

如修改长期文档，还需确认：

```bash
rg -n "latency|latency_ms|client_ip|user_agent|trace-id|trace_id" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md common user-service
```

## Risks And Mitigation

风险：日志字段过宽，导致普通 access log 存储成本上升。

缓解：`user_agent` 只进入认证失败安全事件日志，不默认进入每条 access log。

风险：application 层被 transport 细节污染。

缓解：只通过 context 传递 transport-neutral `ClientContext`，application 不导入 Gin、HTTP request、HTTP DTO 或 response helper。

风险：`client_ip` 在反向代理后不可信。

缓解：继续依赖 Gin trusted proxies 配置，并在文档中说明生产配置要求。

风险：记录 username 可能带来隐私顾虑。

缓解：本变更不新增 username 记录范围；后续如需要合规强化，单独设计 hash 化登录标识。

风险：字段名迁移影响日志查询。

缓解：在 tasks 中明确更新测试和文档；如生产查询依赖 `latency`，落地时可选择短期双写 `latency_ms` 和 `latency`，但最终标准字段为 `latency_ms`。
