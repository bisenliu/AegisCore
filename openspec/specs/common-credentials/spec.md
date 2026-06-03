# common-credentials Specification

## Purpose
TBD - created by archiving change consolidate-common-credentials. Update Purpose after archive.
## Requirements
### Requirement: Provide password credential primitives
系统 SHALL 在 `common/password` 包提供可复用的密码凭证原语，用于生成和校验 Argon2id 密码 hash。密码 hash 输出格式、默认参数、空密码错误和无效 hash 错误语义 MUST 与现有密码凭证行为保持兼容。实现 MUST 使用具名常量表达 encoded hash 最大长度边界，避免在解析逻辑中使用裸字面量。

#### Scenario: Hash password with Argon2id
- **Given** 调用方提供非空明文密码
- **When** 调用方使用 `password.Hash` 生成密码 hash
- **Then** 系统 MUST 返回 Argon2id 格式的密码 hash
- **Then** hash MUST 包含算法、版本、memory、iterations、parallelism、salt 和 key 信息

#### Scenario: Verify matching password
- **Given** 系统已经通过 `password.Hash` 生成密码 hash
- **When** 调用方使用相同明文密码调用 `password.Verify`
- **Then** 系统 MUST 返回匹配成功
- **Then** 校验过程 MUST 使用 constant-time comparison 比较派生 key

#### Scenario: Reject invalid password hash
- **Given** 调用方提供格式非法、版本不匹配、参数非法或 base64 内容非法的密码 hash
- **When** 调用方使用 `password.Verify` 校验密码
- **Then** 系统 MUST 返回密码 hash 无效错误
- **Then** 系统 MUST NOT 将该密码视为匹配成功

#### Scenario: Password hash length boundary is named
- **Given** 维护者查看 `common/password` 的 encoded hash 解析逻辑
- **When** 实现限制 encoded hash 最大长度
- **Then** 系统 MUST 使用具名常量表达最大长度边界
- **Then** 最大长度值 MUST 保持为 `512`

### Requirement: Provide JWT token credential primitives
系统 SHALL 在 `common/auth` 包提供 JWT token 凭证服务，用于签发和解析访问 token、刷新 token 和密码变更 token。JWT 服务 MUST 继续使用现有认证配置中的 secret、issuer 和 audience，并 MUST 保持现有 claims、subject、过期时间、签名算法和用户身份字段校验语义。

#### Scenario: Sign access token
- **Given** 认证配置包含 JWT secret
- **Given** 调用方提供合法 `user_id`、大于零的 `token_version`、非空 `session_id` 和 TTL
- **When** 调用方使用 `auth.NewJWTService` 创建服务并签发 access token
- **Then** 系统 MUST 返回使用 HMAC SHA-256 签名的 JWT token
- **Then** token claims MUST 包含 `user_id`、`token_version`、`session_id`、subject、expires_at、issuer 和 audience

#### Scenario: Parse valid access token
- **Given** JWT access token 签名有效且未过期
- **Given** token claims 包含合法 `user_id`、大于零的 `token_version` 和非空 `session_id`
- **When** 调用方使用 auth JWT 服务解析该 token
- **Then** 系统 MUST 返回 token claims
- **Then** 系统 MUST 将该 token 识别为 access token

#### Scenario: Reject invalid token subject
- **Given** JWT token 签名有效且未过期
- **Given** token subject 与调用方解析方法要求的 subject 不一致
- **When** 调用方使用 auth JWT 服务解析该 token
- **Then** 系统 MUST 返回无效 subject 错误
- **Then** 系统 MUST NOT 将该 token 视为目标凭证类型

#### Scenario: Reject missing identity fields
- **Given** JWT token 签名有效且未过期
- **Given** token claims 缺少合法 `user_id`、大于零的 `token_version` 或非空 `session_id`
- **When** 调用方使用 auth JWT 服务解析需要这些字段的 token
- **Then** 系统 MUST 返回对应凭证校验错误
- **Then** 系统 MUST NOT 返回有效 claims

### Requirement: Provide authentication transport and context credentials
系统 SHALL 在 `common/auth` 包提供认证传输常量和认证上下文 helper，用于表达 Authorization header、Bearer token 类型、Bearer 前缀、认证用户 ID 和认证会话 ID。常量值 MUST 与现有 HTTP 认证契约保持一致。系统 MUST 提供 `StripBearerPrefix(token string) string` 统一剥离可选 Bearer 前缀：该 helper MUST 先 trim token 首尾空白；当前缀按大小写无关匹配等于 `Bearer ` 时，MUST 返回剥离前缀后的 token；当前缀不存在时，MUST 返回 trim 后的原 token。

#### Scenario: Provide bearer authorization constants
- **When** 调用方读取 auth 认证传输常量
- **Then** `auth.AuthorizationHeader` MUST 等于 `Authorization`
- **Then** `auth.TokenTypeBearer` MUST 等于 `Bearer`
- **Then** `auth.TokenPrefix` MUST 等于 `Bearer `

#### Scenario: Strip bearer prefix from token
- **Given** 调用方提供 token 字符串 `Bearer abc.def.ghi`
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Strip bearer prefix case-insensitively
- **Given** 调用方提供 token 字符串 `bearer abc.def.ghi`
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Strip surrounding whitespace before bearer parsing
- **Given** 调用方提供 token 字符串 `  Bearer abc.def.ghi  `
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Keep raw token without bearer prefix
- **Given** 调用方提供 token 字符串 `abc.def.ghi`
- **When** 调用方使用 `auth.StripBearerPrefix`
- **Then** 系统 MUST 返回 `abc.def.ghi`

#### Scenario: Store and read authenticated user id
- **Given** 调用方持有 `context.Context` 和认证用户 ID
- **When** 调用方使用 `auth.WithUserID` 写入用户 ID
- **Then** 调用方 MUST 能使用 `auth.UserIDFromContext` 读取相同用户 ID
- **Then** 空用户 ID 或缺失用户 ID MUST NOT 被读取为有效认证用户

#### Scenario: Store and read authenticated session id
- **Given** 调用方持有 `context.Context` 和认证会话 ID
- **When** 调用方使用 `auth.WithSessionID` 写入会话 ID
- **Then** 调用方 MUST 能使用 `auth.SessionIDFromContext` 读取相同会话 ID
- **Then** 空会话 ID 或缺失会话 ID MUST NOT 被读取为有效认证会话

### Requirement: Keep credentials package focused on credential primitives
系统 SHALL 将 common 模块中的共享凭证原语限定为身份凭证产生、校验、传输和认证上下文绑定相关能力。密码 hash 与校验 MUST 位于 `common/password` 包；JWT token、认证传输常量和认证上下文 helper MUST 位于 `common/auth` 包；系统 MUST NOT 继续提供 `common/credentials` 聚合包。系统 MUST NOT 将 trace-id、日志、配置加载、数据库连接、Redis client、HTTP 响应 envelope 或业务 controller/service/repository 逻辑放入这些共享凭证包。

#### Scenario: Keep trace id outside credential primitives
- **Given** 维护者需要修改 HTTP trace-id header、Gin trace key、Go context trace value 或 Zap 日志 trace 字段
- **When** 维护者查看 `common/password` 和 `common/auth` 包
- **Then** 这些包 MUST NOT 包含 trace-id 相关实现
- **Then** trace-id 行为 MUST 继续由现有中间件和 logger context 边界维护

#### Scenario: Do not create runtime dependencies
- **When** 服务导入或使用 `common/password` 或 `common/auth` 包
- **Then** 这些包 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx lifecycle
- **Then** 这些包 MUST NOT 读取配置文件或连接外部系统

#### Scenario: Do not expose credentials aggregate package
- **When** 维护者查看 common 模块共享凭证原语
- **Then** 系统 MUST NOT 保留 `common/credentials` 目录或 `github.com/aegiscore/common/credentials` 包路径
- **Then** 新代码 MUST 根据用途导入 `github.com/aegiscore/common/password` 或 `github.com/aegiscore/common/auth`
