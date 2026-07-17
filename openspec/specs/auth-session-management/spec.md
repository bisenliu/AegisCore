## Purpose

定义 user-service 的认证会话能力，覆盖登录、令牌签发与刷新、退出、改密、会话状态和 token version 校验。

## Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 在凭证、用户状态和会话策略校验通过后签发用途隔离的 access、refresh 或 password-change token。所有 token MUST 包含标准 `jti`，并 MUST 通过 `access`、`refresh` 和 `password_change` subject 限定使用流程；任一用途、subject 或必要 claims 不匹配时 MUST 拒绝 token。

#### Scenario: 普通登录成功

- **WHEN** 用户提供合法用户名和正确密码，且状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session 并签发 access token 与 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=false`
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodeOK` 和 `success=true`
- **AND** data MUST 包含 access token、refresh token、token type 和 access token 过期秒数，MUST NOT 包含登录状态枚举字段

#### Scenario: 凭证或用户状态无效

- **WHEN** 用户名不存在、密码不匹配，或用户状态不允许登录且不属于强制改密状态
- **THEN** 系统 MUST 拒绝签发任何 token 和创建会话
- **AND** 公开错误 MUST NOT 泄露用户是否存在、密码匹配结果或具体用户状态

#### Scenario: 未知用户的侧信道防护

- **WHEN** 登录用户名不存在
- **THEN** 系统 MUST 使用当前密码 KDF 参数执行 dummy password verification
- **AND** 日志、错误和响应 MUST NOT 泄露用户名是否存在

#### Scenario: 密码 KDF 资源繁忙

- **WHEN** Argon2 执行和等待队列达到资源上限，或 dummy verification 返回 `password.ErrPasswordKDFBusy`
- **THEN** 系统 MUST 将该结果分类为临时服务不可用并返回 `503 Service Unavailable`
- **AND** 系统 MUST NOT 将其折叠为无效凭据，也 MUST NOT 泄露队列长度、KDF 配置或凭证匹配状态

#### Scenario: 强制改密登录

- **WHEN** 凭据有效且用户状态要求强制修改密码
- **THEN** 登录 use case MUST 返回 `PasswordChangeRequired=true` 和 subject 为 `password_change` 的受限 token，而不是返回业务错误
- **AND** 系统 MUST 创建与该 token 绑定的一次性 password-change session，MUST NOT 创建普通 refresh session 或签发 refresh token
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodePasswordChangeRequired`、code `20006` 和 `success=false`
- **AND** data MUST 只包含受限 access token、token type 和过期秒数，MUST NOT 包含 refresh token、`status`、`authenticated` 或 `password_change_required` 枚举字段

### Requirement: Refresh 会话与令牌轮换

系统 MUST 使用有效 refresh token 和服务端 refresh session 换取新的 access token，并 MUST 校验 token 过期时间、subject、token version 及 session claims 一致性。系统 MUST 对每用户活跃 refresh session 数量实施配置上限，并在启用 rotation 时原子替换 session。

#### Scenario: 刷新成功

- **WHEN** 调用方提交有效且未过期的 refresh token，其会话仍存在
- **THEN** 系统 MUST 校验 session 中的 `user_id`、`session_id` 和 `token_version` 与 token claims 完全一致
- **AND** 当前 PostgreSQL token version MUST 与 token claims 一致
- **AND** 系统 MUST 签发新的 access token

#### Scenario: 无效 refresh 会话

- **WHEN** refresh session 不存在、已过期、已撤销，或 session claims 与 token 不一致
- **THEN** 系统 MUST 拒绝刷新并返回统一 token invalid 响应
- **AND** 系统 MUST NOT 泄露 session 不存在或 claims 不匹配的内部细节

#### Scenario: Refresh rotation 原子性

- **WHEN** refresh token rotation 已启用
- **THEN** 系统 MUST 原子创建新 session 并替换旧 session
- **AND** 新 token 已签发但 session 原子替换失败时，系统 MUST NOT 向客户端返回新 token

#### Scenario: 活跃 session 上限

- **WHEN** 新会话使用户超过配置的活跃 refresh session 上限
- **THEN** 系统 MUST 同步裁剪 Redis 中最旧的活跃会话
- **AND** 安全裁剪 MUST NOT 依赖后台 workerpool 完成

### Requirement: Token version 校验与投影

系统 MUST 以 PostgreSQL 当前 `token_version` 为主事实，并通过有界本地缓存和 Redis 投影加速受保护访问校验。校验链路 MUST 在本地缓存或 Redis 未命中时回源 PostgreSQL，MUST NOT 缓存错误结果，且缓存关闭或淘汰 MUST 只影响性能、不得改变认证和撤销语义。

#### Scenario: 受保护访问校验

- **WHEN** access token 已通过签名、subject 和过期校验
- **THEN** 系统 MUST 按有界本地缓存、Redis token version 投影和 PostgreSQL 当前值的顺序解析当前版本
- **AND** token claims 版本与当前版本不一致时 MUST 拒绝访问
- **AND** Redis miss 后系统 MAY 回源 PostgreSQL 并回填 Redis 和本地缓存

#### Scenario: 本地缓存配置

- **WHEN** `auth.token_version_cache.enabled` 为 true
- **THEN** 对应 feature cache 实例的 `size`、`ttl` 和 `load_timeout` MUST 为正值，且容量淘汰、准入拒绝或 TTL 过期后 MUST 可回源恢复校验
- **AND** user-service 装配时缺少具名 `auth_token_version` cache 配置 MUST 明确报错并拒绝装配

#### Scenario: 关闭本地缓存

- **WHEN** `auth.token_version_cache.enabled` 为 false
- **THEN** 系统 MUST 保持 token version 校验和撤销语义正确
- **AND** `size`、`ttl` 和 `load_timeout` MAY 为零，系统 MAY 产生额外 datastore 查询

#### Scenario: Redis 投影 TTL

- **WHEN** 系统写入 Redis token version 投影
- **THEN** 正数 `auth.token_version_cache_ttl` MUST 作为显式 TTL 使用
- **AND** 零值或负值 MUST 使用服务默认 TTL，MUST NOT 创建永久投影

#### Scenario: 投影刷新与本地失效

- **WHEN** 全部会话退出或密码变更导致 PostgreSQL `token_version` 增加
- **THEN** 系统 MUST 失效本实例本地缓存并刷新 Redis 投影
- **AND** 旧版本 MUST NOT 覆盖 Redis 中较新版本
- **AND** Redis 刷新失败时系统 MUST 尝试删除投影，使后续校验回源 PostgreSQL
- **AND** 本地失效或 Redis 投影失败 MUST 被记录并作为可观察错误返回，MUST NOT 被静默忽略

### Requirement: 会话退出与安全撤销

系统 MUST 支持退出当前会话和退出全部会话。退出当前会话 MUST 只撤销目标 refresh session；退出全部会话 MUST 以 PostgreSQL token version 递增使旧 access token 失效，并撤销全部 refresh sessions。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前 refresh session，MUST NOT 递增用户 `token_version`

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增 PostgreSQL `token_version` 并撤销该用户全部活跃 refresh sessions
- **AND** 旧 access token MUST 因版本不匹配而无法访问，旧 refresh token MUST 无法刷新
- **AND** 安全失效 MUST NOT 依赖后台 workerpool；Redis key 的物理清理 MAY 异步执行

#### Scenario: 撤销投影不完整

- **WHEN** PostgreSQL token version 主事实已更新，但本地 cache 失效、Redis 投影刷新或 refresh session 删除失败
- **THEN** use case MUST 返回 `authdomain.ErrSessionRevocationIncomplete`，MUST NOT 返回普通成功结果
- **AND** 错误链 MUST 保留底层原因，metrics MUST 将结果记录为撤销不完整失败

#### Scenario: 重复退出

- **WHEN** 用户重复退出已撤销或不存在的会话
- **THEN** 系统 MUST 返回稳定结果或明确错误，并 MUST NOT 恢复会话

### Requirement: 已认证密码变更

系统 MUST 允许已认证用户在验证旧密码和新密码策略后更新凭证，并 MUST 递增 token version 使旧 token 失效。

#### Scenario: 修改密码成功

- **WHEN** 已认证用户提供正确旧密码和满足策略的新密码
- **THEN** 系统 MUST 原子更新密码哈希并递增 PostgreSQL `token_version`
- **AND** 系统 MUST 执行本地缓存失效、Redis 投影刷新和 refresh session 撤销

#### Scenario: 旧密码错误

- **WHEN** 用户提供的旧密码不正确
- **THEN** 系统 MUST 拒绝修改，并保持密码、状态和 `token_version` 不变

#### Scenario: 新密码不合规

- **WHEN** 新密码不满足密码策略
- **THEN** 系统 MUST 拒绝修改并返回校验错误，MUST NOT 写入部分凭证状态

### Requirement: 强制改密一次性流程

系统 MUST 为强制改密 token 创建服务端一次性 password-change session，并在更新密码前原子消费该 session。token 与 session MUST 使用独立短 TTL，并绑定 `jti`、`session_id`、`user_id` 和 `token_version`；MUST NOT 复用 refresh session 的 TTL、存储语义或上限裁剪策略。

#### Scenario: 创建一次性会话

- **WHEN** 强制改密登录签发 password-change token
- **THEN** 系统 MUST 创建与 token claims 完全一致的 Redis password-change session
- **AND** token 和 session MUST 使用 `auth.jwt.password_change_token_ttl`
- **AND** 该配置未设置或非正数时 MUST 使用 5 分钟默认 TTL，MUST NOT 创建无过期时间的 token 或 session
- **AND** session 创建失败时登录 MUST 失败，已签发 token MUST NOT 返回客户端

#### Scenario: 原子消费成功

- **WHEN** token 有效、未过期且 Redis session 的 `jti`、`session_id`、`user_id` 和 `token_version` 全部匹配
- **THEN** 系统 MUST 原子删除一次性 session，并 MAY 继续执行凭据条件更新

#### Scenario: 一次性凭据无效

- **WHEN** token 或 session 过期、不存在、已撤销、已消费，或任一绑定 claims 不一致
- **THEN** 系统 MUST 返回统一无效凭据错误
- **AND** 系统 MUST NOT 泄露具体失败原因，也 MUST NOT 更新密码、状态或 `token_version`

#### Scenario: 并发消费

- **WHEN** 多个请求并发使用同一个有效 password-change token
- **THEN** 系统 MUST 最多允许一个请求成功消费 session、更新密码并递增一次 `token_version`
- **AND** 其他请求 MUST 返回统一无效凭据

#### Scenario: 条件更新凭据

- **WHEN** session 已消费，用户仍为 `UserStatusMustChangePassword` 且 PostgreSQL 当前 `token_version` 等于 token 中旧版本
- **THEN** 系统 MUST 更新密码哈希、将状态改为 `UserStatusNormal` 并递增 `token_version`
- **AND** 状态或版本不匹配时系统 MUST 返回统一无效凭据，并 MUST NOT 更新密码、状态或版本

#### Scenario: 改密后撤销

- **WHEN** 强制改密凭据更新成功
- **THEN** 系统 MUST 失效本地 token version cache、刷新 Redis 投影并删除该用户 refresh sessions
- **AND** 任一步失败 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通改密成功结果
- **AND** 旧 access token 和 refresh token MUST 无法继续访问或刷新

### Requirement: 认证 HTTP 与错误契约

系统 MUST 仅通过 `/api/v1/auth` 暴露认证入口，将登录、refresh 和强制改密作为公开认证路由，将退出当前会话、退出全部会话和普通改密作为 access token 保护路由。HTTP transport MUST 使用共享 response helper 渲染业务错误，并 MUST NOT 维护认证专用 sentinel-to-HTTP mapper。

#### Scenario: 公开与受保护路由

- **WHEN** 调用方访问登录、refresh 或强制改密入口
- **THEN** 系统 MUST 允许请求进入 controller，并在业务层校验相应凭据或 token
- **AND** 退出和普通改密入口 MUST 在进入业务处理前校验 bearer token、user-service access claims 和 token version
- **AND** 系统 MUST NOT 暴露旧认证路径别名或认证绕过路径

#### Scenario: 共享 middleware 最小能力

- **WHEN** user-service 将 access token verifier 注入共享认证 middleware
- **THEN** 注入对象 MUST 只暴露 access token 验证能力
- **AND** 共享 middleware MUST NOT 获得 refresh token、password-change token 或任何 token 签发能力

#### Scenario: 认证失败响应

- **WHEN** 凭据无效或用户状态拒绝普通登录
- **THEN** 系统 MUST 返回 `401 Unauthorized` 和 `CodeUnauthenticated`，并使用统一无效凭据公开文案
- **AND** 缺失认证会话 MUST 返回 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** token、refresh session 或 password-change session 无效或不匹配 MUST 返回 `401 Unauthorized` 和 `CodeTokenInvalid`

#### Scenario: 临时服务不可用响应

- **WHEN** 认证流程返回 `password.ErrPasswordKDFBusy` 或 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 系统 MUST 返回 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 MUST 使用对应的稳定公开消息，MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本

#### Scenario: 错误分类与统一出口

- **WHEN** auth controller 收到 use case 错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** 认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`
- **AND** 无效凭据、用户状态拒绝和缺失会话的 `Reason` MUST 分别为 `invalid_credentials`、`user_status_rejected` 和 `missing_session`
- **AND** token 无效、refresh session 不存在或不匹配的 `Reason` MUST 分别为 `auth_token_invalid`、`auth_session_not_found` 和 `auth_session_mismatch`
- **AND** password-change session 不存在或不匹配的 `Reason` MUST 分别为 `password_change_session_not_found` 和 `password_change_session_mismatch`
- **AND** 撤销不完整和 KDF 繁忙的 `Reason` MUST 分别为 `session_revocation_incomplete` 和 `password_kdf_busy`
- **AND** 直接或包装后的错误 MUST 保持 `errors.Is` 或稳定 `Reason` 分类能力，供登录、refresh 和 logout metrics 使用

### Requirement: 认证能力边界与私有配置

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject 常量、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语，不得拥有 user-service token 签发入口或专属 claims。认证 application MUST 通过消费侧最小 port 和窄 settings 编排凭据、token、session 与版本行为，MUST NOT 依赖 HTTP transport DTO、完整运行时配置或 Fx 语义。

#### Scenario: 服务私有签发与配置

- **WHEN** user-service 签发 token 或装配认证流程
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、password KDF 预算、refresh rotation、token version cache TTL 和每用户活跃 session 上限
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务策略
- **AND** production-like 环境中的 `auth.jwt.secret` MUST 至少为 32 bytes，校验错误 MUST 明确定位该配置项

#### Scenario: Application 最小依赖

- **WHEN** 构造登录、refresh、改密或退出 use case
- **THEN** constructor MUST 只接收该 use case 所需的 collaborator 和窄 settings
- **AND** application command 包 MUST NOT 导入 `go.uber.org/fx`，也 MUST NOT 通过跨 use case 公共依赖容器暴露无关依赖
- **AND** refresh use case MUST 只接收 rotation 所需窄 settings，MUST NOT 接收完整 `*config.Config`

#### Scenario: 分层存储边界

- **WHEN** 新增凭据、token、session、token version 或撤销行为
- **THEN** 业务编排 MUST 位于 auth application 或 domain，Redis 和 PostgreSQL adapter MUST 只实现消费侧最小存储 port
- **AND** token version 持久化、Redis 投影、refresh session 和本地失效器 MUST 通过可独立依赖的接口表达

#### Scenario: 显式日志依赖

- **WHEN** auth application 或关键 Redis/PostgreSQL infrastructure 记录正式业务日志
- **THEN** logger MUST 由 constructor 显式注入或从 request context 获取，MUST NOT 依赖可变 package-level 默认 logger
- **AND** 撤销失败日志 MUST 保留可用的 `user_id`、错误分类、`request_id`、`trace_id` 和 `span_id`
- **AND** 日志 MUST NOT 暴露 token、jti、session ID、Redis key、SQL、密码或敏感原始错误
