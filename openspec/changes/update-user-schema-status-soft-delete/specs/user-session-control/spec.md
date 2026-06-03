## MODIFIED Requirements

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

## ADDED Requirements

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
