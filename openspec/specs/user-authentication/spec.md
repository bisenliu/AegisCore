# user-authentication

## Purpose

用户认证能力为 HTTP API 提供可复用的 JWT Bearer 认证中间件，统一处理认证校验、失败响应和认证用户身份在请求上下文中的传播。
## Requirements
### Requirement: Authenticate HTTP requests with JWT Bearer tokens
系统 MUST 提供可复用的 Gin 认证中间件，用于校验 `Authorization: Bearer <token>` 请求头。认证中间件 MUST 只保护实际挂载该中间件的路由，不得读取配置化白名单路径，也不得基于请求路径自行判断认证豁免；公开访问必须由服务侧通过不挂载认证中间件的路由分组表达。挂载认证中间件的请求缺少认证头、认证头格式错误、token 为空或 token 无效时，系统 MUST 返回 HTTP 401，并使用 `common/contract/response.Envelope` 失败格式与未认证数字业务码 `20000`。当 token 通过签名、过期时间、issuer 和 audience 校验后，中间件 MUST 解析 `user_id`、`token_version` 和 `session_id`，其中 `user_id` MUST 是用户外部 UUID 标识，并将 token 中的 `token_version` 与服务端当前版本比较；版本不一致时系统 MUST 将该 token 视为无效。

#### Scenario: Public route bypasses authentication by not mounting middleware
- **Given** 服务侧将公开路由注册到未挂载认证中间件的路由分组
- **When** 调用方请求该公开路由且未携带 `Authorization` header
- **Then** 认证中间件 MUST NOT 处理该请求
- **Then** 系统 MUST NOT 因缺少认证 header 返回未认证错误

#### Scenario: Reject missing authorization header
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方未携带 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应信封的 `success` MUST 为 `false`
- **Then** 响应信封的 `code` MUST 为 `20000`

#### Scenario: Reject invalid bearer format
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带不以 `Bearer ` 开头的 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 MUST 使用统一失败响应信封

#### Scenario: Reject empty bearer token
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带 `Authorization: Bearer `
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 调用后续业务 handler

#### Scenario: Reject invalid token
- **Given** 请求命中已挂载认证中间件的路由
- **When** 调用方携带无法通过 JWT 解析、签名校验或标准 claims 校验的 token
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 认证失败日志 MUST NOT 记录 token 原文

#### Scenario: Reject mismatched token version
- **Given** 请求命中已挂载认证中间件的路由
- **Given** Access Token 签名有效且未过期
- **Given** Access Token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方携带该 Access Token 请求受保护 API
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 将该请求标记为已认证

### Requirement: Log authentication decisions with caller-provided logger

系统 MUST 要求 Gin 认证中间件由调用方显式传入 Zap logger。认证中间件 MUST 使用该 logger 记录认证头缺失、认证头格式错误、空 bearer token、token 校验失败和 token version 校验失败等认证决策日志，并 MUST 继续通过请求 `context.Context` 保留 `trace-id` 日志字段。认证失败日志 MUST NOT 记录 token 原文。认证中间件 MUST NOT 输出白名单放行日志，因为公开访问由未挂载认证中间件的路由分组表达。

#### Scenario: Public route does not emit whitelist auth log
- **Given** 服务侧将公开路由注册到未挂载认证中间件的路由分组
- **When** 请求命中该公开路由
- **Then** 认证中间件 MUST NOT 输出白名单放行日志
- **Then** 该请求 MUST 继续执行对应公开 handler

#### Scenario: Log authentication failure with provided logger
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **Given** 请求命中已挂载认证中间件的路由
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
系统 MUST 使用 `github.com/golang-jwt/jwt/v5` 解析 JWT token，并 MUST 校验签名方法与过期时间。配置声明 issuer 或 audience 时，系统 MUST 对应校验 issuer 或 audience；未配置 issuer 或 audience 时，系统 MUST NOT 因这些字段为空而拒绝 token。用于访问受保护业务接口的 Access Token claims MUST 包含非空且格式合法的 UUID `user_id`、大于零的 `token_version` 和非空 `session_id`。

#### Scenario: Accept valid token
- **Given** auth 配置包含 JWT secret
- **Given** token 使用允许的签名方法签名且未过期
- **Given** token claims 包含非空 UUID `user_id`
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

#### Scenario: Reject internal numeric user id claim
- **Given** token 签名有效且未过期
- **Given** token claims 的 `user_id` 为内部数字 ID 或非 UUID 字符串
- **When** 认证中间件解析该 token
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 系统 MUST NOT 将该请求标记为已认证

### Requirement: Propagate authenticated user identity through request contexts
系统 MUST 将认证通过后的外部用户 ID 写入 Gin context 和 `c.Request.Context()`，并使用稳定的 context key 表示当前认证用户。系统 MUST 同时传播当前认证会话标识，使后续 handler 能读取当前 `session_id` 以执行退出当前设备等会话控制操作。

#### Scenario: Store user id after successful authentication
- **Given** token claims 包含 `user_id: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前用户 ID `018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e`
- **Then** Go `context.Context` MUST 包含当前用户 ID `018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e`
- **Then** 后续 handler MUST 能读取该用户 ID

#### Scenario: Store session id after successful authentication
- **Given** token claims 包含 `session_id: "s-123"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前会话 ID `s-123`
- **Then** Go `context.Context` MUST 包含当前会话 ID `s-123`
- **Then** 后续 handler MUST 能读取该会话 ID

#### Scenario: Do not store user id on failed authentication
- **Given** token 无效或缺少非空 UUID `user_id`
- **When** 认证中间件拒绝请求
- **Then** Gin context MUST NOT 被标记为已认证用户
- **Then** 系统 MUST NOT 执行业务 handler

### Requirement: Reuse shared authentication boundary constants
系统 SHALL 使用 `common/security/auth` 包中统一定义的认证边界常量表达 Authorization header、Bearer token type 和 Bearer authorization prefix，避免调用方重复硬编码这些协议值。

#### Scenario: Use shared authorization header constant
- **WHEN** 认证中间件读取请求认证信息
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 Authorization header 常量
- **THEN** header 名值 MUST 保持为 `Authorization`

#### Scenario: Use shared bearer prefix constant
- **WHEN** 认证中间件校验或剥离 Authorization header
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 Bearer prefix 常量
- **THEN** prefix 值 MUST 保持为 `Bearer `

#### Scenario: Use shared bearer token type constant
- **WHEN** 登录或刷新接口响应 token type
- **THEN** 系统 MUST 使用 `common/security/auth` 中的 Bearer token type 常量
- **THEN** token type 值 MUST 保持为 `Bearer`

### Requirement: Reuse shared authentication failure message
系统 SHALL 使用 `common` 模块中统一定义的认证失败公开文案返回缺失认证、token 非法、token 过期或 token version 不匹配等认证失败响应。

#### Scenario: Return shared authentication failure message
- **WHEN** 认证中间件拒绝缺失、格式错误、空值、非法、过期或版本不匹配的 token
- **THEN** 响应 message MUST 使用 `common` 中的统一认证失败公开文案
- **THEN** 公开文案值 MUST 保持为 `登录状态无效或已过期，请重新登录`

#### Scenario: Preserve authentication error classification
- **WHEN** 认证失败文案常量来源迁移到 `common`
- **THEN** 缺失认证信息 MUST 继续返回未认证业务码 `20000`
- **THEN** token 格式错误、非法或版本不匹配 MUST 继续返回 token invalid 业务码
- **THEN** token 过期 MUST 继续返回 token expired 业务码
- **THEN** 所有上述响应 MUST 继续使用 HTTP 401

### Requirement: Use shared credentials package for JWT token service
系统 SHALL 使用 `common/security/auth` 包作为 JWT token 凭证服务、claims、sign input、token subject 常量、认证上下文 helper 和认证传输常量的规范来源。认证中间件和用户认证会话业务 MUST NOT 在新代码中继续依赖 `common/credentials` 或分散的 `common/jwt` 规范来源。

#### Scenario: Authentication middleware uses auth JWT service
- **WHEN** 认证中间件解析 Bearer access token
- **THEN** 系统 MUST 使用 `common/security/auth` 提供的 JWT token 服务类型或接口
- **THEN** access token 的签名、过期时间、issuer、audience、subject、`user_id`、`token_version` 和 `session_id` 校验行为 MUST 保持不变

#### Scenario: Login and refresh responses keep bearer token type
- **WHEN** 登录或刷新接口返回 token 响应
- **THEN** token type MUST 来自 `common/security/auth` 的 Bearer token type 常量
- **THEN** 响应中的 token type 值 MUST 保持为 `Bearer`

#### Scenario: Authentication context uses auth helpers
- **WHEN** 认证中间件将认证成功后的 `user_id` 和 `session_id` 写入请求 context
- **THEN** 系统 MUST 使用 `common/security/auth` 提供的 context helper
- **THEN** 后续 handler 和 service MUST 能通过 `common/security/auth` 读取相同认证身份和会话标识
