# common-credentials Specification

## Purpose
TBD - created by archiving change consolidate-common-credentials. Update Purpose after archive.
## Requirements
### Requirement: Provide password credential primitives
系统 SHALL 在 `common/security/password` 包提供可复用的密码凭证原语，用于生成和校验 Argon2id 密码 hash。密码 hash 输出格式、默认参数、空密码错误和无效 hash 错误语义 MUST 与现有密码凭证行为保持兼容。实现 MUST 使用具名常量表达 encoded hash 最大长度边界，避免在解析逻辑中使用裸字面量。系统 MUST 提供 `HashContext` 和 `VerifyContext` 支持等待 Argon2id KDF 槽位时被 `context.Context` 取消，并 MUST NOT 继续提供旧的 `Hash` 和 `Verify` 同步入口。系统 MUST 限制单进程内 Argon2id KDF 并发数量和执行中/等待中请求总数；当请求总数达到上限时 MUST 返回 KDF busy 错误。系统 MUST 限制明文密码最大长度，并 MUST 拒绝不符合当前策略的 Argon2id 参数、salt 长度或 key 长度。

#### Scenario: Hash password with Argon2id
- **Given** 调用方提供非空且未超过最大长度的明文密码
- **When** 调用方使用 `password.HashContext` 生成密码 hash
- **Then** 系统 MUST 返回 Argon2id 格式的密码 hash
- **Then** hash MUST 包含算法、版本、memory、iterations、parallelism、salt 和 key 信息

#### Scenario: Verify matching password
- **Given** 系统已经通过 `password.HashContext` 生成密码 hash
- **When** 调用方使用相同明文密码调用 `password.VerifyContext`
- **Then** 系统 MUST 返回匹配成功
- **Then** 校验过程 MUST 使用 constant-time comparison 比较派生 key

#### Scenario: Reject invalid password hash
- **Given** 调用方提供格式非法、版本不匹配、参数非法、参数不符合当前策略、salt 长度不符合当前策略、key 长度不符合当前策略或 base64 内容非法的密码 hash
- **When** 调用方使用 `password.VerifyContext` 校验密码
- **Then** 系统 MUST 返回密码 hash 无效错误
- **Then** 系统 MUST NOT 将该密码视为匹配成功

#### Scenario: Password hash length boundary is named
- **Given** 维护者查看 `common/security/password` 的 encoded hash 解析逻辑
- **When** 实现限制 encoded hash 最大长度
- **Then** 系统 MUST 使用具名常量表达最大长度边界
- **Then** 最大长度值 MUST 保持为 `512`

#### Scenario: Reject empty or oversized plain password
- **Given** 调用方提供空明文密码或超过最大长度的明文密码
- **When** 调用方使用 `password.HashContext` 或 `password.VerifyContext`
- **Then** 空明文密码 MUST 返回空密码错误
- **Then** 超过最大长度的明文密码 MUST 返回密码过长错误
- **Then** 系统 MUST NOT 执行 Argon2id KDF

#### Scenario: Remove legacy synchronous password APIs
- **Given** 维护者查看 `common/security/password` 包的公开 API
- **When** 系统完成密码 KDF 强化变更
- **Then** 包 MUST NOT 暴露 `Hash` 同步入口
- **Then** 包 MUST NOT 暴露 `Verify` 同步入口
- **Then** 仓库内调用方 MUST 使用 `HashContext` 或 `VerifyContext`

#### Scenario: Cancel while waiting for password KDF capacity
- **Given** Argon2id KDF 执行槽位不可立即获得
- **Given** 调用方传入的 `context.Context` 在等待期间被取消或超时
- **When** 调用方使用 `password.HashContext` 或 `password.VerifyContext`
- **Then** 系统 MUST 返回 context 取消或超时错误
- **Then** 系统 MUST 释放已占用的等待队列资源

#### Scenario: Reject password KDF when queue is full
- **Given** 单进程内 Argon2id KDF 执行中和等待中的请求总数已达到包内上限
- **When** 新调用方使用 `password.HashContext` 或 `password.VerifyContext` 请求执行 KDF
- **Then** 系统 MUST 返回 KDF busy 错误
- **Then** 系统 MUST NOT 让该请求无限等待或进入 Argon2id KDF

### Requirement: Provide JWT token credential primitives
系统 SHALL 在 `common/security/auth` 包提供 JWT token 凭证服务，用于签发和解析访问 token、刷新 token 和密码变更 token。JWT 服务 MUST 继续使用现有认证配置中的 secret、issuer 和 audience，并 MUST 保持现有 claims、subject、过期时间、签名算法和用户身份字段校验语义。JWT 服务 MUST 区分缺失的 `user_id` 与格式非法的 `user_id`：当 `user_id` 为空时 MUST 返回缺失用户 ID 错误，当 `user_id` 存在但不是合法 UUID 时 MUST 返回无效用户 ID 错误。

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
- **Given** token claims 缺少 `user_id`、大于零的 `token_version` 或非空 `session_id`
- **When** 调用方使用 auth JWT 服务解析需要这些字段的 token
- **Then** 系统 MUST 返回对应凭证校验错误
- **Then** 系统 MUST NOT 返回有效 claims

#### Scenario: Reject invalid user id format
- **Given** JWT token 签名有效且未过期
- **Given** token claims 包含非空但不是合法 UUID 的 `user_id`
- **When** 调用方使用 auth JWT 服务解析该 token
- **Then** 系统 MUST 返回无效用户 ID 错误
- **Then** 系统 MUST NOT 返回缺失用户 ID 错误
- **Then** 系统 MUST NOT 返回有效 claims

### Requirement: Parse Bearer authorization headers consistently
系统 SHALL 在 `common/security/auth` 包提供统一 Bearer authorization header 解析函数，用于要求调用方必须提供 `Authorization: Bearer <token>` 语义的入口。该解析函数 MUST 先 trim header 首尾空白；Bearer 前缀 MUST 按大小写无关方式匹配；提取后的 token MUST trim 首尾空白；缺少 Bearer 前缀和 Bearer token 为空 MUST 可被调用方区分；token 原文除首尾空白外 MUST NOT 被修改。

#### Scenario: Parse bearer authorization header
- **Given** 调用方提供 Authorization header `Bearer abc.def.ghi`
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 返回 token `abc.def.ghi`
- **Then** 系统 MUST 将该 header 识别为有效 Bearer authorization 格式

#### Scenario: Parse bearer prefix case-insensitively
- **Given** 调用方提供 Authorization header `bearer abc.def.ghi`
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 返回 token `abc.def.ghi`
- **Then** 系统 MUST 将该 header 识别为有效 Bearer authorization 格式

#### Scenario: Reject authorization header without bearer prefix
- **Given** 调用方提供 Authorization header `abc.def.ghi`
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 将该 header 识别为格式错误
- **Then** 系统 MUST NOT 返回有效 token

#### Scenario: Reject empty bearer authorization token
- **Given** 调用方提供 Authorization header `Bearer `
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 将该 header 识别为空 Bearer token
- **Then** 系统 MUST NOT 返回有效 token

#### Scenario: Preserve token contents after trimming
- **Given** 调用方提供 Authorization header `  Bearer AbC.Def.GhI  `
- **When** 调用方使用 shared auth Bearer authorization 解析函数
- **Then** 系统 MUST 返回 token `AbC.Def.GhI`
- **Then** 系统 MUST NOT 修改 token 内部字符大小写

### Requirement: Provide authentication transport and context credentials
系统 SHALL 在 `common/security/auth` 包提供认证传输常量、Bearer authorization 解析 helper 和认证上下文 helper，用于表达 Authorization header、Bearer token 类型、Bearer 前缀、认证用户 ID 和认证会话 ID。常量值 MUST 与现有 HTTP 认证契约保持一致。系统 MUST 提供 `StripBearerPrefix(token string) string` 统一剥离可选 Bearer 前缀：该 helper MUST 先 trim token 首尾空白；当前缀按大小写无关匹配等于 `Bearer ` 时，MUST 返回剥离前缀后的 token；当前缀不存在时，MUST 返回 trim 后的原 token。系统 MUST 同时提供要求 Bearer 前缀存在的 Authorization header 解析入口，用于 HTTP 认证边界统一提取 token 并区分格式错误与空 token。

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
系统 SHALL 将 common 模块中的共享凭证原语限定为身份凭证产生、校验、传输和认证上下文绑定相关能力。密码 hash 与校验 MUST 位于 `common/security/password` 包；JWT token、认证传输常量和认证上下文 helper MUST 位于 `common/security/auth` 包；系统 MUST NOT 继续提供 `common/credentials` 聚合包。系统 MUST NOT 将 trace-id、日志、配置加载、数据库连接、Redis client、HTTP 响应 envelope 或业务 controller/service/repository 逻辑放入这些共享凭证包。

#### Scenario: Keep trace id outside credential primitives
- **Given** 维护者需要修改 HTTP trace-id header、Gin trace key、Go context trace value 或 Zap 日志 trace 字段
- **When** 维护者查看 `common/security/password` 和 `common/security/auth` 包
- **Then** 这些包 MUST NOT 包含 trace-id 相关实现
- **Then** trace-id 行为 MUST 继续由现有中间件和 logger context 边界维护

#### Scenario: Do not create runtime dependencies
- **When** 服务导入或使用 `common/security/password` 或 `common/security/auth` 包
- **Then** 这些包 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx lifecycle
- **Then** 这些包 MUST NOT 读取配置文件或连接外部系统

#### Scenario: Do not expose credentials aggregate package
- **When** 维护者查看 common 模块共享凭证原语
- **Then** 系统 MUST NOT 保留 `common/credentials` 目录或 `github.com/aegiscore/common/credentials` 包路径
- **Then** 新代码 MUST 根据用途导入 `github.com/aegiscore/common/security/password` 或 `github.com/aegiscore/common/security/auth`

### Requirement: Keep credential constants with credential primitives
凭证相关常量 SHALL 位于拥有对应凭证原语的包内。Authorization header、Bearer token 类型、Bearer 前缀、认证上下文 key、JWT subject、JWT claim 字段和密码 hash 参数 MUST 由 `common/security/auth` 或 `common/security/password` 维护；用户服务认证业务 TTL fallback 和 session repository key 格式 MUST 保持在用户服务认证能力边界内。

#### Scenario: Authentication transport constants are reused
- **WHEN** middleware、controller、DTO 或 Swagger-adjacent 代码需要表达 Authorization header 或 Bearer token 类型
- **THEN** Go 代码中的运行时逻辑 MUST 复用 `common/security/auth` 的认证传输常量
- **THEN** Swagger annotation 或 example 中无法引用 Go 常量的字面量 MUST 与公共认证传输常量保持一致

#### Scenario: JWT claim and subject constants stay in common auth
- **WHEN** 实现整理 JWT access、refresh 或 password-change token 的 subject 与 claim 字段
- **THEN** subject 和 claim 常量 MUST 由 `common/security/auth` 维护
- **THEN** 实现 MUST NOT 将 JWT 协议常量移动到用户服务 controller、repository 或全局 constants 包

#### Scenario: Token TTL fallbacks are owned by auth service
- **WHEN** 用户服务认证流程在配置缺失或 TTL 为零值时选择 access token、refresh token 或 password-change token fallback TTL
- **THEN** fallback 常量 MUST 位于认证 service 边界附近
- **THEN** 示例 YAML 和 DTO example 如与 fallback 不一致，MUST 明确其是部署示例、响应示例或安全 fallback

#### Scenario: Session key formats stay with Redis session repository
- **WHEN** 实现整理 Redis 中 token version、session 或用户 session index key 格式
- **THEN** key format 常量 MUST 位于 Redis auth session repository 边界附近
- **THEN** 实现 MUST NOT 将 Redis key format 暴露为无业务 owner 的全局常量

### Requirement: Host credential primitives under common security path
共享凭证原语 SHALL 位于 `common/security` 分类目录下。JWT token 凭证服务、Bearer 认证传输常量和认证上下文 helper MUST 位于 `common/security/auth`；Argon2id 密码 hash 与校验 MUST 位于 `common/security/password`。目录迁移 MUST 保持 JWT claims、token subject、Bearer 解析、认证上下文、密码 hash 格式和密码校验语义不变。

#### Scenario: Auth primitives move without token behavior changes
- **WHEN** JWT 服务和认证上下文 helper 迁移到 `common/security/auth`
- **THEN** access token、refresh token 和密码变更 token 的签发与解析语义 MUST 保持不变
- **THEN** Authorization header、Bearer token 类型、Bearer 前缀和认证上下文读取写入行为 MUST 保持不变

#### Scenario: Password primitives move without hash behavior changes
- **WHEN** 密码 helper 迁移到 `common/security/password`
- **THEN** Argon2id hash 输出格式、默认参数、空密码错误和无效 hash 错误语义 MUST 保持不变
- **THEN** 密码校验 MUST 继续使用 encoded hash 中的参数重新计算并执行 constant-time comparison

#### Scenario: Security primitives remain side-effect free
- **WHEN** 服务或测试导入 `common/security/auth` 或 `common/security/password`
- **THEN** 这些包 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx lifecycle
- **THEN** 用户服务登录、刷新、登出和 session repository 逻辑 MUST 继续由 `user-services` 自己拥有
