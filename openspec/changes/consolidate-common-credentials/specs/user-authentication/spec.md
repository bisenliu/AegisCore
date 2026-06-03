## MODIFIED Requirements

### Requirement: Propagate authenticated user identity through request contexts

系统 MUST 将认证通过后的用户 ID 写入 Gin context 和 `c.Request.Context()`，并使用 `common/credentials` 中稳定的 context key 与 helper 表示当前认证用户。系统 MUST 同时传播当前认证会话标识，使后续 handler 能通过 `common/credentials` 读取当前 `session_id` 以执行退出当前设备等会话控制操作。

#### Scenario: Store user id after successful authentication
- **Given** token claims 包含 `user_id: "u-123"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前用户 ID `u-123`
- **Then** Go `context.Context` MUST 包含当前用户 ID `u-123`
- **Then** 后续 handler MUST 能通过 `common/credentials` 读取该用户 ID

#### Scenario: Store session id after successful authentication
- **Given** token claims 包含 `session_id: "s-123"`
- **When** 认证中间件成功认证请求
- **Then** Gin context MUST 包含当前会话 ID `s-123`
- **Then** Go `context.Context` MUST 包含当前会话 ID `s-123`
- **Then** 后续 handler MUST 能通过 `common/credentials` 读取该会话 ID

#### Scenario: Do not store user id on failed authentication
- **Given** token 无效或缺少非空 `user_id`
- **When** 认证中间件拒绝请求
- **Then** Gin context MUST NOT 被标记为已认证用户
- **Then** 系统 MUST NOT 执行业务 handler

### Requirement: Reuse shared authentication boundary constants
系统 SHALL 使用 `common/credentials` 包中统一定义的认证边界常量表达 Authorization header、Bearer token type 和 Bearer authorization prefix，避免调用方重复硬编码这些协议值。

#### Scenario: Use shared authorization header constant
- **WHEN** 认证中间件读取请求认证信息
- **THEN** 系统 MUST 使用 `common/credentials` 中的 Authorization header 常量
- **THEN** header 名值 MUST 保持为 `Authorization`

#### Scenario: Use shared bearer prefix constant
- **WHEN** 认证中间件校验或剥离 Authorization header
- **THEN** 系统 MUST 使用 `common/credentials` 中的 Bearer prefix 常量
- **THEN** prefix 值 MUST 保持为 `Bearer `

#### Scenario: Use shared bearer token type constant
- **WHEN** 登录或刷新接口响应 token type
- **THEN** 系统 MUST 使用 `common/credentials` 中的 Bearer token type 常量
- **THEN** token type 值 MUST 保持为 `Bearer`

## ADDED Requirements

### Requirement: Use shared credentials package for JWT token service
系统 SHALL 使用 `common/credentials` 包作为 JWT token 凭证服务、claims、sign input 和 token subject 常量的规范来源。认证中间件和用户认证会话业务 MUST NOT 在新代码中继续依赖分散的 `common/jwt` 规范来源。

#### Scenario: Authentication middleware uses credentials JWT service
- **WHEN** 认证中间件解析 Bearer access token
- **THEN** 系统 MUST 使用 `common/credentials` 提供的 JWT token 服务类型或接口
- **THEN** access token 的签名、过期时间、issuer、audience、subject、`user_id`、`token_version` 和 `session_id` 校验行为 MUST 保持不变

#### Scenario: Login and refresh responses keep bearer token type
- **WHEN** 登录或刷新接口返回 token 响应
- **THEN** token type MUST 来自 `common/credentials` 的 Bearer token type 常量
- **THEN** 响应中的 token type 值 MUST 保持为 `Bearer`
