## Purpose

定义 user-service 的认证会话能力，覆盖登录、令牌签发、刷新、退出、改密、会话状态和 token version 校验。

## Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。

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

#### Scenario: token 缺少 jti

- **WHEN** access token、refresh token 或 password change token 缺少标准 `jti`
- **THEN** token MUST 被拒绝

#### Scenario: token subject 不匹配

- **WHEN** subject 为 `access`、`refresh` 或 `password_change` 的 token 被用于不匹配的认证流程
- **THEN** 系统 MUST 拒绝该 token，且 MUST NOT 在三类 token 之间兼容复用

### Requirement: 令牌刷新

系统 MUST 支持使用有效 refresh token 换取新的访问令牌，并校验会话状态、token version 和过期时间。

#### Scenario: 刷新成功

- **WHEN** 调用方提交有效且未过期的 refresh token
- **THEN** 系统 MUST 验证对应会话仍有效，并签发新的 access token

#### Scenario: refresh token 已撤销

- **WHEN** refresh token 对应会话已退出、被撤销或不存在
- **THEN** 系统 MUST 拒绝刷新并返回认证失败

#### Scenario: token version 不匹配

- **WHEN** token 中携带的 token version 与当前用户凭证版本不一致
- **THEN** 系统 MUST 拒绝刷新或受保护访问

#### Scenario: refresh session 与 token claims 一致性

- **WHEN** refresh token 已通过 JWT 解析
- **THEN** 系统 MUST 校验 Redis refresh session 存在，且 session 中的 `user_id`、`session_id`、`token_version` MUST 与 token claims 一致；任一不一致 MUST 拒绝续期

#### Scenario: refresh rotation 原子性

- **WHEN** refresh token rotation 已启用，且新 token 已签发但 Redis session 原子替换失败
- **THEN** 系统 MUST NOT 向客户端返回已签发的新 token，并 MUST 按无效 token 或会话错误处理

### Requirement: 会话与 token version 策略

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。

#### Scenario: 活跃 session 上限

- **WHEN** 用户超过配置的活跃 refresh session 上限
- **THEN** Redis 中最旧的活跃会话 MUST 作为安全敏感操作的一部分被同步裁剪

#### Scenario: token version 校验链路

- **WHEN** access token 已通过 JWT 解析且未过期
- **THEN** 受保护路由 MUST 按本地短 TTL 缓存、Redis token version 投影、PostgreSQL 当前值回源的顺序解析当前版本；Redis miss 后 MAY 回源数据库并回填 Redis，但 MUST NOT 缓存错误结果

#### Scenario: token version 投影刷新

- **WHEN** 用户执行全部会话退出或强制改密导致当前 `token_version` 变化
- **THEN** 系统 MUST 使本实例本地 token version 缓存失效，并刷新 Redis token version 投影；旧版本 MUST NOT 覆盖 Redis 中已存在的较新版本

### Requirement: 会话退出

系统 MUST 支持退出当前会话和退出全部会话，并保证退出后令牌无法继续访问受保护资源。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前 refresh session，且 MUST NOT 递增用户 `token_version`

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增用户 `token_version` 并撤销该用户的所有活跃 refresh session，使旧 token 无法继续刷新或访问

#### Scenario: 全部会话后台清理

- **WHEN** 用户执行全部会话退出
- **THEN** 安全失效 MUST NOT 依赖后台 workerpool；Redis refresh session key 的批量物理删除 MAY 通过 auth 专用 purge workerpool 异步执行

#### Scenario: 重复退出

- **WHEN** 用户对已撤销或不存在的会话重复执行退出操作
- **THEN** 系统 MUST 返回稳定结果或明确错误，并 MUST NOT 恢复已撤销会话

### Requirement: 密码变更

系统 MUST 支持已认证用户修改密码，并在密码变更后更新凭证和 token version 以失效旧令牌。

#### Scenario: 修改密码成功

- **WHEN** 已认证用户提供正确旧密码和满足策略的新密码
- **THEN** 系统 MUST 更新密码哈希、提升 token version，并使旧令牌失效

#### Scenario: 旧密码错误

- **WHEN** 用户修改密码时提供的旧密码不正确
- **THEN** 系统 MUST 拒绝修改并保持原密码和 token version 不变

#### Scenario: 新密码不合规

- **WHEN** 新密码不满足密码策略
- **THEN** 系统 MUST 拒绝修改并返回校验错误

### Requirement: 认证 HTTP 边界

系统 MUST 将公开认证路由和受保护认证路由分开挂载，并通过共享认证中间件保护需要 bearer token 的接口。

#### Scenario: 公开登录路由

- **WHEN** 调用方访问登录或刷新等公开认证入口
- **THEN** 系统 MUST 允许请求进入认证 controller 并在业务层完成凭证校验

#### Scenario: 受保护认证路由

- **WHEN** 调用方访问退出、修改密码或其他受保护认证入口
- **THEN** 系统 MUST 先通过 JWT、auth config 和 token version validator 校验

#### Scenario: 无效 bearer token

- **WHEN** 受保护认证路由收到缺失、过期、格式错误或签名无效的 bearer token
- **THEN** 系统 MUST 在进入业务处理前拒绝请求

### Requirement: 认证包组织

系统 MUST 将认证 application 职责按 `command`、`authctx`、`credentials`、`tokens`、`sessions`、`validators` 和 `ports.go` 组织，避免 transport 或 Redis adapter 承载认证业务编排。

#### Scenario: 新增凭据行为

- **WHEN** 新行为需要校验或更新密码凭据
- **THEN** 代码 MUST 位于 `application/credentials` 或 command 编排中，不得放入 HTTP transport 或 Redis adapter

#### Scenario: 新增 token 或会话行为

- **WHEN** 新行为涉及 JWT 签发解析、refresh session 生命周期、token version fallback 或会话撤销
- **THEN** 业务语义 MUST 位于 `application/tokens`、`application/sessions`、`application/validators` 或 command 编排中，Redis adapter 只实现存储契约
