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

系统 SHALL 在修改密码流程中先验证受限改密凭据并解析外部用户 UUID，再执行凭证更新和会话吊销编排。受限改密凭据验证 MUST 复用 `common/security/auth.StripBearerPrefix` 支持剥离可选 `Bearer ` 前缀，MUST 解析 password-change token，MUST 校验服务端当前 `token_version` 与 token claims 一致，并 MUST 将 claims 中的 `user_id` 解析为 UUID。用户存在性检查、用户仍处于 `status=300` 的校验、新密码 hash、凭证更新和用户状态恢复 MUST 由认证凭证组件负责。修改密码成功后的用户级 token/session 失效 MUST 由认证会话组件负责。

#### Scenario: Password-change token validation rejects invalid token
- **Given** 调用方提交空白、格式非法、签名无效或 subject 非改密凭据的 token
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 更新用户凭证

#### Scenario: Password-change token validation accepts optional bearer prefix
- **Given** 调用方提交 `Bearer <password-change-token>`
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 通过 `common/security/auth.StripBearerPrefix` 剥离可选 Bearer 前缀
- **Then** 系统 MUST 按剥离后的 password-change token 执行后续校验

#### Scenario: Password-change token validation rejects changed token version
- **Given** 受限改密凭据签名有效且未过期
- **Given** token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方请求修改密码
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 查询后续状态或更新用户凭证

#### Scenario: Credential component owns password-change persistence
- **Given** 受限改密凭据通过 token 校验并解析出 UUID `user_id`
- **When** 系统继续处理修改密码请求
- **Then** 修改密码流程 MUST 调用认证凭证组件执行凭证更新
- **Then** 认证凭证组件 MUST 使用该 UUID 查询用户并校验用户仍处于 `status=300`
- **Then** 只有状态校验通过后认证凭证组件 MUST 更新 `password_hash` 并将状态更新为 `100`
- **Then** Auth Service MUST NOT 直接生成密码 hash 或直接调用用户 repository 更新凭证

#### Scenario: Session component owns password-change revocation
- **Given** 受限改密凭据通过 token 校验
- **Given** 认证凭证组件完成新密码持久化
- **When** 系统完成修改密码流程
- **Then** 修改密码流程 MUST 调用认证会话组件执行用户级会话吊销
- **Then** 认证会话组件 MUST 使旧受限改密凭据和既有认证会话失效
- **Then** Auth Service MUST NOT 直接删除 token version 缓存或直接删除 Redis Refresh Token 会话记录

### Requirement: Refresh access tokens through revocable refresh sessions
系统 SHALL 提供 Refresh Token 刷新能力。刷新时，系统 MUST 校验 Refresh Token 签名、过期时间和 Refresh Token subject，解析外部用户标识和会话标识，确认 Redis 会话仍存在，并校验该用户当前 `token_version` 与 Refresh Token 或会话记录中的版本一致；校验通过后签发新的 Access Token。系统 SHOULD 对 Refresh Token 执行轮转，使旧 Refresh Token 对应会话失效并创建新会话。启用 Refresh Token 轮转时，系统 MUST 确保成功响应中返回的新 Refresh Token 对应的 Redis 会话已经可用，并且 MUST NOT 在新 token 签发失败或新 Redis 会话创建失败时提前撤销已通过校验的旧 Refresh Token 会话。Access Token 与 Refresh Token 的 subject/token type 枚举 MUST 由 `common/security/auth` 统一提供，签发方法 MUST 内部强制设置对应 subject。刷新流程 MUST 通过 `common/security/auth.StripBearerPrefix` 统一处理请求体中可选 Bearer 前缀。

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
- **Then** 系统 MUST 通过 `common/security/auth.StripBearerPrefix` 剥离 `Bearer ` 前缀
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

#### Scenario: Refresh token rotation revokes old session after new session is usable
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 创建新的会话标识或刷新会话记录
- **Then** 系统 MUST 确保新的 Refresh Token 对应的 Redis 会话已经可用
- **Then** 系统 MUST 撤销旧 Refresh Token 会话记录
- **Then** 系统 MUST 返回新的 Refresh Token

#### Scenario: Rotation keeps old session when new token signing fails
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **Given** 新 token 签发失败
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 撤销已通过校验的旧 Refresh Token 会话记录
- **Then** 调用方后续 MAY 使用旧 Refresh Token 重试刷新

#### Scenario: Rotation keeps old session when new session creation fails
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **Given** 新 Redis 会话创建失败
- **When** 调用方请求刷新 Access Token
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 撤销已通过校验的旧 Refresh Token 会话记录
- **Then** 系统 MUST NOT 返回没有可用 Redis 会话支撑的新 Refresh Token

#### Scenario: Rotation does not expose new token when old session revocation fails
- **Given** Refresh Token 轮转已启用
- **Given** 新 token 已签发
- **Given** 新 Redis 会话已创建
- **Given** 旧 Refresh Token 会话撤销失败
- **When** 系统处理刷新响应
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 向调用方返回新 Refresh Token
- **Then** 系统 SHOULD 尝试清理已创建但未返回给调用方的新 Redis 会话

#### Scenario: Strict replay prevention uses atomic rotation
- **Given** Refresh Token 轮转已启用
- **Given** 系统安全目标要求防止同一个旧 Refresh Token 被并发重放
- **When** 系统执行 Refresh Token 轮转
- **Then** 系统 MUST 使用 Redis 事务、Lua 脚本或等价机制原子完成旧会话校验、新会话创建和旧会话撤销
- **Then** 系统 MUST 避免命令间失败或并发刷新造成旧会话和新会话同时可用或同时不可用的中间状态

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
系统 SHALL 支持退出全部设备。退出全部设备时，系统 MUST 通过认证会话组件在 PostgreSQL 中原子递增该用户 `token_version`，更新成功后将 Redis 中该用户的 token version cache 覆盖为递增后的新版本，并删除全部活跃会话记录和会话索引。`AuthService` MUST 仅从认证上下文提取当前用户身份并调用认证会话组件，不得直接执行用户 repository 写入或 Redis 会话清理。

#### Scenario: Logout all sessions invalidates tokens
- **Given** 请求已通过 Access Token 认证
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先在 PostgreSQL 中递增该用户 `token_version`
- **Then** 系统 MUST 将 Redis 中该用户的 token version cache 写入为递增后的新版本
- **Then** 系统 MUST 删除该用户所有 Redis Refresh Token 会话记录
- **Then** 系统 MUST 清空该用户活跃会话索引
- **Then** 旧 Access Token MUST 因版本不一致而失效

#### Scenario: Auth session lifecycle owns logout all writes
- **Given** 请求已通过 Access Token 认证
- **When** `AuthService` 处理退出全部设备流程
- **Then** `AuthService` MUST 从认证上下文提取并校验当前 `user_id`
- **Then** `AuthService` MUST 调用认证会话组件执行全部会话吊销
- **Then** 认证会话组件 MUST 调用用户 repository 原子递增 `token_version`
- **Then** 认证会话组件 MUST 调用 auth session repository 写入新 token version cache 并清理全部 Refresh Token 会话记录
- **Then** `AuthService` MUST NOT 直接持有用户 repository 来完成退出全部设备写操作

#### Scenario: Logout all fails when token version cache cannot be refreshed
- **Given** PostgreSQL 已成功递增该用户 `token_version`
- **Given** Redis 中该用户可能仍存在旧 token version cache
- **When** 系统无法将 Redis token version cache 写入为递增后的新版本
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 将该次退出全部设备报告为成功

### Requirement: Store authentication session data in Redis as cache and session layer
系统 SHALL 使用 Redis 保存用户 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引。Redis 中的 `token_version` 只能作为缓存，缓存未命中或被删除时系统 MUST 由认证会话 service 组件或 token version resolver 使用外部 `user_id` 回源 PostgreSQL 获取真实值。用户级安全事件递增 PostgreSQL `token_version` 后，系统 MUST 将 Redis token version cache 覆盖为递增后的新版本，不得只删除旧缓存作为成功路径。认证会话 Redis key MUST 使用 `config.App.Name` 作为前缀来源；系统 MUST NOT 校验 `app.name`，MUST NOT 为 `app.name` 设置代码级默认值。当 `config.App.Name` 去除首尾空白后非空时，token version 缓存 key MUST 为 `<app.name>:auth:user:<user_id>:token_version`，Refresh Token 会话记录 key MUST 为 `<app.name>:auth:session:<session_id>`，用户活跃会话索引 key MUST 为 `<app.name>:auth:user:<user_id>:sessions`。当 `config.App.Name` 去除首尾空白后为空时，Redis key MUST 保持无前缀业务格式：`auth:user:<user_id>:token_version`、`auth:session:<session_id>` 和 `auth:user:<user_id>:sessions`。用户活跃会话索引 MUST 使用 Redis ZSet，member MUST 使用 `session_id`，score MUST 使用该会话过期时间的 Unix 时间戳。系统 MUST 以 Redis session key 的实际 TTL 推导会话过期时间，并使会话 payload 中的 `ExpiresAt`、session key TTL 和用户活跃会话索引 score 保持一致。系统 MUST 在写入、读取或删除该用户活跃会话索引时，按当前 Unix 时间戳执行过期 member 清理。系统 MUST 为用户活跃会话索引设计过期或清理策略，避免没有活跃会话的 ZSet key 和已过期 `session_id` 长期残留。

#### Scenario: Token version cache miss reads PostgreSQL
- **Given** Redis 中不存在某用户的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** 系统 MUST 使用外部 `user_id` 查询 PostgreSQL 获取真实 `token_version`
- **Then** 系统 MUST 将真实版本回填 Redis 缓存

#### Scenario: User security update refreshes Redis cache after PostgreSQL update
- **Given** 用户级安全事件需要整体失效 token
- **When** 系统处理该安全事件
- **Then** 系统 MUST 先更新 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 再将 Redis token version cache 写入为 PostgreSQL 返回的新版本
- **Then** 系统 MUST NOT 只更新 Redis 而不更新 PostgreSQL

#### Scenario: Token version cache refresh failure is not successful revocation
- **Given** 用户级安全事件已更新 PostgreSQL 中的 `token_version`
- **When** Redis token version cache 无法写入 PostgreSQL 返回的新版本
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 在旧 Redis cache 仍可能被认证路径信任时报告吊销成功

#### Scenario: Session creation indexes expiration timestamp
- **Given** 登录或 Refresh Token 轮转需要创建新的 Redis 会话记录
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 系统保存会话记录和用户活跃会话索引
- **Then** 系统 MUST 将 `aegiscore-user-services:auth:session:<session_id>` 保存为带 TTL 的会话记录
- **Then** 系统 MUST 将 `session_id` 写入 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet
- **Then** 该 ZSet member 的 score MUST 等于该会话过期时间的 Unix 时间戳
- **Then** 系统 MUST 在写入索引时清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Session creation uses one expiration source
- **Given** 登录或 Refresh Token 轮转需要创建新的 Redis 会话记录
- **Given** 调用方传入的会话 payload 包含空白或非空白 `ExpiresAt`
- **When** 系统根据有效 Refresh Token TTL 保存会话记录和用户活跃会话索引
- **Then** 系统 MUST 以该 TTL 和当前时间推导唯一会话过期时间
- **Then** Redis session key 的 TTL MUST 与该推导过期时间一致
- **Then** 序列化会话 payload 中的 `ExpiresAt` MUST 与该推导过期时间一致
- **Then** 用户活跃会话 ZSet member 的 score MUST 与该推导过期时间一致
- **Then** 系统 MUST NOT 让调用方传入的旧 `ExpiresAt` 导致 ZSet score 与 session key 实际过期时间不一致

#### Scenario: User session index receives expiration or bounded cleanup
- **Given** 系统创建或轮转 Refresh Token 会话
- **When** 系统写入用户活跃会话 ZSet 索引
- **Then** 系统 MUST 为该 ZSet key 设置可使无活跃会话索引最终消失的过期策略，或提供等价的有界清理策略
- **Then** 该策略 MUST NOT 在仍存在未过期会话时提前删除用户活跃会话索引
- **Then** 过期 `session_id` MUST NOT 只能依赖未来批量退出操作才被清理

#### Scenario: Empty app name keeps unprefixed Redis keys
- **Given** `config.App.Name` 去除首尾空白后为空
- **When** 系统保存 token version 缓存、Refresh Token 会话记录或用户活跃会话索引
- **Then** 系统 MUST 使用 `auth:user:<user_id>:token_version` 作为 token version 缓存 key
- **Then** 系统 MUST 使用 `auth:session:<session_id>` 作为 Refresh Token 会话记录 key
- **Then** 系统 MUST 使用 `auth:user:<user_id>:sessions` 作为用户活跃会话索引 key
- **Then** 系统 MUST NOT 使用代码级默认服务名补齐 Redis key 前缀

#### Scenario: Logout current session removes indexed member and stale members
- **Given** 请求已通过 Access Token 认证
- **Given** 认证上下文包含当前 `user_id` 和 `session_id`
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 调用方请求退出当前设备
- **Then** 系统 MUST 删除当前 `session_id` 对应的 Redis 会话记录
- **Then** 系统 MUST 从 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet 移除当前 `session_id`
- **Then** 系统 MUST 清理该 ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member

#### Scenario: Logout all sessions reads only non-expired indexed members
- **Given** 请求已通过 Access Token 认证
- **Given** `config.App.Name` 去除首尾空白后为 `aegiscore-user-services`
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先清理 `aegiscore-user-services:auth:user:<user_id>:sessions` ZSet 中所有 score 小于或等于当前 Unix 时间戳的过期 member
- **Then** 系统 MUST 从该 ZSet 读取仍未过期的 `session_id`
- **Then** 系统 MUST 删除读取到的 Redis 会话记录
- **Then** 系统 MUST 删除或清空该用户活跃会话索引

#### Scenario: Stale index data does not inflate future session operations
- **Given** 用户活跃会话 ZSet 中存在已过期 `session_id` member
- **When** 系统创建新会话、删除当前会话或删除该用户全部会话
- **Then** 系统 MUST 在读取或写入业务相关 member 前清理按 score 已过期的 member
- **Then** 系统 MUST 避免长期遍历已过期残留作为活跃会话
- **Then** 后续会话统计、管理或审计能力 MUST 能基于清理后的索引语义区分活跃会话和过期残留

### Requirement: Separate token version cache from database lookup strategy

用户会话控制能力 SHALL 将 Redis token version cache 操作与 PostgreSQL token version 读取策略分离。Redis auth session repository MUST 只负责 Redis 会话记录、用户活跃会话索引、token version cache key 的读取、写入和删除；它 MUST NOT 直接依赖用户 repository 或在缓存未命中时自行回源 PostgreSQL。认证会话 service 组件或专门的 token version resolver MUST 组合 Redis cache 与 `UserTokenVersionRepository`，并保持缓存未命中时回源 PostgreSQL、成功后回填 Redis 缓存的行为。

#### Scenario: Cache hit uses Redis value without database lookup
- **Given** Redis 中存在某用户有效的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** 系统 MUST 使用 Redis 缓存中的版本值
- **Then** 系统 MUST NOT 查询 PostgreSQL 获取该用户的 token version

#### Scenario: Cache miss is resolved outside Redis repository
- **Given** Redis 中不存在某用户的 token version 缓存
- **When** 系统需要校验该用户的 token version
- **Then** Redis auth session repository MUST 只报告缓存未命中或等价结果
- **Then** 认证会话 service 组件或 token version resolver MUST 使用外部 `user_id` 回源 PostgreSQL 获取真实 `token_version`
- **Then** 回源成功后系统 MUST 将真实版本回填 Redis 缓存

#### Scenario: Redis repository does not depend on user repository
- **Given** 用户服务运行时通过 Fx 构造 Redis auth session repository
- **When** 查看 Redis auth session repository 的构造参数和字段
- **Then** Redis auth session repository MUST 依赖具名 `cache_redis` Redis client、Redis key builder 和认证缓存 TTL 配置
- **Then** Redis auth session repository MUST NOT 持有 `UserRepository` 或 `UserTokenVersionRepository`

#### Scenario: Invalid token version cache falls back through resolver
- **Given** Redis 中存在无法解析或非正数的 token version 缓存值
- **When** 系统需要校验该用户的 token version
- **Then** Redis auth session repository MUST 将该值视为无效缓存
- **Then** 认证会话 service 组件或 token version resolver MUST 回源 PostgreSQL 获取真实 `token_version`
- **Then** 回源成功后系统 MUST 使用真实版本覆盖 Redis 缓存

#### Scenario: External behavior remains compatible
- **Given** 调用方执行登录、刷新、修改密码、退出当前设备或退出全部设备流程
- **When** token version cache 与 PostgreSQL 读取策略完成解耦
- **Then** 系统 MUST 保持现有 HTTP 路由、请求体、响应信封、错误码和认证语义不变
- **Then** 系统 MUST 继续使用 `auth.token_version_cache_ttl` 控制 token version 缓存 TTL
- **Then** 系统 MUST NOT 修改 Ent schema、Atlas migration 或 Redis key 格式

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
系统 SHALL 在登录认证中通过 `common/security/password` 的统一密码校验方法验证密码。认证服务不得直接调用底层 Argon2 API，不得使用明文比较，不得在日志或错误响应中公开密码明文、完整 hash、salt 或 hash 参数。

#### Scenario: Login uses shared password verification
- **Given** 用户存在且数据库中保存 Argon2id 密码 hash
- **When** 调用方提交 `username` 和 `password` 登录
- **Then** auth service MUST 调用 `common/security/password` 密码校验方法
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

系统 SHALL 在修改密码成功后通过用户仓储的通用凭证更新契约持久化新凭证。凭证更新 MUST 面向未软删除用户，MUST 写入新的 `password_hash`，MUST 更新目标用户状态，MUST 递增 PostgreSQL 中的 `token_version`，并 MUST 返回更新后的 `token_version`。系统 MUST NOT 在该流程中读取或写入旧 `password` 字段。修改密码流程完成凭证更新后，系统 MUST 将 Redis token version cache 覆盖为更新后的新版本，使旧凭据和旧认证会话不再因为旧缓存命中而继续通过版本校验。

#### Scenario: Password change updates credentials and invalidates tokens
- **Given** 用户存在且 `deleted_at` 为 `NULL`
- **Given** 用户当前状态为 `300`
- **Given** 调用方持有有效的受限改密凭据
- **When** 调用方提交有效的新密码并完成密码哈希
- **Then** 系统 MUST 通过凭证更新契约写入新的 `password_hash`
- **Then** 系统 MUST 将用户状态更新为 `100`
- **Then** 系统 MUST 递增 PostgreSQL 中的 `token_version`
- **Then** 系统 MUST 将 Redis token version cache 写入为递增后的新版本，使旧凭据不可继续用于改密

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

#### Scenario: Password change fails when token version cache cannot be refreshed
- **Given** 用户凭证更新已成功递增 PostgreSQL `token_version`
- **When** 系统无法将 Redis token version cache 写入为递增后的新版本
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 将该次修改密码报告为成功

### Requirement: Authentication sessions use repository abstraction with Redis implementation boundary
用户会话控制能力 SHALL 通过认证 app 层声明的 `authapp.AuthSessionStore` 抽象管理 token version、Refresh Token 会话和用户活跃会话索引，具体 Redis 实现 MUST 位于 `user-services/internal/features/auth/store/redis` 包。service 层 MUST NOT 定义或持有 Redis session store 具体实现。

#### Scenario: Auth service depends on auth session repository abstraction
- **Given** 登录、刷新、退出当前设备、退出全部设备或修改密码流程需要访问会话状态
- **When** auth service 调用会话数据访问层
- **Then** auth service MUST 依赖 `authapp.AuthSessionStore` 或更高层 session lifecycle 组件
- **Then** auth service MUST 使用 `authdomain.AuthSession` 表达会话数据
- **Then** auth service MUST NOT 依赖 Redis client 或 `features/auth/store/redis` 私有实现类型

#### Scenario: Session not found error remains mappable
- **Given** Redis 中不存在指定 Refresh Token 会话记录
- **When** auth service 读取会话
- **Then** Redis 实现 MUST 返回 `authdomain.ErrAuthSessionNotFound`
- **Then** auth service MUST 继续将该错误映射为未认证或 token 无效响应

#### Scenario: Token version mismatch remains mappable
- **Given** token claims 或会话记录中的 `token_version` 与服务端当前版本不一致
- **When** 系统校验 token version
- **Then** auth session store 或 lifecycle 组件 MUST 返回认证领域 token version mismatch 错误
- **Then** 系统 MUST 继续拒绝刷新、受保护请求或改密凭据校验

#### Scenario: Redis session storage behavior remains compatible
- **Given** `features/auth/store/redis` 承载认证会话 Redis 实现
- **When** 系统创建、读取、删除或批量删除认证会话
- **Then** Redis key 格式、Refresh Token 会话 TTL、用户活跃会话 ZSet 和过期 member 清理行为 MUST 与迁移前保持一致
- **Then** token version 缓存未命中时 Redis 实现 MUST 只报告缓存未命中或等价结果，由认证会话 service 组件或 token version resolver 回源 PostgreSQL

### Requirement: Authentication token validator uses auth session repository abstraction
用户服务运行时 SHALL 将 `authapp.TokenVersionValidator` 作为认证中间件 token version validator 的依赖来源。该抽象 MUST 提供 `ValidateTokenVersion(ctx, userID, tokenVersion)`，并保持现有 token version 校验语义。

#### Scenario: Protected route token validation remains compatible
- **Given** 用户服务注册受保护路由认证中间件
- **When** 中间件需要校验 Access Token 的 token version
- **Then** 中间件 MUST 使用 Fx 注入的 `authapp.TokenVersionValidator`
- **Then** 有效 token MUST 继续允许进入受保护 handler
- **Then** 版本不一致 token MUST 继续在进入 handler 前被拒绝

### Requirement: User session repository returns domain user-not-found errors

用户会话控制能力 SHALL 保持 service/repository 分层边界。PostgreSQL 用户 repository 在会话控制流程中读取 `token_version`、递增 `token_version` 或更新用户凭证时，若目标未软删除用户不存在，MUST 返回 `domain.ErrUserNotFound`，MUST NOT 直接构造 `common/contract/response` 应用错误。service 层 MUST 负责将该领域错误映射为现有认证失败、token 无效、not found 或内部错误响应语义。

#### Scenario: Token version lookup misses user
- **Given** Redis token version 缓存未命中
- **Given** PostgreSQL 中不存在对应未软删除用户
- **When** 用户 repository 读取该用户 `token_version`
- **Then** PostgreSQL repository MUST 返回 `domain.ErrUserNotFound`
- **Then** PostgreSQL repository MUST NOT 返回 `response.NotFoundError` 或其他 `common/contract/response` 应用错误
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
系统 MUST 在 Auth Service 执行认证会话业务编排前完成认证请求的请求级清洗和基础校验。登录请求的 `username` 和 `password` 裁剪、空凭据校验，改密请求的 `new_password` 裁剪和空值校验，以及刷新请求体 `refresh_token` 的可选 Bearer 前缀剥离和空值校验 MUST 位于 Controller、共享请求校验器或服务内 validators 层，而不是作为 Auth Service 的主要职责。

#### Scenario: Normalize login credentials before authentication
- **Given** 调用方提交登录请求且 `username` 或 `password` 前后包含空白
- **When** controller 处理登录请求并调用 Auth Service
- **Then** 空白裁剪 MUST 在 Controller 或服务内 validators 层完成
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
- **Then** 新密码空白裁剪和裁剪后空值校验 MUST 在 Controller 或服务内 validators 层完成
- **Then** Auth Service MUST 使用已规范化的新密码执行密码哈希和凭证更新流程

#### Scenario: Normalize refresh token request before refresh flow
- **Given** 调用方在刷新请求体 `refresh_token` 字段提交裸 Refresh Token 或 `Bearer <refresh-token>`
- **When** controller 处理刷新请求并调用 Auth Service
- **Then** 请求体 token 的可选 Bearer 前缀剥离和空值校验 MUST 在 Controller 或服务内 validators 层完成
- **Then** Auth Service MUST 使用已规范化的 Refresh Token 执行 token claims、session 和 token version 校验

### Requirement: Keep authentication and session semantics in Auth Service
系统 MUST 保持认证会话安全语义由 Auth Service 或认证中间件边界负责。validators 层 MUST NOT 执行依赖 JWT claims、Redis session、token version、用户状态或 Repository 查询的认证业务校验。

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
- **Then** 普通请求 validators 层 MUST NOT 替代认证上下文安全校验

### Requirement: Separate auth orchestration from credential token and session strategies

用户会话控制能力 SHALL 将认证用例编排与凭证校验、凭证更新、token 签发解析和会话管理策略分离。`AuthService` MUST 继续作为登录、修改密码、刷新 token、退出当前设备和退出全部设备的统一入口，并 MUST 保持现有 HTTP 契约、响应信封、错误映射、token claims、Redis 会话行为和 token_version 行为不变。凭证校验和凭证更新 MUST 由认证凭证组件承载，token 签发和解析 MUST 由认证 token 组件承载，Refresh Token 会话生命周期和用户级会话吊销 MUST 由认证会话组件承载，而不是持续堆叠在 `AuthService` 的用例方法中。

#### Scenario: Auth service orchestrates login without owning credential and token strategies
- **Given** 用户提交登录请求
- **When** `AuthService` 处理登录流程
- **Then** 系统 MUST 通过独立凭证组件读取用户认证资料并校验密码
- **Then** `AuthService` MUST 根据用户状态编排普通 token pair 签发或受限改密凭据签发
- **Then** token TTL 兜底、JWT 签发和 Redis Refresh Token 会话创建 MUST 由独立 token 或 session 组件执行
- **Then** 登录成功、无效凭证、禁用用户和必须改密用户的外部行为 MUST 与拆分前保持一致

#### Scenario: Auth service refreshes tokens through token and session components
- **Given** 调用方提交 Refresh Token
- **When** `AuthService` 处理刷新流程
- **Then** 系统 MUST 通过独立 token 组件解析和校验 Refresh Token claims
- **Then** 系统 MUST 通过独立 session 组件校验 Redis 会话存在性、会话 claims 一致性和当前 token_version
- **Then** `AuthService` MUST 继续按配置编排 Refresh Token rotation
- **Then** 新 token 签发、旧会话删除、新会话创建和失败响应语义 MUST 与拆分前保持一致

#### Scenario: Password change delegates credential update and revocation
- **Given** 调用方持有受限改密凭据并提交新密码
- **When** `AuthService` 处理修改密码流程
- **Then** 系统 MUST 通过独立 token 组件解析受限改密凭据
- **Then** 系统 MUST 通过独立 session 组件校验服务端当前 `token_version` 与 token claims 一致
- **Then** `AuthService` MUST 调用独立凭证组件完成用户状态校验、密码 hash、凭证更新和状态恢复
- **Then** `AuthService` MUST 调用独立 session 组件完成用户级 token/session 吊销
- **Then** `AuthService` MUST NOT 直接读取用户状态、hash 新密码、调用用户 repository 更新凭证、删除 token version 缓存或删除 Redis 会话
- **Then** 用户状态校验、凭证更新和受限改密凭据失效语义 MUST 与拆分前保持一致

#### Scenario: Logout flows keep session semantics unchanged
- **Given** 请求已通过 Access Token 认证
- **When** `AuthService` 处理退出当前设备或退出全部设备流程
- **Then** 系统 MUST 继续在 service 边界校验认证上下文中的 `user_id` 和 `session_id`
- **Then** 退出当前设备 MUST 通过独立 session 组件删除当前 Redis Refresh Token 会话
- **Then** 退出全部设备 MUST 通过独立 session 组件先递增 PostgreSQL `token_version`，再写入新 token version cache 并删除所有 Redis Refresh Token 会话
- **Then** `AuthService` MUST NOT 直接调用用户 repository 递增 `token_version` 或直接删除 Redis token version 缓存和会话记录
- **Then** 当前设备退出和全部设备退出的外部行为 MUST 与拆分前保持一致

#### Scenario: Components remain inside service layer boundaries
- **Given** 认证能力需要拆分凭证、token 和 session 策略
- **When** 实现新增组件或领域服务
- **Then** 组件 MUST 位于 `user-services/internal/features/auth/app` 或等价 service 层边界内
- **Then** 组件 MUST 依赖 `authapp.UserCredentialStore`、`authapp.UserTokenVersionStore`、`authapp.AuthSessionStore`、`common/security/auth`、`common/security/password` 和配置等现有抽象
- **Then** 组件 MUST NOT 直接依赖 Ent 生成模型、Redis client、controller、router 或 HTTP response writer
- **Then** repository 层 MUST 继续只负责数据访问，controller 层 MUST 继续只负责 HTTP 请求解析和响应输出

#### Scenario: Auth service stores only orchestration dependencies
- **Given** `AuthService` 由 Fx 构造函数创建
- **When** 开发者检查 `authService` 结构体字段
- **Then** `authService` MUST 只保存认证凭证组件、认证 token 组件、认证会话组件和必要的高层编排策略
- **Then** `authService` MUST NOT 保存原始 JWT service 作为字段
- **Then** `authService` MUST NOT 保存用户 repository 作为字段用于凭证更新或 token version 递增

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

### Requirement: Authentication API contracts are grouped by capability

用户会话控制能力 SHALL 使用按业务能力组织的认证 API 契约包承载登录、刷新、强制改密、登出和 token 响应模型。实现 MUST NOT 依赖全局 DTO 包表达认证会话契约，并 MUST 保持认证相关 HTTP API、token 语义、Redis 会话行为和响应结构不变。

#### Scenario: Auth contract types use auth API package
- **WHEN** controller、service、validation 或测试引用登录请求、刷新请求、改密请求、token 响应、改密响应或登出响应
- **THEN** 这些引用 MUST 来自认证 API 契约包
- **THEN** 这些引用 MUST NOT 来自全局 DTO 包

#### Scenario: Login and refresh contracts remain compatible
- **WHEN** 认证请求和响应类型迁移完成
- **THEN** 登录请求 MUST 继续使用 `username` 和 `password` 字段
- **THEN** 刷新请求 MUST 继续使用 `refresh_token` 字段并兼容裸 token 或 Bearer 值
- **THEN** token 响应 MUST 继续包含 `access_token`、可选 `refresh_token`、`token_type`、`expires_in` 和可选 `password_change_required`

#### Scenario: Password change and logout contracts remain compatible
- **WHEN** 改密和登出响应类型迁移完成
- **THEN** 改密请求 MUST 继续从 Authorization header 接收受限 token 并从 JSON 请求体接收 `new_password`
- **THEN** 改密响应 MUST 继续使用 `changed` 字段表达完成状态
- **THEN** 登出响应 MUST 继续使用 `logged_out` 字段表达完成状态

#### Scenario: Auth service semantics remain unchanged
- **WHEN** 认证 API 契约类型迁移完成
- **THEN** 登录、刷新、强制改密、退出当前设备和退出全部设备的认证边界、token claims、token version 校验、Redis 会话生命周期和失败响应语义 MUST 保持不变

### Requirement: Refresh orchestration remains readable and strategy-oriented

用户会话控制能力 SHALL 保持 `AuthService.Refresh` 的高层用例编排清晰。刷新流程 MUST 将请求规范化、Refresh Token claims 解析、Refresh 会话校验、非轮转刷新、轮转刷新和轮转失败处理表达为职责明确的内部方法或组件边界。`AuthService.Refresh` MUST NOT 直接堆叠 Redis 旧会话校验、新会话创建、旧会话删除和补偿清理的低层细节。

#### Scenario: Refresh method selects a refresh strategy
- **Given** 调用方提交 Refresh Token 请求
- **When** `AuthService.Refresh` 处理请求
- **Then** 系统 MUST 先完成请求规范化、Refresh Token claims 解析和 Refresh 会话校验
- **Then** 系统 MUST 根据 Refresh Token 轮转配置选择非轮转刷新或轮转刷新策略
- **Then** 轮转策略的会话写入和撤销细节 MUST 位于职责明确的内部辅助方法、session lifecycle 组件或 repository 抽象内

#### Scenario: Non-rotation refresh keeps current session semantics
- **Given** Refresh Token 轮转未启用
- **Given** Refresh Token 校验通过且 Redis 中存在对应会话
- **When** 调用方请求刷新 token
- **Then** 系统 MUST 复用当前 `session_id` 签发新的 Access Token 和 Refresh Token
- **Then** 系统 MUST 保持现有响应信封、错误映射、JWT claims 和 Redis key 格式不变

### Requirement: Refresh token rotation consumes old sessions atomically

启用 Refresh Token 轮转时，系统 SHALL 将旧 Refresh 会话仍有效的校验、新 Refresh 会话创建、用户会话索引更新和旧 Refresh 会话撤销作为一个原子提交动作执行。该原子动作 MUST 在 Redis repository 或等价持久化边界内实现，并 MUST 覆盖多 goroutine、多进程和多服务实例并发刷新场景。系统 MUST NOT 依赖服务进程内互斥锁作为主要重放防护机制。

#### Scenario: Concurrent rotation succeeds once for the same old refresh token
- **Given** Refresh Token 轮转已启用
- **Given** Redis 中存在旧 Refresh Token 对应的会话记录
- **When** 两个或多个请求并发使用同一个旧 Refresh Token 执行刷新
- **Then** 系统 MUST 最多只允许一个请求完成旧会话消费并返回新的 Refresh Token
- **Then** 其他请求 MUST 因旧会话已不存在、已被消费或会话状态不匹配而失败
- **Then** Redis 中 MUST NOT 同时保留多个由同一个旧 Refresh Token 并发轮转成功产生的新会话

#### Scenario: Atomic rotation leaves no split-brain session state
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **When** 系统提交 Refresh Token 轮转
- **Then** 旧会话存在性校验、新会话写入、用户会话索引写入、旧会话删除和旧索引移除 MUST 作为 Redis Lua 脚本、Redis 事务或等价机制中的一个原子动作完成
- **Then** 系统 MUST 避免因命令间失败造成旧会话和新会话同时可用或同时不可用的中间状态

#### Scenario: Rotation failure does not expose an unusable new refresh token
- **Given** Refresh Token 轮转已启用
- **Given** 新 token 已在内存中签发
- **Given** Redis 原子轮转提交失败、旧会话已被其他请求消费或旧会话状态不匹配
- **When** 系统处理刷新响应
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 向调用方返回该新 Refresh Token
- **Then** 该失败 MUST NOT 破坏已经由其他请求成功提交的会话状态

#### Scenario: Token signing failure keeps old refresh session usable
- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **Given** 新 token 签发失败
- **When** 调用方请求刷新 token
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 消费或撤销已通过校验的旧 Refresh 会话
- **Then** 调用方后续 MAY 使用旧 Refresh Token 重试刷新

### Requirement: Use explicit authentication session handler names
用户会话控制能力 SHALL 在 auth controller 和路由注册中使用能独立表达认证或会话动作的 handler 名称。实现 MUST 保持 `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/logout`、`/api/v1/auth/logout-all` 和 `/api/v1/auth/change-password` 的 HTTP 契约、认证边界、响应信封、错误语义、Redis 会话行为和 token version 行为不变。

#### Scenario: Login route uses explicit handler name
- **Given** 公开认证路由已注册
- **When** 开发者检查 `POST /api/v1/auth/login` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `AuthController.LoginUser`
- **Then** 路由 MUST NOT 引用 `AuthController.Login`

#### Scenario: Refresh route uses explicit handler name
- **Given** 公开认证路由已注册
- **When** 开发者检查 `POST /api/v1/auth/refresh` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `AuthController.RefreshToken`
- **Then** 路由 MUST NOT 引用 `AuthController.Refresh`

#### Scenario: Logout routes use explicit session handler names
- **Given** 受保护认证路由已注册
- **When** 开发者检查退出当前设备和退出全部设备的 Gin handler 引用
- **Then** `POST /api/v1/auth/logout` 路由 MUST 引用 `AuthController.LogoutCurrentSession`
- **Then** `POST /api/v1/auth/logout-all` 路由 MUST 引用 `AuthController.LogoutAllSessions`
- **Then** 路由 MUST NOT 引用 `AuthController.Logout` 或 `AuthController.LogoutAll`

#### Scenario: Authentication session API behavior remains unchanged
- **Given** 调用方按现有契约请求认证或会话接口
- **When** 系统处理登录、刷新 token、退出当前设备或退出全部设备请求
- **Then** 系统 MUST 保持现有 token 签发、刷新、撤销、token version 和统一响应行为
- **Then** controller、service、session store 和 repository 的职责边界 MUST 保持不变

### Requirement: Authentication uses credential and token version repository interfaces

用户会话控制能力 SHALL 将认证凭证访问与 token version 访问拆分为独立仓储接口。认证凭证组件 MUST 依赖凭证仓储接口读取认证所需用户资料并更新凭证；认证会话组件 MUST 依赖 token version 仓储接口读取和原子递增用户 token version。认证服务和认证组件 MUST NOT 为登录、改密、刷新或退出全部设备流程依赖包含用户资料创建、用户列表查询和其他无关方法的完整用户仓储大接口。

#### Scenario: Credential component declares credential repository dependency
- **Given** 登录或修改密码流程需要读取用户认证资料或更新用户凭证
- **When** 认证凭证组件声明仓储依赖
- **Then** 认证凭证组件 MUST 依赖凭证仓储接口
- **Then** 该接口 MUST 覆盖按用户名读取认证资料和更新凭证所需方法
- **Then** 认证凭证组件 MUST NOT 依赖用户列表查询或 token version 原子递增能力

#### Scenario: Session component declares token version repository dependency
- **Given** 刷新、退出全部设备或 token version 校验流程需要读取或递增用户 token version
- **When** 认证会话组件声明仓储依赖
- **Then** 认证会话组件 MUST 依赖 token version 仓储接口
- **Then** 该接口 MUST 覆盖读取 token version 和原子递增 token version 所需方法
- **Then** 认证会话组件 MUST NOT 依赖用户资料创建、用户列表查询或凭证更新能力

#### Scenario: Auth service construction injects separated repository capabilities
- **Given** Fx 构造认证服务及其内部凭证、token 和会话组件
- **When** 依赖注入容器提供 PostgreSQL 用户仓储实现
- **Then** 同一个底层 PostgreSQL 用户仓储实例 MUST 能以凭证仓储接口和 token version 仓储接口身份注入
- **Then** Fx 装配 MUST NOT 为不同小接口重复创建多个语义独立的 PostgreSQL 用户仓储实例
- **Then** `AuthService` MUST 继续只保存认证凭证组件、认证 token 组件、认证会话组件和必要编排策略

#### Scenario: Authentication behavior remains compatible
- **Given** PostgreSQL 用户仓储实现通过小接口提供认证凭证和 token version 能力
- **When** 系统处理登录、修改密码、刷新 token、退出当前设备或退出全部设备请求
- **Then** token 签发、Refresh Token 会话、token version 校验与递增、Redis 会话清理和统一响应行为 MUST 与迁移前保持一致
- **Then** service 层 MUST 继续负责领域错误到认证失败、token 无效、not found 或内部错误响应的映射

#### Scenario: Authentication tests use narrow fakes
- **Given** 单元测试只验证登录凭证校验流程
- **When** 测试构造认证凭证组件的仓储替身
- **Then** 测试替身 MUST 只需要实现凭证读取或凭证更新相关方法
- **Then** 测试替身 MUST NOT 为用户资料创建、用户列表查询或 token version 递增提供无关空实现

#### Scenario: Token version tests use narrow fakes
- **Given** 单元测试只验证刷新、改密凭据校验或退出全部设备中的 token version 行为
- **When** 测试构造认证会话组件的仓储替身
- **Then** 测试替身 MUST 只需要实现 token version 读取和递增相关方法
- **Then** 测试替身 MUST NOT 为用户资料创建、用户列表查询、按用户名读取或凭证更新提供无关空实现
