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

### Requirement: Refresh access tokens through revocable refresh sessions
系统 SHALL 提供 Refresh Token 刷新能力。刷新时，系统 MUST 校验 Refresh Token 签名、过期时间和 Refresh Token subject，解析外部用户标识和会话标识，确认 Redis 会话仍存在，并校验该用户当前 `token_version` 与 Refresh Token 或会话记录中的版本一致；校验通过后签发新的 Access Token。系统 SHOULD 对 Refresh Token 执行轮转，使旧 Refresh Token 对应会话失效并创建新会话。Access Token 与 Refresh Token 的 subject/token type 枚举 MUST 由 `common/auth` 统一提供，签发方法 MUST 内部强制设置对应 subject。

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
- **Then** 系统 MUST 在解析前剥离 `Bearer ` 前缀
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
