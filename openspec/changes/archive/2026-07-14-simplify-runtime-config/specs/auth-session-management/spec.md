## MODIFIED Requirements

### Requirement: 会话与 token version 策略

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。受保护路由的 token version 本地缓存 MUST 使用有容量上限的 `common/runtime/localcache` loading cache，并且 MUST 将 Redis token version 投影和 PostgreSQL 当前值作为回源路径。user-service MUST 从服务自有 `resources.redis` 和 `resources.postgres` 读取认证资源，并使用 `auth.token_version_cache` 配置本地缓存，MUST NOT 从共享核心 Config 读取 Redis、PostgreSQL 或 LocalCache 业务字段。`auth.token_version_cache_ttl` MUST 允许正数 duration 表示显式 Redis token version 投影 TTL，并 MUST 允许非正数 duration 表示使用服务默认 TTL；非正数配置 MUST NOT 创建永久 Redis token version 投影。auth application port MUST 将 PostgreSQL token version 持久化、Redis token version 投影和 refresh session 生命周期拆分为最小依赖接口，业务组件 MUST 只依赖自身所需的 port。token version 本地缓存失效接口 MUST 返回失败错误；会话撤销流程 MUST 记录本地失效失败并将其纳入投影错误返回，MUST NOT 静默忽略本地 cache 删除失败。

#### Scenario: 活跃 session 上限

- **WHEN** 用户超过配置的活跃 refresh session 上限
- **THEN** Redis 中最旧的活跃会话 MUST 作为安全敏感操作的一部分被同步裁剪

#### Scenario: token version 校验链路

- **WHEN** access token 已通过 JWT 解析且未过期
- **THEN** 受保护路由 MUST 按有界本地缓存、Redis token version 投影、PostgreSQL 当前值回源的顺序解析当前版本
- **AND** Redis miss 后 MAY 回源数据库并回填 Redis
- **AND** 系统 MUST NOT 缓存错误结果
- **AND** token version validator MUST NOT 依赖 refresh session 创建、轮换、查询或批量删除 port

#### Scenario: token version 本地缓存容量

- **WHEN** 不同用户的 access token version 在同一实例内被校验
- **THEN** 系统 MUST 通过 `auth.token_version_cache.size` 限制进程内条目预算
- **AND** 系统 MUST 在容量淘汰、准入拒绝或 TTL 过期后通过 Redis 或 PostgreSQL 回源恢复校验能力

#### Scenario: 认证资源和缓存缺省值

- **WHEN** user-service 装配 auth token version validator
- **THEN** Redis 和 PostgreSQL MUST 从 `resources.redis` 和 `resources.postgres` 的必需具名资源读取
- **AND** 未显式配置 `auth.token_version_cache` 时 MUST 使用 `enabled=true`、`size=100000`、`ttl=1s` 和 `load_timeout=300ms`
- **AND** `auth.token_version_cache` MUST NOT 暴露 `num_counters` 或 `buffer_items`

#### Scenario: 校验启用的 token version cache

- **WHEN** `auth.token_version_cache.enabled` 为 true
- **THEN** `size`、`ttl` 和 `load_timeout` MUST 为正值

#### Scenario: 关闭 token version cache

- **WHEN** `auth.token_version_cache.enabled` 为 false
- **THEN** 认证授权和撤销语义 MUST 保持正确
- **AND** 关闭缓存 MAY 只造成额外 datastore 查询
- **AND** `size`、`ttl` 和 `load_timeout` MAY 为零值

#### Scenario: production-like JWT secret 长度校验

- **WHEN** runtime environment 为 production-like 环境且配置包含 `auth.jwt.secret`
- **THEN** user-service 配置校验 MUST 要求该 secret 至少为 32 bytes
- **AND** development 环境 MAY 不执行该长度约束
- **AND** 校验错误 MUST 明确定位到 `auth.jwt.secret`

#### Scenario: token version 投影 TTL 默认值

- **WHEN** `auth.token_version_cache_ttl` 配置为 `0` 或负数，且系统写入 Redis token version 投影
- **THEN** 系统 MUST 使用服务默认 TTL 写入 Redis token version 投影
- **AND** 系统 MUST NOT 写入无过期时间的 token version 投影

#### Scenario: token version 投影 TTL 显式值

- **WHEN** `auth.token_version_cache_ttl` 配置为正数 duration，且系统写入 Redis token version 投影
- **THEN** 系统 MUST 使用该显式 TTL 写入 Redis token version 投影

#### Scenario: token version 投影刷新

- **WHEN** 用户执行全部会话退出或强制改密导致当前 `token_version` 变化
- **THEN** 系统 MUST 使本实例本地 token version 缓存失效，并刷新 Redis token version 投影
- **AND** 旧版本 MUST NOT 覆盖 Redis 中已存在的较新版本
- **AND** Redis token version 投影刷新失败时，系统 MUST 尝试删除 Redis 投影，使后续校验能够回源 PostgreSQL
- **AND** 投影刷新失败 MUST 被记录并可测试，不得被静默忽略

#### Scenario: token version 本地缓存失效失败

- **WHEN** 用户执行全部会话退出或强制改密导致系统尝试删除本实例本地 token version cache，且本地 cache 删除返回错误
- **THEN** 系统 MUST 记录包含 `user_id` 和错误信息的日志
- **AND** 会话撤销流程 MUST 将该错误纳入投影错误返回
- **AND** 系统 MUST NOT 继续静默忽略本地 token version cache 删除失败

#### Scenario: session lifecycle 必需本地失效器

- **WHEN** auth application 构造 refresh session lifecycle
- **THEN** `TokenVersionLocalInvalidator` MUST 作为必需依赖提供
- **AND** 缺失该依赖时系统 MUST fail-fast 或拒绝装配，MUST NOT 静默跳过本地 token version cache 失效
