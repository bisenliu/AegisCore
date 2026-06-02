## MODIFIED Requirements

### Requirement: Authenticate HTTP requests with JWT Bearer tokens
系统 MUST 提供可复用的 Gin 认证中间件，用于校验 `Authorization: Bearer <token>` 请求头。中间件 MUST 支持配置化白名单路径；非白名单请求缺少认证头、认证头格式错误、token 为空或 token 无效时，系统 MUST 返回 HTTP 401，并使用 `common/response.Envelope` 失败格式与未认证数字业务码 `20000`。当 token 通过签名、过期时间、issuer 和 audience 校验后，中间件 MUST 解析 `user_id`、`token_version` 和 `session_id`，并将 token 中的 `token_version` 与服务端当前版本比较；版本不一致时系统 MUST 将该 token 视为无效。

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

#### Scenario: Reject mismatched token version
- **Given** 请求路径不在 auth 白名单中
- **Given** Access Token 签名有效且未过期
- **Given** Access Token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方携带该 Access Token 请求受保护 API
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 将该请求标记为已认证

### Requirement: Parse JWT tokens using v5 dependency
系统 MUST 使用 `github.com/golang-jwt/jwt/v5` 解析 JWT token，并 MUST 校验签名方法与过期时间。配置声明 issuer 或 audience 时，系统 MUST 对应校验 issuer 或 audience；未配置 issuer 或 audience 时，系统 MUST NOT 因这些字段为空而拒绝 token。用于访问受保护业务接口的 Access Token claims MUST 包含非空 `user_id`、大于零的 `token_version` 和非空 `session_id`。

#### Scenario: Accept valid token
- **Given** auth 配置包含 JWT secret
- **Given** token 使用允许的签名方法签名且未过期
- **Given** token claims 包含非空 `user_id`
- **Given** token claims 包含大于零的 `token_version` 和非空 `session_id`
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

#### Scenario: Reject missing token version or session id
- **Given** token 签名有效且未过期
- **Given** token claims 缺少 `token_version` 或 `session_id`
- **When** 认证中间件解析该 token
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 执行业务 handler

### Requirement: Propagate authenticated user identity through request contexts
系统 MUST 将认证通过后的用户 ID 写入 Gin context 和 `c.Request.Context()`，并使用稳定的 context key 表示当前认证用户。系统 MUST 同时传播当前认证会话标识，使后续 handler 能读取当前 `session_id` 以执行退出当前设备等会话控制操作。

#### Scenario: Store user id after successful authentication
- **Given** token claims 包含 `user_id: "u-123"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前用户 ID `u-123`
- **Then** Go `context.Context` MUST 包含当前用户 ID `u-123`
- **Then** 后续 handler MUST 能读取该用户 ID

#### Scenario: Store session id after successful authentication
- **Given** token claims 包含 `session_id: "s-123"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前会话 ID `s-123`
- **Then** Go `context.Context` MUST 包含当前会话 ID `s-123`
- **Then** 后续 handler MUST 能读取该会话 ID

#### Scenario: Do not store user id on failed authentication
- **Given** token 无效或缺少非空 `user_id`
- **When** 认证中间件拒绝请求
- **Then** Gin context MUST NOT 被标记为已认证用户
- **Then** 系统 MUST NOT 执行业务 handler
