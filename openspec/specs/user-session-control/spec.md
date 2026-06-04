# user-session-control

## Purpose

用户会话控制能力通过 token version、Access Token、Refresh Token 和 Redis 会话记录支持登录、刷新、退出当前设备、退出全部设备和用户级 token 失效。
## Requirements
### Requirement: Authenticate users and create revocable sessions
系统 SHALL 提供用户登录能力。登录成功时，系统 MUST 从 PostgreSQL 读取未软删除用户资料、密码哈希、状态和当前 `token_version`。`status=100` 时系统 MUST 创建新的会话标识，签发携带当前 `token_version` 和 `session_id` 的普通 Access Token，签发 Refresh Token，并在 Redis 保存 Refresh Token 会话记录和用户活跃会话索引。`status=300` 时系统 MUST 在密码校验通过后签发仅可用于修改密码接口的受限改密凭据，不得签发普通会话 token 或创建普通 Redis 会话。

#### Scenario: Login creates access and refresh tokens
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户 `status` 为 `100`
- **Given** 用户提交的密码与 PostgreSQL 中的 `password_hash` 校验通过
- **When** 调用方提交登录请求
- **Then** 系统 MUST 从 PostgreSQL 当前用户记录读取 `token_version`
- **Then** 系统 MUST 创建新的 `session_id`
- **Then** 系统 MUST 返回 Access Token 和 Refresh Token
- **Then** Access Token claims MUST 包含 `user_id`、`token_version` 和 `session_id`
- **Then** Redis MUST 保存该 Refresh Token 对应的会话记录和用户活跃会话索引

#### Scenario: Login rejects invalid credentials
- **Given** 用户不存在、已软删除或密码校验失败
- **When** 调用方提交登录请求
- **Then** 系统 MUST 返回统一失败响应信封
- **Then** 系统 MUST NOT 签发 Access Token 或 Refresh Token
- **Then** 系统 MUST NOT 创建 Redis 会话记录

#### Scenario: Login rejects disabled user status
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户 `status` 为 `200`
- **When** 调用方提交登录请求
- **Then** 系统 MUST 拒绝登录
- **Then** 系统 MUST NOT 签发 Access Token 或 Refresh Token
- **Then** 系统 MUST NOT 创建 Redis 会话记录

#### Scenario: Login issues password-change credential for must-change-password user
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户 `status` 为 `300`
- **Given** 用户提交的密码与 PostgreSQL 中的 `password_hash` 校验通过
- **When** 调用方提交登录请求
- **Then** 系统 MUST NOT 签发普通 Access Token 或 Refresh Token
- **Then** 系统 MUST NOT 创建 Redis 会话记录
- **Then** 系统 MUST 返回受限改密凭据
- **Then** 该凭据 MUST 只能用于修改密码接口，不得用于普通受保护 API

#### Scenario: Password-change credential can access password change only
- **Given** 调用方持有 `status=300` 登录后返回的受限改密凭据
- **When** 调用方请求修改密码接口
- **Then** 系统 MUST 允许该请求进入修改密码处理流程
- **Then** 修改密码成功后系统 MUST 将用户 `status` 更新为 `100`
- **Then** 修改密码成功后系统 MUST 使该受限改密凭据失效或不再可用于后续改密

#### Scenario: Password-change credential is rejected by normal APIs
- **Given** 调用方持有 `status=300` 登录后返回的受限改密凭据
- **When** 调用方请求非修改密码的普通受保护 API
- **Then** 系统 MUST 返回 HTTP 401 或等价认证失败响应
- **Then** 普通业务 handler MUST NOT 执行

### Requirement: Separate credential authentication from login token issuance

系统 SHALL 在登录流程中保持用户凭据认证与认证成功后的 token 签发策略分离。凭据认证 MUST 校验归一化后的 `username` 和明文密码均非空，MUST 按 `username` 读取未软删除用户认证资料，MUST 使用共享密码校验 helper 验证 `password_hash`，并 MUST 将用户不存在、密码不匹配、密码 hash 校验错误和禁用用户状态映射为统一凭据无效响应。`status=300` 用户在密码校验通过后 MUST 被视为认证成功，并由登录签发策略返回受限改密凭据。

#### Scenario: Credential authentication rejects empty login input
- **Given** 调用方提交空白 `username` 或空白 `password`
- **When** 系统执行登录凭据认证
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 系统 MUST NOT 查询用户资料或签发任何 token

#### Scenario: Credential authentication hides invalid credential details
- **Given** 用户不存在、已软删除、密码校验失败或密码 hash 格式非法
- **When** 调用方提交登录请求
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 响应和业务日志 MUST NOT 包含明文密码、完整 hash、salt 或 hash 参数

#### Scenario: Must-change-password remains an issuance policy
- **Given** 用户存在且 `status` 为 `300`
- **Given** 用户提交的密码与 PostgreSQL 中的 `password_hash` 校验通过
- **When** 调用方提交登录请求
- **Then** 系统 MUST 将该用户视为凭据认证成功
- **Then** 登录签发策略 MUST 返回受限改密凭据
- **Then** 系统 MUST NOT 创建普通 Redis Refresh Token 会话

#### Scenario: Disabled user remains unauthenticated
- **Given** 用户存在且 `status` 为禁用状态
- **Given** 用户提交的密码与 PostgreSQL 中的 `password_hash` 校验通过
- **When** 调用方提交登录请求
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 系统 MUST NOT 签发普通 token 或受限改密凭据

### Requirement: Verify password-change credentials before loading user state

系统 SHALL 在修改密码流程中先验证受限改密凭据并解析外部用户 UUID，再读取用户状态和更新凭证。受限改密凭据验证 MUST 复用 `common/auth.StripBearerPrefix` 支持剥离可选 `Bearer ` 前缀，MUST 解析 password-change token，MUST 校验服务端当前 `token_version` 与 token claims 一致，并 MUST 将 claims 中的 `user_id` 解析为 UUID。用户存在性检查、用户仍处于 `status=300` 的校验、新密码 hash、凭证更新和 Redis token version 缓存失效 MUST 继续由修改密码业务流程负责。

#### Scenario: Password-change token validation rejects invalid token
- **Given** 调用方提交空白、格式非法、签名无效或 subject 非改密凭据的 token
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 更新用户凭证

#### Scenario: Password-change token validation accepts optional bearer prefix
- **Given** 调用方提交 `Bearer <password-change-token>`
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 通过 `common/auth.StripBearerPrefix` 剥离可选 Bearer 前缀
- **Then** 系统 MUST 按剥离后的 password-change token 执行后续校验

#### Scenario: Password-change token validation rejects changed token version
- **Given** 受限改密凭据签名有效且未过期
- **Given** token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方请求修改密码
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 查询后续状态或更新用户凭证

#### Scenario: Password-change flow owns user state validation
- **Given** 受限改密凭据通过 token 校验并解析出 UUID `user_id`
- **When** 系统继续处理修改密码请求
- **Then** 修改密码流程 MUST 使用该 UUID 查询用户
- **Then** 修改密码流程 MUST 校验用户仍处于 `status=300`
- **Then** 只有状态校验通过后系统 MUST 更新 `password_hash`、将状态更新为 `100` 并失效 token version 缓存

### Requirement: Refresh access tokens through revocable refresh sessions
系统 SHALL 提供 Refresh Token 刷新能力。刷新时，系统 MUST 校验 Refresh Token 签名、过期时间和 Refresh Token subject，解析外部用户标识和会话标识，确认 Redis 会话仍存在，并校验该用户当前 `token_version` 与 Refresh Token 或会话记录中的版本一致；校验通过后签发新的 Access Token。系统 SHOULD 对 Refresh Token 执行轮转，使旧 Refresh Token 对应会话失效并创建新会话。Access Token 与 Refresh Token 的 subject/token type 枚举 MUST 由 `common/auth` 统一提供，签发方法 MUST 内部强制设置对应 subject。刷新流程 MUST 通过 `common/auth.StripBearerPrefix` 统一处理请求体中可选 Bearer 前缀。

#### Scenario: Refresh succeeds for active session
- **Given** Refresh Token 签名有效且未过期
- **Given** Refresh Token claims 包含 UUID `user_id`
- **Given** Redis 中存在对应会话记录
- **Given** 用户当前 `token_version` 与会话版本一致
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 签发新的 Access Token
- **Then** 新 Access Token MUST 包含当前 UUID `user_id`、`token_version` 和有效 `session_id`

#### Scenario: Refresh accepts optional bearer prefix in request body
- **Given** Refresh Token 签名有效且未过期
- **Given** Redis 中存在对应会话记录
- **When** 调用方在刷新请求体 `refresh_token` 字段中提交 `Bearer <refresh-token>`
- **Then** 系统 MUST 通过 `common/auth.StripBearerPrefix` 剥离 `Bearer ` 前缀
- **Then** 系统 MUST 按剥离后的 Refresh Token 执行刷新校验

#### Scenario: Refresh request body prefers raw token
- **Given** 调用方查看刷新接口 DTO 或 Swagger 文档
- **When** 文档展示 `refresh_token` 字段示例
- **Then** 示例值 SHOULD 使用裸 Refresh Token
- **Then** 文档 MUST NOT 要求请求体字段必须带 `Bearer ` 前缀

#### Scenario: Refresh rejects revoked session
- **Given** Refresh Token 签名有效且未过期
- **Given** Redis 中不存在对应会话记录
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 返回未认证或 token 无效响应
- **Then** 系统 MUST NOT 签发新的 Access Token

#### Scenario: Refresh rejects access token subject
- **Given** Access Token 签名有效且未过期
- **When** 调用方将该 Access Token 作为刷新请求体 `refresh_token` 字段提交
- **Then** 系统 MUST 因 subject 不是 Refresh Token subject 而拒绝刷新
- **Then** 系统 MUST NOT 签发新的 Access Token

#### Scenario: Refresh token signer sets refresh subject
- **Given** 调用方使用公共 JWT service 签发 Refresh Token
- **When** `SignRefreshToken` 返回 token
- **Then** token claims subject MUST 等于公共 Refresh Token subject 枚举
- **Then** 调用方 MUST NOT 通过字符串入参覆盖该 subject

#### Scenario: Refresh rejects changed token version
- **Given** Refresh Token 签名有效且未过期
- **Given** Redis 中存在对应会话记录
- **Given** PostgreSQL 或版本缓存中的当前 `token_version` 与会话版本不一致
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 拒绝刷新
- **Then** 系统 MUST NOT 签发新的 Access Token

#### Scenario: Refresh token rotation revokes old session
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 删除旧 Refresh Token 会话记录
- **Then** 系统 MUST 创建新的会话标识或刷新会话记录
- **Then** 系统 MUST 返回新的 Refresh Token

### Requirement: Logout current device without changing user token version
系统 SHALL 支持退出当前设备。退出当前设备时，系统 MUST 删除当前 `session_id` 对应的 Redis Refresh Token 会话记录，并从用户活跃会话索引移除该会话；系统 MUST NOT 修改 PostgreSQL 中的 `token_version`。

#### Scenario: Logout current session
- **Given** 请求已通过 Access Token 认证
- **Given** 认证上下文包含当前 `user_id` 和 `session_id`
- **When** 调用方请求退出当前设备
- **Then** 系统 MUST 删除当前 `session_id` 对应的 Redis 会话记录
- **Then** 系统 MUST 从该用户会话索引移除当前 `session_id`
- **Then** 系统 MUST NOT 修改该用户的 PostgreSQL `token_version`

### Requirement: Logout all devices through token version increment
系统 SHALL 支持退出全部设备。退出全部设备时，系统 MUST 在 PostgreSQL 中原子递增该用户 `token_version`，更新成功后删除 Redis 中该用户的版本缓存、全部活跃会话记录和会话索引。

#### Scenario: Logout all sessions invalidates tokens
- **Given** 请求已通过 Access Token 认证
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先在 PostgreSQL 中递增该用户 `token_version`
- **Then** 系统 MUST 删除 Redis 中该用户的 token version 缓存
- **Then** 系统 MUST 删除该用户所有 Redis Refresh Token 会话记录
- **Then** 系统 MUST 清空该用户活跃会话索引
- **Then** 旧 Access Token MUST 因版本不一致而失效

### Requirement: Store authentication session data in Redis as cache and session layer
系统 SHALL 使用 Redis 保存用户 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引。Redis 中的 `token_version` 只能作为缓存，缓存未命中或被删除时系统 MUST 使用外部 `user_id` 回源 PostgreSQL 获取真实值。用户活跃会话索引 MUST 使用 Redis ZSet，Key MUST 保持 `auth:user:<user_id>:sessions`，member MUST 使用 `session_id`，score MUST 使用该会话过期时间的 Unix 时间戳。系统 MUST 在写入、读取或删除该用户活跃会话索引时，按当前 Unix 时间戳执行过期 member 清理。

#### Scenario: Token version cache miss reads PostgreSQL
- **Given** Redis 中不存在某用户的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** 系统 MUST 使用外部 `user_id` 查询 PostgreSQL 获取真实 `token_version`
- **Then** 系统 MUST 将真实版本回填 Redis 缓存

#### Scenario: User security update deletes Redis cache after PostgreSQL update
- **Given** 用户级安全事件需要整体失效 token
- **When** 系统处理该安全事件
- **Then** 系统 MUST 先更新 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 再删除或刷新 Redis token version 缓存
- **Then** 系统 MUST NOT 只更新 Redis 而不更新 PostgreSQL

#### Scenario: Session creation indexes expiration timestamp
- **Given** 登录或 Refresh Token 轮转需要创建新的 Redis 会话记录
- **When** 系统保存会话记录和用户活跃会话索引
- **Then** 系统 MUST 将 `auth:session:<session_id>` 保存为带 TTL 的会话记录
- **Then** 系统 MUST 将 `session_id` 写入 `auth:user:<user_id>:sessions` ZSet
- **Then** 该 ZSet member 的 score MUST 等于该会话过期时间的 Unix 时间戳
- **Then** 系统 MUST 在写入索引时清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Logout current session removes indexed member and stale members
- **Given** 请求已通过 Access Token 认证
- **Given** 认证上下文包含当前 `user_id` 和 `session_id`
- **When** 调用方请求退出当前设备
- **Then** 系统 MUST 删除当前 `session_id` 对应的 Redis 会话记录
- **Then** 系统 MUST 从 `auth:user:<user_id>:sessions` ZSet 移除当前 `session_id`
- **Then** 系统 MUST 清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Logout all sessions reads only non-expired indexed members
- **Given** 请求已通过 Access Token 认证
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先清理 `auth:user:<user_id>:sessions` ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member
- **Then** 系统 MUST 从该 ZSet 读取仍未过期的 `session_id`
- **Then** 系统 MUST 删除读取到的 Redis 会话记录
- **Then** 系统 MUST 删除或清空该用户活跃会话索引

### Requirement: Centralize default authentication TTL values

认证会话服务 SHALL 将默认 Access Token TTL、Refresh Token TTL 和 token version cache TTL 集中声明为包级常量。实现 MUST 使用这些常量作为零值或非法 TTL 配置的兜底值，并保持现有默认生命周期不变。

#### Scenario: Default access token TTL uses named constant
- **GIVEN** 认证配置未提供有效 `auth.jwt.access_token_ttl`
- **WHEN** 登录或刷新流程签发 Access Token
- **THEN** 系统 MUST 使用集中声明的默认 Access Token TTL 常量
- **THEN** 默认 Access Token TTL MUST 保持为 15 分钟

#### Scenario: Default refresh token TTL uses named constant
- **GIVEN** 认证配置未提供有效 `auth.jwt.refresh_token_ttl`
- **WHEN** 登录或启用轮转的刷新流程创建 Refresh Token 会话
- **THEN** 系统 MUST 使用集中声明的默认 Refresh Token TTL 常量
- **THEN** 默认 Refresh Token TTL MUST 保持为 7 天

#### Scenario: Default token version cache TTL uses named constant
- **GIVEN** 认证配置未提供有效 `auth.token_version_cache_ttl`
- **WHEN** session store 回源 PostgreSQL 并写入 Redis token version 缓存
- **THEN** 系统 MUST 使用集中声明的默认 token version cache TTL 常量
- **THEN** 默认 token version cache TTL MUST 保持为 5 分钟

#### Scenario: Explicit TTL config still takes precedence
- **GIVEN** 认证配置提供有效 Access Token TTL、Refresh Token TTL 或 token version cache TTL
- **WHEN** 认证服务签发 token 或 session store 写入缓存
- **THEN** 系统 MUST 使用显式配置值
- **THEN** 系统 MUST NOT 用默认 TTL 常量覆盖有效配置值

### Requirement: Verify login passwords through shared Argon2id helper
系统 SHALL 在登录认证中通过 `common/password` 的统一密码校验方法验证密码。认证服务不得直接调用底层 Argon2 API，不得使用明文比较，不得在日志或错误响应中公开密码明文、完整 hash、salt 或 hash 参数。

#### Scenario: Login uses shared password verification
- **Given** 用户存在且数据库中保存 Argon2id 密码 hash
- **When** 调用方提交 `username` 和 `password` 登录
- **Then** auth service MUST 调用 `common/password` 密码校验方法
- **Then** 密码校验通过时系统 MUST 继续创建认证会话

#### Scenario: Login hides password verification details
- **Given** 密码校验失败或 hash 格式非法
- **When** 系统处理登录请求
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 响应和业务日志 MUST NOT 包含明文密码、完整 hash、salt 或 hash 参数

### Requirement: Read authentication credentials from password hash field
系统 MUST 使用 `password_hash` 作为用户认证凭据持久化字段，并不得在认证流程中读取或依赖旧 `password` 数据库字段。

#### Scenario: Password verification uses password hash
- **Given** 登录流程需要校验用户密码
- **When** repository 读取用户认证资料
- **Then** repository MUST 读取 `password_hash`
- **Then** service MUST 使用 `password_hash` 执行密码校验
- **Then** service 和 repository MUST NOT 引用旧持久化字段 `password`

#### Scenario: Soft deleted user token version is unavailable
- **Given** Redis token version 缓存未命中
- **Given** PostgreSQL 中对应用户的 `deleted_at` 不为 `NULL`
- **When** 系统回源读取用户 `token_version`
- **Then** repository MUST 按未删除条件查询
- **Then** 系统 MUST 将该用户视为不可认证

### Requirement: Update user credentials through a credential update contract

系统 SHALL 在修改密码成功后通过用户仓储的通用凭证更新契约持久化新凭证。凭证更新 MUST 面向未软删除用户，MUST 写入新的 `password_hash`，MUST 更新目标用户状态，MUST 递增 PostgreSQL 中的 `token_version`，并 MUST 返回更新后的 `token_version`。系统 MUST NOT 在该流程中读取或写入旧 `password` 字段。

#### Scenario: Password change updates credentials and invalidates tokens
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户当前状态为 `300`
- **Given** 调用方持有有效的受限改密凭据
- **When** 调用方提交有效的新密码并完成密码哈希
- **Then** 系统 MUST 通过凭证更新契约写入新的 `password_hash`
- **Then** 系统 MUST 将用户状态更新为 `100`
- **Then** 系统 MUST 递增 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 删除或刷新 Redis token version 缓存，使旧凭据不可继续用于改密

#### Scenario: Credential update ignores soft deleted users
- **Given** PostgreSQL 中对应用户的 `deleted_at` 不为 `NULL`
- **When** 系统尝试更新该用户凭证
- **Then** 系统 MUST 将该用户视为不存在
- **Then** 系统 MUST NOT 更新该用户的 `password_hash`
- **Then** 系统 MUST NOT 更新该用户状态或递增该用户 `token_version`

#### Scenario: Credential update returns current token version after persistence
- **Given** 用户凭证更新成功
- **When** repository 完成 PostgreSQL 更新
- **Then** repository MUST 返回更新后的 `token_version`
- **Then** 调用方 MUST NOT 只依赖 Redis token version 缓存判断该次安全更新是否完成

### Requirement: Authentication sessions use repository abstraction with Redis implementation boundary
用户会话控制能力 SHALL 通过根 `repository.AuthSessionRepository` 抽象管理 token version、Refresh Token 会话和用户活跃会话索引，具体 Redis 实现 MUST 位于 `user-services/internal/repository/redis` 包。service 层 MUST NOT 定义或持有 Redis session store 具体实现。

#### Scenario: Auth service depends on auth session repository abstraction
- **Given** 登录、刷新、退出当前设备、退出全部设备或修改密码流程需要访问会话状态
- **When** auth service 调用会话数据访问层
- **Then** auth service MUST 依赖 `repository.AuthSessionRepository`
- **Then** auth service MUST 使用 `repository.AuthSession` 表达会话数据
- **Then** auth service MUST NOT 依赖 Redis client 或 `repository/redis` 私有实现类型

#### Scenario: Session not found error remains mappable
- **Given** Redis 中不存在指定 Refresh Token 会话记录
- **When** auth service 读取会话
- **Then** Redis 实现 MUST 返回 `repository.ErrAuthSessionNotFound`
- **Then** auth service MUST 继续将该错误映射为未认证或 token 无效响应

#### Scenario: Token version mismatch remains mappable
- **Given** token claims 或会话记录中的 `token_version` 与服务端当前版本不一致
- **When** 系统校验 token version
- **Then** auth session repository MUST 返回 `repository.ErrTokenVersionMismatch`
- **Then** 系统 MUST 继续拒绝刷新、受保护请求或改密凭据校验

#### Scenario: Redis session storage behavior remains compatible
- **Given** `repository/redis` 承载认证会话 Redis 实现
- **When** 系统创建、读取、删除或批量删除认证会话
- **Then** Redis key 格式、Refresh Token 会话 TTL、用户活跃会话 ZSet 和过期 member 清理行为 MUST 与迁移前保持一致
- **Then** token version 缓存未命中时 MUST 继续回源 `repository.UserRepository`

### Requirement: Authentication token validator uses auth session repository abstraction
用户服务运行时 SHALL 将 `repository.AuthSessionRepository` 作为认证中间件 token version validator 的依赖来源。该抽象 MUST 提供 `ValidateTokenVersion(ctx, userID, tokenVersion)`，并保持现有 token version 校验语义。

#### Scenario: Protected route token validation remains compatible
- **Given** 用户服务注册受保护路由认证中间件
- **When** 中间件需要校验 Access Token 的 token version
- **Then** 中间件 MUST 使用 Fx 注入的 `repository.AuthSessionRepository`
- **Then** 有效 token MUST 继续允许进入受保护 handler
- **Then** 版本不一致 token MUST 继续在进入 handler 前被拒绝

### Requirement: User session repository returns domain user-not-found errors

用户会话控制能力 SHALL 保持 service/repository 分层边界。PostgreSQL 用户 repository 在会话控制流程中读取 `token_version`、递增 `token_version` 或更新用户凭证时，若目标未软删除用户不存在，MUST 返回 `domain.ErrUserNotFound`，MUST NOT 直接构造 `common/response` 应用错误。service 层 MUST 负责将该领域错误映射为现有认证失败、token 无效、not found 或内部错误响应语义。

#### Scenario: Token version lookup misses user
- **Given** Redis token version 缓存未命中
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 读取该用户 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 返回 `response.NotFoundError` 或其他 `common/response` 应用错误
- **Then** service 层 MUST 继续按现有认证或 token 校验流程映射该错误

#### Scenario: Logout all devices misses user during token version increment
- **Given** 请求已通过 Access Token 认证
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 尝试递增该用户 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 构造 HTTP not found 应用错误
- **Then** service 层 MUST 负责输出与迁移前兼容的失败响应

#### Scenario: Credential update misses user
- **Given** 修改密码流程已验证受限改密凭据
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 尝试更新该用户 `password_hash`、状态和 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 更新任何软删除用户或不存在用户
- **Then** service 层 MUST 负责将该领域错误映射为现有修改密码失败语义

#### Scenario: Unexpected database error remains internal
- **Given** PostgreSQL 在读取 token version、递增 token version 或更新凭据时发生非 not found 错误
- **When** 用户 repository 返回错误给 service 层
- **Then** repository MUST NOT 将该错误伪装为 `domain.ErrUserNotFound`
- **Then** service 层 MUST 继续将非预期错误映射为内部错误或既有安全失败语义

### Requirement: Validate and normalize auth request input before service business flow
系统 MUST 在 Auth Service 执行认证会话业务编排前完成认证请求的请求级清洗和基础校验。登录请求的 `username` 和 `password` 裁剪、空凭据校验，改密请求的 `new_password` 裁剪和空值校验，以及刷新请求体 `refresh_token` 的可选 Bearer 前缀剥离和空值校验 MUST 位于 Controller、共享请求校验器或服务内 Validation 层，而不是作为 Auth Service 的主要职责。

#### Scenario: Normalize login credentials before authentication
- **Given** 调用方提交登录请求且 `username` 或 `password` 前后包含空白
- **When** controller 处理登录请求并调用 Auth Service
- **Then** 空白裁剪 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Auth Service MUST 使用已规范化的 `username` 和明文密码执行凭据认证

#### Scenario: Reject empty login credentials before repository lookup
- **Given** 登录请求的 `username` 或 `password` 在裁剪后为空
- **When** controller 处理登录请求
- **Then** 请求 MUST 在查询用户资料、校验密码或签发 token 前被判定为认证失败或请求校验失败
- **Then** 系统 MUST NOT 创建 Redis 会话记录
- **Then** Auth Service MUST NOT 将空凭据基础校验作为登录流程的主要业务分支

#### Scenario: Normalize password change input before credential update
- **Given** 调用方提交改密请求且 `new_password` 前后包含空白
- **When** controller 处理改密请求并调用 Auth Service
- **Then** 新密码空白裁剪和裁剪后空值校验 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Auth Service MUST 使用已规范化的新密码执行密码哈希和凭证更新流程

#### Scenario: Normalize refresh token request before refresh flow
- **Given** 调用方在刷新请求体 `refresh_token` 字段提交裸 Refresh Token 或 `Bearer <refresh-token>`
- **When** controller 处理刷新请求并调用 Auth Service
- **Then** 请求体 token 的可选 Bearer 前缀剥离和空值校验 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Auth Service MUST 使用已规范化的 Refresh Token 执行 token claims、session 和 token version 校验

### Requirement: Keep authentication and session semantics in Auth Service
系统 MUST 保持认证会话安全语义由 Auth Service 或认证中间件边界负责。Validation 层 MUST NOT 执行依赖 JWT claims、Redis session、token version、用户状态或 Repository 查询的认证业务校验。

#### Scenario: Auth service verifies credentials after request validation
- **Given** 登录请求已经完成请求级规范化和基础校验
- **When** Auth Service 执行登录流程
- **Then** Auth Service MUST 查询用户认证资料
- **Then** Auth Service MUST 使用共享密码 helper 校验 `password_hash`
- **Then** Auth Service MUST 根据用户状态决定拒绝登录、签发普通 token 或签发受限改密凭据

#### Scenario: Auth service verifies password-change token semantics
- **Given** 改密请求已经完成请求级规范化和基础校验
- **When** Auth Service 执行改密流程
- **Then** Auth Service MUST 解析 password-change token claims
- **Then** Auth Service MUST 校验服务端当前 `token_version` 与 claims 一致
- **Then** Auth Service MUST 查询用户状态并只允许 `status=300` 用户完成改密

#### Scenario: Auth service verifies refresh session semantics
- **Given** 刷新请求已经完成请求级规范化和基础校验
- **When** Auth Service 执行刷新流程
- **Then** Auth Service MUST 校验 Refresh Token 签名、subject、claims user_id 和 session_id
- **Then** Auth Service MUST 校验 Redis session 存在且与 claims 匹配
- **Then** Auth Service MUST 校验当前 `token_version` 与会话版本一致后才签发新 token

#### Scenario: Auth context validation remains security boundary
- **Given** 调用方请求退出当前设备或退出全部设备
- **When** Auth Service 从认证上下文读取 `user_id` 和 `session_id`
- **Then** Auth Service 或认证中间件 MUST 继续校验上下文中的认证身份和会话标识
- **Then** 普通请求 Validation 层 MUST NOT 替代认证上下文安全校验

### Requirement: Auth service uses domain user model
用户会话控制能力 SHALL 通过用户领域实体读取认证流程所需的用户状态、密码哈希、外部用户 ID 和 token version。`AuthService` MUST NOT 直接依赖 Ent 用户模型，登录、改密、刷新、退出当前设备和退出全部设备的安全语义 MUST 保持不变。

#### Scenario: Login authenticates domain user
- **Given** 登录流程按用户名读取未软删除用户
- **When** 用户 Repository 返回用户领域实体
- **Then** `AuthService` MUST 使用领域实体中的密码哈希执行共享密码校验
- **Then** `AuthService` MUST 使用领域实体或 `domain.UserStatus` 方法判断普通登录、禁用状态或必须改密状态
- **Then** `AuthService` MUST NOT 为登录流程导入 Ent 用户模型

#### Scenario: Must-change-password issuance remains unchanged
- **Given** 用户领域实体表示用户状态为必须修改密码
- **When** 用户提交的密码校验通过
- **Then** `AuthService` MUST 继续签发受限改密凭据
- **Then** 系统 MUST NOT 创建普通 Redis Refresh Token 会话
- **Then** 响应语义 MUST 与现有会话控制能力保持一致

#### Scenario: Password change validates domain user state
- **Given** 受限改密凭据验证通过并解析出外部用户 ID
- **When** `AuthService` 读取用户领域实体
- **Then** `AuthService` MUST 通过领域实体或 `domain.UserStatus` 方法确认用户仍处于必须改密状态
- **Then** 状态校验通过后系统 MUST 继续通过 Repository 凭证更新契约写入新 `password_hash`、更新状态为正常并递增 token version
- **Then** 状态校验失败时响应语义 MUST 与现有改密流程保持一致

#### Scenario: Token version operations preserve repository contracts
- **Given** 刷新、退出全部设备或认证中间件需要读取或更新 token version
- **When** Service 调用用户 Repository 的 token version 相关方法
- **Then** 这些方法 MAY 继续返回标量 token version 结果
- **Then** Repository MUST 继续以 `domain.ErrUserNotFound` 表达未找到未软删除用户
- **Then** Service MUST 继续负责将领域错误映射为认证失败、token 无效、not found 或内部错误响应
