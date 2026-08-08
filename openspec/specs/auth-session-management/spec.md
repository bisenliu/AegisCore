## Purpose

定义 user-service 的认证会话能力，覆盖登录、用途隔离 token、refresh 轮换、token version 主事实、会话撤销、强制改密、HTTP 契约和认证资源生命周期。
## Requirements
### Requirement: 登录、用途隔离令牌与 HTTP 边界

系统 MUST 在凭证、用户状态和会话策略校验通过后签发用途隔离的 access、refresh 或 password-change token。所有 token MUST 包含标准 `jti`，并通过 `access`、`refresh` 和 `password_change` subject 限定使用流程；任一用途、subject 或必要 claims 不匹配时 MUST 被拒绝。系统 MUST 仅通过 `/api/v1/auth` 暴露认证入口，执行请求体容量检查，并使用共享 response helper 渲染业务错误。登录审计客户端地址 MUST 来自 Gin trusted proxy 规则解析后的 `c.ClientIP()`。

#### Scenario: 普通登录成功

- **WHEN** 用户提供合法用户名和正确密码，且状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session 并签发 access token 与 refresh token
- **AND** 登录结果 MUST 为 `PasswordChangeRequired=false`
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodeOK` 和 `success=true`，data MUST 包含 access token、refresh token、token type 和 access token 过期秒数，MUST NOT 包含登录状态枚举字段

#### Scenario: 登录拒绝与侧信道防护

- **WHEN** 用户名不存在、密码不匹配，或用户状态不允许登录且不属于强制改密状态
- **THEN** 系统 MUST 拒绝签发任何 token 和创建会话，且公开错误 MUST NOT 泄露用户是否存在、密码匹配结果或具体用户状态
- **AND** 用户名不存在时 MUST 使用当前 bcrypt dummy hash 执行 dummy password verification
- **AND** 旧 Argon2id、未知算法或格式非法的存储哈希 MUST 被视为无效凭据，MUST NOT 触发旧哈希验证、迁移、fallback 或 rehash

#### Scenario: 强制改密登录响应

- **WHEN** 凭据有效且用户状态要求强制修改密码
- **THEN** 登录结果 MUST 为 `PasswordChangeRequired=true`，系统 MUST 创建一次性 password-change session 并只签发 subject 为 `password_change` 的受限 token，MUST NOT 创建普通 refresh session 或签发 refresh token
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodePasswordChangeRequired`、code `20006` 和 `success=false`
- **AND** data MUST 只包含受限 access token、token type 和过期秒数，MUST NOT 包含 refresh token、`status`、`authenticated` 或 `password_change_required` 枚举字段

#### Scenario: 路由保护与最小 verifier

- **WHEN** 调用方访问登录、refresh 或强制改密入口
- **THEN** controller MUST 允许请求进入并在业务层校验相应凭据或 token
- **AND** 强制改密入口 MUST 挂载为 `POST /api/v1/auth/force-change-password`，MUST 使用 `password_change` 受限 token 和一次性 password-change session 完成校验，MUST NOT 要求普通 access token middleware 先放行
- **AND** 退出当前会话和退出全部会话 MUST 在业务处理前校验 bearer token、user-service access claims 和 token version
- **AND** 系统 MUST NOT 暴露旧认证路径别名或认证绕过路径，包括 `POST /api/v1/auth/change-password`
- **AND** 共享认证 middleware 只 MUST 获得 access token 验证能力，MUST NOT 获得 refresh token、password-change token 或任何 token 签发能力

#### Scenario: 认证错误统一出口

- **WHEN** auth controller 收到凭据、用户状态、token 或 session 无效错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)` 返回稳定的 `401 Unauthorized` 认证错误和统一公开文案
- **WHEN** 认证流程返回 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 系统 MUST 返回 `503 Service Unavailable`、`CodeServiceUnavailable` 和稳定公开消息
- **AND** 认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`，直接或包装后 MUST 保持 `errors.Is` 或稳定 `Reason` 分类能力
- **AND** HTTP transport MUST NOT 维护认证专用 sentinel-to-HTTP mapper，响应 MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本

#### Scenario: 认证入口请求体容量边界

- **WHEN** 登录、refresh 或强制改密请求的固定长度、chunked 或含尾随 JSON 的总请求体超过配置上限
- **THEN** 系统 MUST 在密码校验、token 解析、session 消费、use case 和 token 签发前返回 `413 Payload Too Large`
- **AND** 认证、限流和字段校验错误 MUST 保持各自语义，不得互相伪装

#### Scenario: 登录审计客户端地址

- **WHEN** 登录请求来自显式信任的代理且代理提供已清洗的 forwarded headers
- **THEN** controller MUST 将 `c.ClientIP()` 解析的客户端地址写入认证审计上下文
- **WHEN** 登录请求来自未信任 TCP peer
- **THEN** 系统 MUST 使用 peer 地址并忽略其提供的 `X-Forwarded-For` 或 `X-Real-IP`
- **AND** controller、input preparer 与 auth application MUST NOT 手写解析 forwarded headers

### Requirement: Refresh 会话、轮换与数量限制

系统 MUST 使用有效 refresh token 和服务端 refresh session 换取新的 access token，校验 token 过期时间、subject、token version 及 session claims 一致性，并对每用户活跃 refresh session 数量实施配置上限；启用 rotation 时 MUST 原子替换 session。

#### Scenario: 刷新成功或拒绝

- **WHEN** 调用方提交有效且未过期的 refresh token，且 session 存在
- **THEN** 系统 MUST 校验 session 中的 `user_id`、`session_id` 和 `token_version` 与 token claims 完全一致，并校验 PostgreSQL 当前 token version 后签发新的 access token
- **WHEN** session 不存在、已过期、已撤销，或 session claims 与 token 不一致
- **THEN** 系统 MUST 拒绝刷新并返回统一 token invalid 响应，MUST NOT 泄露具体失败原因

#### Scenario: 原子轮换与容量限制

- **WHEN** refresh token rotation 已启用
- **THEN** 系统 MUST 原子创建新 session 并替换旧 session；新 token 已签发但替换失败时 MUST NOT 向客户端返回新 token
- **WHEN** 新会话使用户超过配置的活跃 refresh session 上限
- **THEN** 系统 MUST 同步裁剪 Redis 中最旧的活跃会话，MUST NOT 依赖后台 workerpool 完成安全裁剪

### Requirement: Token version 校验与会话撤销

系统 MUST 以 PostgreSQL 当前 `token_version` 为主事实，并通过有界本地 loading cache 和 Redis 投影加速校验。缓存未命中、过期、容量驱逐或显式禁用只能影响性能，MUST NOT 改变认证与撤销语义。退出全部会话和强制改密 MUST 递增主事实，使旧 access token 失效并撤销相应 refresh sessions。token-version feature cache MUST 由 user-service 私有配置提供完整默认值、启用时校验和到通用 localcache 配置的集中映射，auth 构造路径 MUST 只消费窄 auth settings。递增 `token_version` 或更新凭证并返回新 `token_version` MUST 是单一确定的 PostgreSQL 结果，成功路径 MUST NOT 在提交后通过第二条 `SELECT` 才获取撤销版本。

#### Scenario: 受保护访问与本地缓存

- **WHEN** access token 已通过签名、subject 和过期校验
- **THEN** 系统 MUST 按有界本地缓存、Redis 投影和 PostgreSQL 当前值的顺序解析 token version，版本不一致时 MUST 拒绝访问，Redis miss 后 MAY 回源并回填
- **AND** UUID MUST 在 validator 边界转换为规范 string key；`int64` token version MUST 直接缓存，MUST NOT 配置 common key encoder 或 clone callback
- **AND** 系统 MUST NOT 缓存错误结果；容量驱逐或 TTL 过期后 MUST 可通过同 key 合并回源恢复校验，不得依赖异步写入可见性
- **WHEN** `auth.token_version_cache` 未配置
- **THEN** user-service MUST 使用 `enabled=true`、`size=100000`、`ttl=1s` 和 `load_timeout=300ms` 的完整默认值
- **WHEN** `auth.token_version_cache.enabled` 为 true
- **THEN** 具名 `auth_token_version` cache 的 `size`、`ttl` 和 `load_timeout` MUST 为正值，`size` MUST 表示最大 item 数
- **WHEN** cache 被禁用
- **THEN** 系统 MUST 忽略 cache 的 `size`、`ttl` 和 `load_timeout`，不创建通用 loading cache，并保持校验和撤销正确
- **AND** direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次回源结果

#### Scenario: Redis 投影更新与本地失效顺序

- **WHEN** 系统写入 Redis token version 投影
- **THEN** 正数 `auth.token_version_cache_ttl` MUST 作为显式 TTL，零值或负值 MUST 使用服务默认 TTL，MUST NOT 创建永久投影
- **WHEN** PostgreSQL `token_version` 增加并进入撤销编排
- **THEN** 系统 MUST 在 Redis 投影更新前执行第一次本地 `Invalidate`，在更新后执行第二次本地 `Invalidate`，旧版本 MUST NOT 覆盖较新版本
- **AND** 删除 refresh sessions 后 MUST NOT 为旧 localcache 生命周期兼容执行第三次本地失效
- **AND** Redis 刷新失败时 MUST 尝试删除投影，使后续校验回源 PostgreSQL；投影失败 MUST 被记录并作为可观察错误返回

#### Scenario: Token version 并发失效保持 fail-closed

- **WHEN** token-version loader 在本地失效前开始且在失效后完成
- **THEN** 旧 token version MUST NOT 返回给 validator，也 MUST NOT 回填本地缓存
- **AND** localcache MUST 透明重试后只返回失效后的新版本；若重试再次与失效并发，validator MUST 对 `ErrInvalidated` fail-closed
- **WHEN** 撤销流程与旧 access token 校验并发
- **THEN** 撤销所需本地失效返回后，旧 loader MUST NOT 使旧 access token 获得认证成功

#### Scenario: 退出当前会话

- **WHEN** 已认证用户退出当前会话，或重复退出已撤销或不存在的会话
- **THEN** 系统 MUST 只撤销目标 refresh session，MUST NOT 递增 `token_version` 或恢复会话，并 MUST 返回稳定结果或明确错误

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增 PostgreSQL `token_version` 并在同一数据库结果中返回新版本，然后撤销全部活跃 refresh sessions，使旧 access 和 refresh token 无效
- **AND** 安全失效 MUST NOT 依赖后台 workerpool，Redis key 物理清理 MAY 异步执行

#### Scenario: 撤销版本原子返回

- **WHEN** 系统执行退出全部会话所需的 `token_version` 递增，或执行强制改密所需的密码哈希、状态和 `token_version` 条件更新
- **THEN** PostgreSQL mutation 与新 `token_version` 返回 MUST 来自单条 `UPDATE ... RETURNING token_version` 或等价事务内更新返回
- **AND** 成功更新路径 MUST NOT 在更新提交后执行第二条 `SELECT` 才获取新版本
- **AND** 故障注入 MUST 只能产生“未更新且返回失败”或“已更新、已拿到新版本并进入撤销编排”两种 application 可观察状态

#### Scenario: 条件凭证更新拒绝

- **WHEN** 强制改密条件更新时用户不存在
- **THEN** 系统 MUST 返回用户不存在错误，并保持密码、状态和 `token_version` 不变
- **WHEN** 用户状态或当前 `token_version` 与强制改密 token/session 绑定条件不匹配
- **THEN** 系统 MUST 返回统一无效凭据，并保持密码、状态和 `token_version` 不变

#### Scenario: 撤销投影不完整

- **WHEN** PostgreSQL 主事实已更新并已返回新 `token_version`，但 Redis 投影刷新或 refresh session 删除失败
- **THEN** use case MUST 返回 `authdomain.ErrSessionRevocationIncomplete`，MUST NOT 返回普通成功结果
- **AND** 错误链 MUST 保留底层原因，metrics MUST 将结果记录为撤销不完整失败

#### Scenario: Auth settings 依赖边界

- **WHEN** composition 构造 token issuer、session 策略、token-version localcache 或 validator
- **THEN** auth provider MUST 接收只包含 JWT、session 和 token-version cache 所需字段的 auth settings
- **AND** auth feature MUST NOT 依赖完整 user-service 根配置或读取 RBAC、Ent、resources 等无关配置段

### Requirement: 强制改密一次性流程

系统 MUST 为强制改密 token 创建服务端一次性 password-change session，并在更新密码前原子消费。token 与 session MUST 使用独立短 TTL，并绑定 `jti`、`session_id`、`user_id` 和 `token_version`；MUST NOT 复用 refresh session 的 TTL、存储语义或上限裁剪策略。RBAC bootstrap 用户 MUST 通过同一流程完成首次改密，bootstrap CLI MUST NOT 直接实现认证撤销逻辑。

#### Scenario: 创建会话与 bootstrap 首次登录

- **WHEN** 强制改密登录签发 password-change token
- **THEN** 系统 MUST 创建 claims 完全一致的 Redis session，token 与 session MUST 使用 `auth.jwt.password_change_token_ttl`；配置未设置或非正数时 MUST 使用 5 分钟默认 TTL，MUST NOT 永不过期
- **AND** session 创建失败时登录 MUST 失败，已签发 token MUST NOT 返回客户端
- **WHEN** RBAC bootstrap 创建的固定超级管理员以临时密码首次登录
- **THEN** 用户状态 MUST 为 `identity.UserStatusMustChangePassword`，只能获得受限 token；改密完成后 MUST 转为 normal，随后才能普通登录并使用超级管理员权限
- **AND** bootstrap CLI MUST NOT 直接执行条件凭据更新、token version 更新、投影刷新、缓存失效或 session 撤销

#### Scenario: 原子消费、并发与无效凭据

- **WHEN** token 有效且 Redis session 的绑定 claims 全部匹配
- **THEN** 系统 MUST 原子删除一次性 session，并 MAY 继续执行条件凭据更新；并发请求最多一个能消费 session、更新密码并递增一次 `token_version`
- **WHEN** token 或 session 过期、不存在、已撤销、已消费、绑定不一致，或并发请求未成功消费
- **THEN** 系统 MUST 返回统一无效凭据，MUST NOT 泄露原因或更新密码、状态与 `token_version`

#### Scenario: 条件更新与撤销

- **WHEN** session 已消费，用户仍为 `UserStatusMustChangePassword` 且 PostgreSQL 当前 `token_version` 等于 token 中旧版本
- **THEN** 系统 MUST 更新密码哈希、将状态改为 `UserStatusNormal` 并递增 `token_version`，且 MUST 在同一数据库结果中返回新 `token_version`
- **AND** 状态或版本不匹配时 MUST 返回统一无效凭据且不得更新任何字段
- **AND** 更新成功并拿到新 `token_version` 后 MUST 失效本地缓存、刷新 Redis 投影并删除该用户 refresh sessions；任一步失败 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通成功结果
- **AND** 成功更新路径 MUST NOT 在更新提交后执行第二条 `SELECT` 才获取新版本

### Requirement: 认证架构、配置与 Redis 资源生命周期

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语。application、adapter、controller 和 composition MUST 通过 framework-neutral constructor、消费侧最小 port 和窄 settings 表达依赖，并显式管理 auth 自有后台资源。auth Redis adapter MUST 使用 Cluster-capable client 与同用户 hash tag 保证多 key 原子操作，且不得关闭共享 Redis client。user-service 的配置渲染边界 MUST 私有拥有 JWT、Redis、PostgreSQL 及服务私有敏感字段路径策略，并显式传入共享脱敏原语。

#### Scenario: 服务私有配置与分层

- **WHEN** user-service 签发 token、装配认证流程或新增认证行为
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、refresh rotation、token version cache TTL 和活跃 session 上限，`common/runtime/config` MUST NOT 声明或校验这些策略
- **AND** 服务私有配置 MUST NOT 承载 password KDF、Argon2 或 bcrypt cost 预算；production-like 环境的 `auth.jwt.secret` MUST 至少 32 bytes，错误 MUST 定位配置项；默认 `auth.jwt.issuer` MUST 为 `aegiscore-user-service`
- **AND** 业务编排 MUST 位于 auth application 或 domain，Redis 与 PostgreSQL adapter MUST 只实现消费侧最小存储 port

#### Scenario: 服务私有敏感路径策略

- **WHEN** user-service CLI 或测试渲染 effective settings
- **THEN** user-service MUST 在自身 config 或 CLI 边界集中声明 `auth.jwt.secret`、Redis password、PostgreSQL password 及其他服务私有敏感路径
- **AND** 调用 `common/runtime/config` redaction primitive 时 MUST 显式传入这些路径，不得依赖 common 默认识别 auth、JWT、RBAC、Ent 或具名资源业务语义
- **AND** render 输出 MUST NOT 包含 JWT secret、Redis 凭据或 PostgreSQL 凭据，且原 settings map MUST 保持不变

#### Scenario: Framework-neutral 构造与安全依赖

- **WHEN** 构造 auth use case、store、controller 或本地 cache/invalidator
- **THEN** constructor MUST 只接收职责所需的普通 Go collaborator 和窄 settings，MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag 或服务级 named resource metadata
- **AND** 必需安全 collaborator 或 settings 缺失时 MUST 返回明确 error 并拒绝装配，MUST NOT panic、静默降级或提供 no-op 安全替身

#### Scenario: 自有资源生命周期与共享资源所有权

- **WHEN** auth session purge pool 或其他主动资源启用、停止或启动失败
- **THEN** composition MUST 显式创建、启动和幂等关闭 auth 自有主动资源；部分失败时 MUST 立即清理并保留原始失败与清理失败，关闭 MUST 受 context/deadline 约束
- **WHEN** token-version localcache 启用或禁用
- **THEN** composition MUST 提供 cache 或 direct validator 所需的稳定读取、失效和统计视图，MUST NOT 为 localcache 创建 `Close`、`ErrClosed`、resource closed 状态或 no-op close 生命周期
- **AND** auth MUST NOT 关闭共享 Redis client、Redis 投影存储或 PostgreSQL 用户存储，且 auth 自有主动资源 MUST 先于共享 Redis client 关闭
- **AND** 资源不可用时受保护访问和撤销 MUST 明确报错或 fail-closed，MUST NOT 因 holder 为空而放行旧 token、无效 session 或撤销不完整结果

#### Scenario: 日志与 Redis key 命名

- **WHEN** auth application 或关键 adapter 记录业务日志
- **THEN** logger MUST 显式注入或来自 request context，MUST NOT 依赖可变 package-level 默认 logger
- **AND** 撤销失败日志 MUST 保留 `user_id`、错误分类、`request_id`、`trace_id` 和 `span_id`，MUST NOT 暴露 token、jti、session ID、Redis key、SQL、密码或敏感原始错误
- **WHEN** auth Redis adapter 生成 session、token version projection 或 purge key
- **THEN** prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`，MUST NOT 查询、删除、迁移或双写旧 prefix；旧 prefix 数据 MUST NOT 再影响认证结果

#### Scenario: refresh session 多 key 原子操作

- **WHEN** auth 创建、轮换、撤销或裁剪同一用户的 refresh sessions
- **THEN** 同一 Lua 或事务性操作涉及的 Redis key MUST 使用同一用户 hash tag
- **AND** Redis Cluster MUST NOT 因 CROSSSLOT 拒绝同一用户的 refresh session 原子操作

#### Scenario: 强制改密和 token version key schema

- **WHEN** auth 创建或消费 password-change session，或读写 token version projection
- **THEN** 相关 Redis key MUST 使用与用户一致的 hash tag 规则
- **AND** token version projection 刷新失败时的删除补偿 MUST 继续保持 Cluster 兼容

#### Scenario: Cluster client 生命周期边界

- **WHEN** auth store、purge pool、本地 cache 或 invalidator 停止
- **THEN** auth MUST NOT 关闭共享 Redis Cluster client
- **AND** Redis Cluster MOVED/ASK、slot 初始化或 CROSSSLOT 错误 MUST 作为可诊断错误暴露，不得被吞掉或降级为认证成功

### Requirement: 后台会话清理日志不暴露 Redis key material

系统 MUST 在认证会话全量撤销后的后台 Redis purge 任务日志中避免暴露完整 Redis key、Redis key prefix、Redis namespace、用户 UUID hash tag、session ID 或可拼装 refresh session key 的材料。后台任务执行所需的 `purgeKey` 和 `sessionPrefix` MAY 作为闭包内部数据使用，但 MUST NOT 进入 `workerpool.Task.Fields` 或后台任务 error/panic 日志字段。系统 MUST NOT 保留旧的 `purge_key`、`session_prefix` 或等价兼容日志字段。

#### Scenario: purge 任务返回 error 时日志不含 key material
- **WHEN** 退出全部会话或强制改密触发 detached refresh session 后台 purge，且后台 Redis purge 任务返回 error
- **THEN** workerpool 失败日志 MUST 只包含稳定任务名、低敏批量大小、cut time 和可选不可逆 opaque 标识
- **AND** 日志字段名和值 MUST NOT 包含 `purge_key`、`session_prefix`、Redis namespace、`auth:session`、`auth:user:sessions`、`{user_uuid}`、session ID 或可拼装 refresh session key 的材料

#### Scenario: purge 任务 panic 时日志不含 key material
- **WHEN** detached refresh session 后台 purge 任务在 workerpool 执行边界发生 panic
- **THEN** workerpool panic 日志 MUST 保留 panic 和 stacktrace 观测能力
- **AND** 日志字段名和值 MUST NOT 包含 `purge_key`、`session_prefix`、Redis namespace、`auth:session`、`auth:user:sessions`、`{user_uuid}`、session ID 或可拼装 refresh session key 的材料

#### Scenario: purge 执行语义保持不变
- **WHEN** 后台 purge 任务被提交并成功执行
- **THEN** 系统 MUST 继续使用 detached purge key 读取待清理 session 索引，并使用同一用户 session prefix 构造待删除 refresh session key
- **AND** 日志字段收敛 MUST NOT 改变退出全部会话、强制改密、token version 递增、refresh session 撤销或 Redis key 存储格式

