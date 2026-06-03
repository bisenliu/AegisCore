## MODIFIED Requirements

### Requirement: Reuse shared authentication boundary constants
系统 SHALL 使用 `common/auth` 包中统一定义的认证边界常量表达 Authorization header、Bearer token type 和 Bearer authorization prefix，避免调用方重复硬编码这些协议值。

#### Scenario: Use shared authorization header constant
- **WHEN** 认证中间件读取请求认证信息
- **THEN** 系统 MUST 使用 `common/auth` 中的 Authorization header 常量
- **THEN** header 名值 MUST 保持为 `Authorization`

#### Scenario: Use shared bearer prefix constant
- **WHEN** 认证中间件校验或剥离 Authorization header
- **THEN** 系统 MUST 使用 `common/auth` 中的 Bearer prefix 常量
- **THEN** prefix 值 MUST 保持为 `Bearer `

#### Scenario: Use shared bearer token type constant
- **WHEN** 登录或刷新接口响应 token type
- **THEN** 系统 MUST 使用 `common/auth` 中的 Bearer token type 常量
- **THEN** token type 值 MUST 保持为 `Bearer`

### Requirement: Use shared credentials package for JWT token service
系统 SHALL 使用 `common/auth` 包作为 JWT token 凭证服务、claims、sign input、token subject 常量、认证上下文 helper 和认证传输常量的规范来源。认证中间件和用户认证会话业务 MUST NOT 在新代码中继续依赖 `common/credentials` 或分散的 `common/jwt` 规范来源。

#### Scenario: Authentication middleware uses auth JWT service
- **WHEN** 认证中间件解析 Bearer access token
- **THEN** 系统 MUST 使用 `common/auth` 提供的 JWT token 服务类型或接口
- **THEN** access token 的签名、过期时间、issuer、audience、subject、`user_id`、`token_version` 和 `session_id` 校验行为 MUST 保持不变

#### Scenario: Login and refresh responses keep bearer token type
- **WHEN** 登录或刷新接口返回 token 响应
- **THEN** token type MUST 来自 `common/auth` 的 Bearer token type 常量
- **THEN** 响应中的 token type 值 MUST 保持为 `Bearer`

#### Scenario: Authentication context uses auth helpers
- **WHEN** 认证中间件将认证成功后的 `user_id` 和 `session_id` 写入请求 context
- **THEN** 系统 MUST 使用 `common/auth` 提供的 context helper
- **THEN** 后续 handler 和 service MUST 能通过 `common/auth` 读取相同认证身份和会话标识
