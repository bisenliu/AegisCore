## MODIFIED Requirements

### Requirement: Token version 校验与会话撤销

系统 MUST 以 PostgreSQL 当前 `token_version` 为主事实，并通过有界本地 loading cache 和 Redis 投影加速校验。缓存未命中、过期、容量驱逐或显式禁用只能影响性能，MUST NOT 改变认证与撤销语义。退出全部会话和密码变更 MUST 递增主事实，使旧 access token 失效并撤销相应 refresh sessions。token-version feature cache MUST 由 user-service 私有配置提供完整默认值、启用时校验和到通用 localcache 配置的集中映射，auth 构造路径 MUST 只消费窄 auth settings。递增 `token_version` 或更新凭证并返回新 `token_version` MUST 是单一确定的 PostgreSQL 结果，成功路径 MUST NOT 在提交后通过第二条 `SELECT` 才获取撤销版本。

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

#### Scenario: 全部退出与已认证改密

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增 PostgreSQL `token_version` 并在同一数据库结果中返回新版本，然后撤销全部活跃 refresh sessions，使旧 access 和 refresh token 无效
- **AND** 安全失效 MUST NOT 依赖后台 workerpool，Redis key 物理清理 MAY 异步执行
- **WHEN** 已认证用户提供正确旧密码和满足策略的新密码
- **THEN** 系统 MUST 原子更新密码哈希并递增 `token_version`，在同一数据库结果中返回新版本，再执行本地缓存失效、Redis 投影刷新和 refresh session 撤销
- **WHEN** 旧密码错误或新密码不满足策略
- **THEN** 系统 MUST 拒绝修改并保持密码、状态和 `token_version` 不变

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

### Requirement: 认证架构、配置与资源生命周期

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语。application、adapter、controller 和 composition MUST 通过 framework-neutral constructor、消费侧最小 port 和窄 settings 表达依赖，并显式管理 auth 自有后台资源。默认 JWT issuer、auth Redis key prefix 和相关示例 MUST 使用 `aegiscore-user-service`，MUST NOT 兼容、读取或双写旧 `aegiscore-user-services` 值。

#### Scenario: 服务私有配置与分层

- **WHEN** user-service 签发 token、装配认证流程或新增认证行为
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、refresh rotation、token version cache TTL 和活跃 session 上限，`common/runtime/config` MUST NOT 声明或校验这些策略
- **AND** 服务私有配置 MUST NOT 承载 password KDF、Argon2 或 bcrypt cost 预算；production-like 环境的 `auth.jwt.secret` MUST 至少 32 bytes，错误 MUST 定位配置项；默认 `auth.jwt.issuer` MUST 为 `aegiscore-user-service`
- **AND** 业务编排 MUST 位于 auth application 或 domain，Redis 与 PostgreSQL adapter MUST 只实现消费侧最小存储 port

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

