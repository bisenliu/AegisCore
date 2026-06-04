## ADDED Requirements

### Requirement: User service logs include necessary context
用户服务 MUST 通过 `common/logger` 的 context API 输出关键业务日志，并 MUST 自动关联 `trace-id`。关键错误日志 MUST 包含错误对象和必要业务标识，例如 user_id、username、session_id、分页参数或状态字段。用户服务 MUST NOT 为本次变更新增私有日志框架或强制追加 module、method、event、error_code、error_kind、reason 等额外通用字段。

#### Scenario: Error log includes trace and operation context
- **Given** HTTP 请求携带或生成了 `X-Trace-ID`
- **When** 用户服务在 service 层记录系统错误
- **Then** 日志 MUST 包含 `trace-id`
- **Then** 日志 MUST 包含错误对象以及必要业务标识
- **Then** 日志 MUST NOT 依赖用户服务私有日志框架才能输出

#### Scenario: User profile query error is searchable
- **Given** 查询用户资料时 repository 返回非用户不存在错误
- **When** 用户服务记录查询失败日志
- **Then** 日志 MUST 使用 error 级别
- **Then** 日志 MUST 包含 `user_id` 和错误字段
- **Then** 日志 MUST 可通过 `trace-id` 或 `user_id` 检索到该失败

#### Scenario: User list error includes pagination context
- **Given** 用户列表查询包含分页参数
- **When** 用户服务记录列表查询失败日志
- **Then** 日志 MUST 包含 page 和 page_size
- **Then** 日志 MUST NOT 记录过细的筛选条件或请求 body
- **Then** 日志 MUST NOT 包含任何密码、token 或数据库连接凭证

### Requirement: Error levels reflect operational severity
用户服务 MUST 按错误可操作性选择日志级别。依赖故障、未知异常、序列化失败、token 签发失败和持久化失败 MUST 使用 error 级别。认证失败、token 无效、会话缺失、用户不存在、唯一冲突和状态不允许等可预期业务拒绝 MUST 使用 warn 级别或不重复记录为 error。

#### Scenario: Dependency failure is error level
- **Given** Redis、PostgreSQL、JWT 签发或密码哈希流程返回系统错误
- **When** 用户服务记录该失败
- **Then** 日志 MUST 使用 error 级别
- **Then** 日志 MUST 包含底层错误和调用方法上下文
- **Then** 日志 MUST 携带错误堆栈

#### Scenario: Authentication rejection is not error level
- **Given** 登录密码不匹配、用户状态不允许登录、refresh token 无效或会话缺失
- **When** 用户服务记录该拒绝
- **Then** 日志 MUST NOT 使用 error 级别
- **Then** 日志 MUST 使用 warn 级别或在已有边界日志足够时不重复记录
- **Then** 日志 MUST NOT 泄露密码、token 或 Authorization header

#### Scenario: Domain conflict is not system failure
- **Given** 创建用户时用户名唯一性冲突
- **When** 用户服务返回冲突响应
- **Then** 日志 MUST NOT 使用 error 级别
- **Then** 日志 MAY 使用 warn 级别记录 `username` 和冲突上下文

### Requirement: Sensitive values are excluded from logs
用户服务日志 MUST NOT 记录密码、新密码、password hash、access token、refresh token、password-change token、完整 Authorization header、Redis session JSON payload、数据库 DSN 或数据库密码。需要定位问题时 MUST 使用用户 UUID、用户名、session id、token version、错误分类和是否存在凭证等安全上下文替代。

#### Scenario: Login failure does not leak credentials
- **Given** 登录请求包含 username 和 password
- **When** 登录认证失败或密码校验失败
- **Then** 日志 MAY 包含 username 和失败原因分类
- **Then** 日志 MUST NOT 包含 password、password hash、token 或 Authorization header

#### Scenario: Token refresh failure does not leak token
- **Given** Refresh 请求包含 refresh token
- **When** refresh token 解析失败、会话不存在或 token version 不匹配
- **Then** 日志 MUST NOT 包含 refresh token 原文
- **Then** 日志 MAY 包含解析失败分类、session id、user_id 或 token version 这类安全上下文

#### Scenario: Password change failure does not leak new password
- **Given** 修改密码请求包含改密 token 和新密码
- **When** token 校验、密码哈希或凭证更新失败
- **Then** 日志 MUST NOT 包含改密 token、新密码或 password hash
- **Then** 日志 MAY 包含 user_id、错误分类和方法名

### Requirement: Critical user and session flows have observability coverage
用户服务 MUST 覆盖用户创建、用户查询、用户列表、登录、强制改密、refresh token、logout、logout all 和认证会话仓储的关键成功入口与异常分支日志。日志 MUST 便于按 `trace-id`、`user_id`、`username` 和 `session_id` 检索。

#### Scenario: User creation logs important branches
- **Given** 用户创建请求通过校验
- **When** 创建流程开始、发生唯一冲突、密码哈希失败、UUID 生成失败或持久化失败
- **Then** 用户服务 MUST 记录对应流程或异常日志
- **Then** 日志 MUST 包含 `username` 和 `status`
- **Then** 系统错误日志 MUST 包含堆栈

#### Scenario: Authentication session repository logs Redis failures
- **Given** 认证会话仓储执行 token version 读取、会话创建、会话读取、会话删除或缓存失效
- **When** Redis 操作返回非 miss 的系统错误
- **Then** 日志 MUST 包含 Redis 操作类型、user_id 或 session_id 和底层错误
- **Then** 日志 MUST NOT 包含 Redis session JSON payload 或 token 原文

#### Scenario: Logout all logs session invalidation failures
- **Given** 用户请求退出全部设备
- **When** token version 增加、token version 缓存失效或用户会话删除失败
- **Then** 用户服务 MUST 记录可检索的异常日志
- **Then** 日志 MUST 包含 `user_id` 和错误字段

### Requirement: Logs remain compatible with API behavior
日志增强 MUST NOT 改变 HTTP API 行为。新增或调整日志后，用户服务 MUST 保持现有路由、请求参数、响应信封、错误码、HTTP 状态码、数据库 schema、Redis key 和 token claims 兼容。

#### Scenario: Error response remains unchanged after logging
- **Given** 用户服务发生参数错误、认证失败、用户不存在、冲突或内部错误
- **When** 新日志逻辑记录该事件
- **Then** HTTP 响应 MUST 继续使用 `common/response.Envelope`
- **Then** 响应错误码和 HTTP 状态码 MUST 与日志增强前保持兼容
- **Then** 日志写入失败或 logger 缺失 MUST NOT 改变业务响应
