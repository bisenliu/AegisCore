## MODIFIED Requirements

### Requirement: Authenticate users and create revocable sessions
系统 SHALL 提供用户登录能力。登录成功时，系统 MUST 按 `username` 从 PostgreSQL 读取用户资料、密码哈希、外部 `user_id` 和当前 `token_version`，使用 `common` Argon2id 密码校验方法验证密码，创建新的会话标识，签发携带当前 `user_id`、`token_version` 和 `session_id` 的 Access Token，签发 Refresh Token，并在 Redis 保存 Refresh Token 会话记录和用户活跃会话索引。

#### Scenario: Login creates access and refresh tokens
- **Given** 用户名对应用户存在且密码校验通过
- **When** 调用方提交包含 `username` 和 `password` 的登录请求
- **Then** 系统 MUST 从 PostgreSQL 当前用户记录读取外部 `user_id` 和 `token_version`
- **Then** 系统 MUST 创建新的 `session_id`
- **Then** 系统 MUST 返回 Access Token 和 Refresh Token
- **Then** Access Token claims MUST 包含 UUID `user_id`、`token_version` 和 `session_id`
- **Then** Redis MUST 保存该 Refresh Token 对应的会话记录和用户活跃会话索引

#### Scenario: Login rejects invalid credentials
- **Given** 用户名不存在或密码校验失败
- **When** 调用方提交登录请求
- **Then** 系统 MUST 返回统一失败响应信封
- **Then** 系统 MUST NOT 签发 Access Token 或 Refresh Token
- **Then** 系统 MUST NOT 创建 Redis 会话记录

#### Scenario: Login does not accept email credential
- **Given** 调用方提交仅包含 `email` 和 `password` 的登录请求
- **When** controller 使用共享校验器校验登录 DTO
- **Then** 系统 MUST 返回 HTTP 400 参数校验失败响应
- **Then** 系统 MUST NOT 按邮箱查询用户
- **Then** auth service 和 repository MUST NOT 保留 `GetByEmail` 登录调用路径

### Requirement: Refresh access tokens through revocable refresh sessions
系统 SHALL 提供 Refresh Token 刷新能力。刷新时，系统 MUST 校验 Refresh Token 签名、过期时间和 Refresh Token subject，解析外部用户标识和会话标识，确认 Redis 会话仍存在，并校验该用户当前 `token_version` 与 Refresh Token 或会话记录中的版本一致；校验通过后签发新的 Access Token。系统 SHOULD 对 Refresh Token 执行轮转，使旧 Refresh Token 对应会话失效并创建新会话。Access Token 与 Refresh Token 的 subject/token type 枚举 MUST 由 `common/jwt` 统一提供，签发方法 MUST 内部强制设置对应 subject。

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

### Requirement: Store authentication session data in Redis as cache and session layer
系统 SHALL 使用 Redis 保存用户 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引。Redis 中的 `token_version` 只能作为缓存，缓存未命中或被删除时系统 MUST 使用外部 `user_id` 回源 PostgreSQL 获取真实值。

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

## ADDED Requirements

### Requirement: Verify login passwords through shared Argon2id helper
系统 SHALL 在登录认证中通过 `common` 模块的统一密码校验方法验证密码。认证服务不得直接调用底层 Argon2 API，不得使用明文比较，不得在日志或错误响应中公开密码明文、完整 hash、salt 或 hash 参数。

#### Scenario: Login uses shared password verification
- **Given** 用户存在且数据库中保存 Argon2id 密码 hash
- **When** 调用方提交 `username` 和 `password` 登录
- **Then** auth service MUST 调用 `common` 密码校验方法
- **Then** 密码校验通过时系统 MUST 继续创建认证会话

#### Scenario: Login hides password verification details
- **Given** 密码校验失败或 hash 格式非法
- **When** 系统处理登录请求
- **Then** 系统 MUST 返回统一凭据无效响应
- **Then** 响应和业务日志 MUST NOT 包含明文密码、完整 hash、salt 或 hash 参数
