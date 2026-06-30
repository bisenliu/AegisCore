## MODIFIED Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。系统 MUST 将密码 KDF 资源池繁忙视为临时服务不可用，而不是无效凭据。

#### Scenario: 登录成功

- **WHEN** 用户提供合法用户名和正确密码，且用户状态允许登录
- **THEN** 系统 MUST 创建会话、签发 access token 与 refresh token，并返回会话相关过期时间

#### Scenario: 凭证错误

- **WHEN** 用户名不存在或密码不匹配
- **THEN** 系统 MUST 拒绝登录并返回一致的认证错误，且 MUST NOT 泄露具体凭证匹配细节

#### Scenario: 用户状态禁止登录

- **WHEN** 用户存在但状态不允许登录
- **THEN** 系统 MUST 拒绝签发令牌并返回明确的状态相关错误

#### Scenario: 强制改密用户登录

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token，不得创建普通 refresh session，也不得返回 refresh token

#### Scenario: 密码 KDF 资源繁忙

- **WHEN** 登录凭据校验进入密码 KDF 但实例内 Argon2 执行和等待队列已达资源上限
- **THEN** 系统 MUST 拒绝本次登录并返回 `503 Service Unavailable`
- **AND** 系统 MUST NOT 将该错误映射为无效凭据
- **AND** 系统 MUST NOT 签发 access token、refresh token 或 password change token
- **AND** 系统 MUST NOT 泄露用户名存在性、密码匹配状态、队列长度或 Argon2 并发配置

#### Scenario: token 缺少 jti

- **WHEN** access token、refresh token 或 password change token 缺少标准 `jti`
- **THEN** token MUST 被拒绝

#### Scenario: token subject 不匹配

- **WHEN** subject 为 `access`、`refresh` 或 `password_change` 的 token 被用于不匹配的认证流程
- **THEN** 系统 MUST 拒绝该 token，且 MUST NOT 在三类 token 之间兼容复用

### Requirement: 认证 HTTP 边界

系统 MUST 将公开认证路由和受保护认证路由分开挂载，并通过共享认证中间件保护需要 bearer token 的接口。认证 HTTP 边界 MUST 区分凭据认证失败和认证服务临时不可用。

#### Scenario: 公开登录路由

- **WHEN** 调用方访问登录或刷新等公开认证入口
- **THEN** 系统 MUST 允许请求进入认证 controller 并在业务层完成凭证校验

#### Scenario: 受保护认证路由

- **WHEN** 调用方访问退出、修改密码或其他受保护认证入口
- **THEN** 系统 MUST 先通过 JWT、auth config 和 token version validator 校验

#### Scenario: 无效 bearer token

- **WHEN** 受保护认证路由收到缺失、过期、格式错误或签名无效的 bearer token
- **THEN** 系统 MUST 在进入业务处理前拒绝请求

#### Scenario: 登录 KDF busy HTTP 响应

- **WHEN** 登录 use case 返回 `password.ErrPasswordKDFBusy`
- **THEN** 认证 HTTP 边界 MUST 返回 `503 Service Unavailable`
- **AND** 响应 envelope MUST 使用服务不可用错误分类和认证服务繁忙消息
- **AND** OpenAPI MUST 声明登录接口可能返回 503
