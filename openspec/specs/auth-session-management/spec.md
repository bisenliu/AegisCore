## Purpose

定义 user-service 的认证会话能力，覆盖登录、用途隔离 token、refresh 轮换、token version 主事实、退出、改密、强制改密、HTTP 契约和认证资源生命周期。

## Requirements

### Requirement: 用户登录与用途隔离令牌

系统 MUST 在凭证、用户状态和会话策略校验通过后签发用途隔离的 access、refresh 或 password-change token。所有 token MUST 包含标准 `jti`，并 MUST 通过 `access`、`refresh` 和 `password_change` subject 限定使用流程；任一用途、subject 或必要 claims 不匹配时 MUST 拒绝 token。

#### Scenario: 普通登录成功

- **WHEN** 用户提供合法用户名和正确密码，且状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session 并签发 access token 与 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=false`
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodeOK` 和 `success=true`
- **AND** data MUST 包含 access token、refresh token、token type 和 access token 过期秒数，MUST NOT 包含登录状态枚举字段

#### Scenario: 凭证、状态和侧信道防护

- **WHEN** 用户名不存在、密码不匹配，或用户状态不允许登录且不属于强制改密状态
- **THEN** 系统 MUST 拒绝签发任何 token 和创建会话
- **AND** 公开错误 MUST NOT 泄露用户是否存在、密码匹配结果或具体用户状态
- **AND** 用户名不存在时系统 MUST 使用当前 bcrypt dummy hash 执行 dummy password verification
- **AND** 旧 Argon2id、未知算法或格式非法的存储哈希 MUST 被视为无效凭据，MUST NOT 触发旧哈希验证、迁移、fallback 或 rehash

#### Scenario: 强制改密登录

- **WHEN** 凭据有效且用户状态要求强制修改密码
- **THEN** 登录 use case MUST 返回 `PasswordChangeRequired=true` 和 subject 为 `password_change` 的受限 token，而不是返回业务错误
- **AND** 系统 MUST 创建与该 token 绑定的一次性 password-change session，MUST NOT 创建普通 refresh session 或签发 refresh token
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodePasswordChangeRequired`、code `20006` 和 `success=false`
- **AND** data MUST 只包含受限 access token、token type 和过期秒数，MUST NOT 包含 refresh token、`status`、`authenticated` 或 `password_change_required` 枚举字段

### Requirement: Refresh 会话、轮换与数量限制

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

### Requirement: Token version 权威来源与缓存投影

系统 MUST 以 PostgreSQL 当前 `token_version` 为主事实，并通过有界本地 loading cache 和 Redis 投影加速受保护访问校验。校验链路 MUST 在本地缓存或 Redis 未命中时回源 PostgreSQL，MUST NOT 缓存错误结果，且缓存关闭、TTL 过期或容量驱逐 MUST 只影响性能、不得改变认证和撤销语义。

#### Scenario: 受保护访问校验

- **WHEN** access token 已通过签名、subject 和过期校验
- **THEN** 系统 MUST 按有界本地缓存、Redis token version 投影和 PostgreSQL 当前值的顺序解析当前版本
- **AND** token claims 版本与当前版本不一致时 MUST 拒绝访问
- **AND** Redis miss 后系统 MAY 回源 PostgreSQL 并回填 Redis 和本地缓存

#### Scenario: 本地缓存启用或关闭

- **WHEN** `auth.token_version_cache.enabled` 为 true
- **THEN** 对应 feature cache 实例的 `size`、`ttl` 和 `load_timeout` MUST 为正值，`size` MUST 映射为最大 item 数
- **AND** token version string key MUST 由 auth feature 直接提供，MUST NOT 配置 common key string encoder
- **AND** `int64` token version MUST 作为不可变值直接缓存，不得为其配置 common clone callback
- **AND** 容量驱逐或 TTL 过期后 MUST 可通过同 key 回源合并恢复校验，不得依赖 admission rejected、set dropped 或异步写入可见性
- **AND** user-service 装配时缺少具名 `auth_token_version` cache 配置 MUST 明确报错并拒绝装配
- **WHEN** `auth.token_version_cache.enabled` 为 false
- **THEN** 系统 MUST 保持 token version 校验和撤销语义正确，且 MAY 产生额外 datastore 查询
- **AND** direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次回源结果

#### Scenario: Redis 投影和刷新

- **WHEN** 系统写入 Redis token version 投影
- **THEN** 正数 `auth.token_version_cache_ttl` MUST 作为显式 TTL 使用，零值或负值 MUST 使用服务默认 TTL，MUST NOT 创建永久投影
- **WHEN** 全部会话退出或密码变更导致 PostgreSQL `token_version` 增加
- **THEN** 系统 MUST 失效本实例本地缓存并刷新 Redis 投影
- **AND** 旧版本 MUST NOT 覆盖 Redis 中较新版本
- **AND** Redis 刷新失败时系统 MUST 尝试删除投影，使后续校验回源 PostgreSQL
- **AND** 本地失效或 Redis 投影失败 MUST 被记录并作为可观察错误返回，MUST NOT 被静默忽略

### Requirement: 凭据变更与会话撤销

系统 MUST 支持退出当前会话、退出全部会话和已认证密码变更。退出当前会话 MUST 只撤销目标 refresh session；退出全部会话和密码变更 MUST 递增 PostgreSQL token version，使旧 access token 失效，并撤销受影响 refresh sessions。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前 refresh session，MUST NOT 递增用户 `token_version`

#### Scenario: 全部会话撤销

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增 PostgreSQL `token_version` 并撤销该用户全部活跃 refresh sessions
- **AND** 旧 access token MUST 因版本不匹配而无法访问，旧 refresh token MUST 无法刷新
- **AND** 安全失效 MUST NOT 依赖后台 workerpool；Redis key 的物理清理 MAY 异步执行

#### Scenario: 已认证密码变更

- **WHEN** 已认证用户提供正确旧密码和满足策略的新密码
- **THEN** 系统 MUST 原子更新密码哈希并递增 PostgreSQL `token_version`
- **AND** 系统 MUST 执行本地缓存失效、Redis 投影刷新和 refresh session 撤销
- **WHEN** 旧密码错误或新密码不满足策略
- **THEN** 系统 MUST 拒绝修改，并保持密码、状态和 `token_version` 不变

#### Scenario: 撤销投影不完整

- **WHEN** PostgreSQL token version 主事实已更新，但本地 cache 失效、Redis 投影刷新或 refresh session 删除失败
- **THEN** use case MUST 返回 `authdomain.ErrSessionRevocationIncomplete`，MUST NOT 返回普通成功结果
- **AND** 错误链 MUST 保留底层原因，metrics MUST 将结果记录为撤销不完整失败

#### Scenario: 重复退出

- **WHEN** 用户重复退出已撤销或不存在的会话
- **THEN** 系统 MUST 返回稳定结果或明确错误，并 MUST NOT 恢复会话

### Requirement: 强制改密一次性流程

系统 MUST 为强制改密 token 创建服务端一次性 password-change session，并在更新密码前原子消费该 session。token 与 session MUST 使用独立短 TTL，并绑定 `jti`、`session_id`、`user_id` 和 `token_version`；MUST NOT 复用 refresh session 的 TTL、存储语义或上限裁剪策略。

#### Scenario: 创建一次性会话

- **WHEN** 强制改密登录签发 password-change token
- **THEN** 系统 MUST 创建与 token claims 完全一致的 Redis password-change session
- **AND** token 和 session MUST 使用 `auth.jwt.password_change_token_ttl`
- **AND** 该配置未设置或非正数时 MUST 使用 5 分钟默认 TTL，MUST NOT 创建无过期时间的 token 或 session
- **AND** session 创建失败时登录 MUST 失败，已签发 token MUST NOT 返回客户端

#### Scenario: 原子消费和并发保护

- **WHEN** token 有效、未过期且 Redis session 的 `jti`、`session_id`、`user_id` 和 `token_version` 全部匹配
- **THEN** 系统 MUST 原子删除一次性 session，并 MAY 继续执行凭据条件更新
- **WHEN** 多个请求并发使用同一个有效 password-change token
- **THEN** 系统 MUST 最多允许一个请求成功消费 session、更新密码并递增一次 `token_version`
- **AND** 其他请求 MUST 返回统一无效凭据

#### Scenario: 一次性凭据无效

- **WHEN** token 或 session 过期、不存在、已撤销、已消费，或任一绑定 claims 不一致
- **THEN** 系统 MUST 返回统一无效凭据错误
- **AND** 系统 MUST NOT 泄露具体失败原因，也 MUST NOT 更新密码、状态或 `token_version`

#### Scenario: 条件更新和撤销

- **WHEN** session 已消费，用户仍为 `UserStatusMustChangePassword` 且 PostgreSQL 当前 `token_version` 等于 token 中旧版本
- **THEN** 系统 MUST 更新密码哈希、将状态改为 `UserStatusNormal` 并递增 `token_version`
- **AND** 状态或版本不匹配时系统 MUST 返回统一无效凭据，并 MUST NOT 更新密码、状态或版本
- **AND** 凭据更新成功后系统 MUST 失效本地 token version cache、刷新 Redis 投影并删除该用户 refresh sessions
- **AND** 任一步失败 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通改密成功结果

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

#### Scenario: 认证失败和临时不可用响应

- **WHEN** 凭据无效、用户状态拒绝、缺失认证会话、token 无效、refresh session 无效或 password-change session 无效
- **THEN** 系统 MUST 返回稳定的 `401 Unauthorized` 认证错误，并保持统一公开文案或 token invalid 文案
- **WHEN** 认证流程返回 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 系统 MUST 返回 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 MUST 使用对应稳定公开消息，MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本

#### Scenario: 错误分类与统一出口

- **WHEN** auth controller 收到 use case 错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** 认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`
- **AND** 直接或包装后的错误 MUST 保持 `errors.Is` 或稳定 `Reason` 分类能力，供登录、refresh 和 logout metrics 使用

### Requirement: 认证架构、配置与资源生命周期

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject 常量、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语。认证 application、adapter、controller 和 composition MUST 通过 framework-neutral constructor、消费侧最小 port 和窄 settings 表达依赖，并显式管理 auth 自有后台资源。默认 JWT issuer、auth Redis key prefix 和认证相关配置示例 MUST 使用 `aegiscore-user-service`，旧 `aegiscore-user-services` issuer 或 Redis key prefix MUST NOT 被兼容接受、读取或双写。

#### Scenario: 服务私有签发、配置和分层边界

- **WHEN** user-service 签发 token、装配认证流程或新增凭据、token、session、token version 行为
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、refresh rotation、token version cache TTL 和每用户活跃 session 上限
- **AND** 系统 MUST NOT 从服务私有配置读取 password KDF、Argon2 或 bcrypt cost 预算
- **AND** production-like 环境中的 `auth.jwt.secret` MUST 至少为 32 bytes，校验错误 MUST 明确定位该配置项
- **AND** 默认 `auth.jwt.issuer` MUST 为 `aegiscore-user-service`
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务策略
- **AND** 业务编排 MUST 位于 auth application 或 domain，Redis 和 PostgreSQL adapter MUST 只实现消费侧最小存储 port

#### Scenario: framework-neutral 构造和缺失依赖错误

- **WHEN** 构造 auth use case、PostgreSQL credential store、Redis session store、HTTP controller 或本地 cache/invalidator
- **THEN** constructor MUST 只接收职责所需的普通 Go collaborator 和窄 settings
- **AND** 生产 constructor 输入 MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag 或服务级 named resource metadata
- **AND** 必需安全 collaborator 或无效窄 settings 缺失时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic、静默降级或提供 no-op 安全替身

#### Scenario: auth 自有资源启动、停止和回滚

- **WHEN** auth session purge pool、token-version 本地缓存或其他主动资源被启用
- **THEN** composition MUST 显式创建、启动和关闭这些 auth 自有资源
- **AND** 服务停止或启动失败时 MUST 停止已启动的 purge pool 并关闭已创建的本地缓存
- **AND** constructor 或启动阶段部分失败时 MUST 立即清理已创建且归 auth 拥有的资源，并保留原始失败和清理失败
- **AND** 停止和关闭 MUST 幂等、受 context/deadline 约束，disabled 或 direct 模式 MUST 提供一致 no-op close 语义

#### Scenario: 共享资源所有权和 fail-closed

- **WHEN** auth 资源停止、关闭或尚未启动
- **THEN** auth 组件 MUST NOT 关闭共享 Redis client、Redis token version 投影存储或 PostgreSQL 用户存储
- **AND** auth 自有资源 MUST 先于共享 Redis client 关闭
- **AND** 受保护访问和会话撤销流程 MUST 返回明确错误或保持 fail-closed
- **AND** 系统 MUST NOT 因 holder 中资源为空而允许旧 token、无效 refresh session 或撤销不完整结果通过

#### Scenario: 显式日志依赖

- **WHEN** auth application 或关键 Redis/PostgreSQL infrastructure 记录正式业务日志
- **THEN** logger MUST 由 constructor 显式注入或从 request context 获取，MUST NOT 依赖可变 package-level 默认 logger
- **AND** 撤销失败日志 MUST 保留可用的 `user_id`、错误分类、`request_id`、`trace_id` 和 `span_id`
- **AND** 日志 MUST NOT 暴露 token、jti、session ID、Redis key、SQL、密码或敏感原始错误

#### Scenario: auth Redis key prefix

- **WHEN** auth Redis adapter 生成 refresh session、password-change session、token version projection 或 session purge key
- **THEN** key prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`
- **AND** adapter MUST NOT 查询、删除、迁移或双写旧 `aegiscore-user-services` prefix 下的 key
- **AND** 发布后旧 prefix 下的 token version projection 或 session 数据 MUST 不再影响认证结果
