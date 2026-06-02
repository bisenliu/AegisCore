# user-authentication

## Purpose

用户认证能力为 HTTP API 提供可复用的 JWT Bearer 认证中间件，统一处理认证校验、失败响应和认证用户身份在请求上下文中的传播。

## Requirements

### Requirement: Authenticate HTTP requests with JWT Bearer tokens

系统 MUST 提供可复用的 Gin 认证中间件，用于校验 `Authorization: Bearer <token>` 请求头。中间件 MUST 支持配置化白名单路径；非白名单请求缺少认证头、认证头格式错误、token 为空或 token 无效时，系统 MUST 返回 HTTP 401，并使用 `common/response.Envelope` 失败格式与未认证数字业务码 `20000`。

#### Scenario: Skip whitelisted path
- **Given** auth 配置白名单包含 `/healthz`
- **When** 调用方请求 `GET /healthz` 且未携带 `Authorization` header
- **Then** 认证中间件 MUST 放行该请求
- **Then** 系统 MUST NOT 返回未认证错误

#### Scenario: Reject missing authorization header
- **Given** 请求路径不在 auth 白名单中
- **When** 调用方未携带 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应信封的 `success` MUST 为 `false`
- **Then** 响应信封的 `code` MUST 为 `20000`

#### Scenario: Reject invalid bearer format
- **Given** 请求路径不在 auth 白名单中
- **When** 调用方携带不以 `Bearer ` 开头的 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 MUST 使用统一失败响应信封

#### Scenario: Reject empty bearer token
- **Given** 请求路径不在 auth 白名单中
- **When** 调用方携带 `Authorization: Bearer `
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 调用后续业务 handler

#### Scenario: Reject invalid token
- **Given** 请求路径不在 auth 白名单中
- **When** 调用方携带无法通过 JWT 解析、签名校验或标准 claims 校验的 token
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 认证失败日志 MUST NOT 记录 token 原文

### Requirement: Log authentication decisions with caller-provided logger

系统 MUST 要求 Gin 认证中间件由调用方显式传入 Zap logger。认证中间件 MUST 使用该 logger 记录白名单放行、认证头缺失、认证头格式错误、空 bearer token 和 token 校验失败等认证决策日志，并 MUST 继续通过请求 `context.Context` 保留 `trace-id` 日志字段。认证失败日志 MUST NOT 记录 token 原文。

#### Scenario: Log whitelisted path with provided logger
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **Given** auth 配置白名单包含 `/healthz`
- **When** 请求路径为 `/healthz`
- **Then** 认证中间件 MUST 使用调用方传入的 logger 记录白名单放行日志
- **Then** 该请求 MUST 继续执行后续 handler

#### Scenario: Log authentication failure with provided logger
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **Given** 请求路径不在 auth 白名单中
- **When** 调用方未携带有效 `Authorization: Bearer <token>` 请求头
- **Then** 认证中间件 MUST 使用调用方传入的 logger 记录认证失败日志
- **Then** 认证失败日志 MUST NOT 包含 token 原文
- **Then** 系统 MUST 返回现有 HTTP 401 失败响应信封

#### Scenario: Preserve trace id in authentication logs
- **Given** 请求 context 中存在 trace id
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **When** 认证中间件输出认证相关日志
- **Then** 日志 MUST 包含请求 context 对应的 `trace-id` 字段

### Requirement: Parse JWT tokens using v5 dependency

系统 MUST 使用 `github.com/golang-jwt/jwt/v5` 解析 JWT token，并 MUST 校验签名方法与过期时间。配置声明 issuer 或 audience 时，系统 MUST 对应校验 issuer 或 audience；未配置 issuer 或 audience 时，系统 MUST NOT 因这些字段为空而拒绝 token。

#### Scenario: Accept valid token
- **Given** auth 配置包含 JWT secret
- **Given** token 使用允许的签名方法签名且未过期
- **Given** token claims 包含非空 `user_id`
- **When** 认证中间件解析该 token
- **Then** 系统 MUST 将请求视为已认证
- **Then** 系统 MUST 继续执行后续 handler

#### Scenario: Reject expired token
- **Given** token 的过期时间早于当前时间
- **When** 认证中间件解析该 token
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 将该请求标记为已认证

#### Scenario: Validate configured issuer and audience
- **Given** auth 配置声明 issuer 和 audience
- **When** token 的 issuer 或 audience 与配置不匹配
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 执行业务 handler

### Requirement: Propagate authenticated user identity through request contexts

系统 MUST 将认证通过后的用户 ID 写入 Gin context 和 `c.Request.Context()`，并使用稳定的 context key 表示当前认证用户。后续 handler MUST 能通过该 key 读取认证用户 ID。

#### Scenario: Store user id after successful authentication
- **Given** token claims 包含 `user_id: "u-123"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前用户 ID `u-123`
- **Then** Go `context.Context` MUST 包含当前用户 ID `u-123`
- **Then** 后续 handler MUST 能读取该用户 ID

#### Scenario: Do not store user id on failed authentication
- **Given** token 无效或缺少非空 `user_id`
- **When** 认证中间件拒绝请求
- **Then** Gin context MUST NOT 被标记为已认证用户
- **Then** 系统 MUST NOT 执行业务 handler
