## MODIFIED Requirements

### Requirement: 会话与 token version 策略

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。受保护路由的 token version 本地缓存 MUST 使用有容量上限的 `common/runtime/localcache` loading cache，并且 MUST 将 Redis token version 投影和 PostgreSQL 当前值作为回源路径。user-service auth/provider 边界 MUST 拥有 `auth_token_version` 缓存实例名，并 MUST 在缺少该配置实例时拒绝服务装配。`auth.token_version_cache_ttl` MUST 允许正数 duration 表示显式 Redis token version 投影 TTL，并 MUST 允许非正数 duration 表示使用服务默认 TTL；非正数配置 MUST NOT 创建永久 Redis token version 投影。auth application port MUST 将 PostgreSQL token version 持久化、Redis token version 投影和 refresh session 生命周期拆分为最小依赖接口，业务组件 MUST 只依赖自身所需的 port。token version 本地缓存失效接口 MUST 返回失败错误；会话撤销流程 MUST 记录本地失效失败并将其纳入投影错误返回，MUST NOT 静默忽略本地 cache 删除失败。

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
- **THEN** 系统 MUST 通过 `auth_token_version` 本地缓存容量限制控制进程内条目预算
- **AND** 系统 MUST 在容量淘汰、准入拒绝或 TTL 过期后通过 Redis 或 PostgreSQL 回源恢复校验能力

#### Scenario: token version 必需缓存配置

- **WHEN** user-service 装配 auth token version validator
- **THEN** auth/provider MUST 使用本服务常量读取 `local_cache.auth_token_version`
- **AND** 缺少该配置实例时 MUST 返回明确错误并拒绝继续装配 token version 本地缓存

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

### Requirement: 会话退出

系统 MUST 支持退出当前会话和退出全部会话，并保证退出后令牌无法继续访问受保护资源。全部会话退出 MUST 以 PostgreSQL token version 递增作为旧 access token 失效的主事实，并 MUST 明确表达本地 token version cache 失效、Redis token version 投影刷新和 refresh session 删除失败时的最终一致处理语义。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前 refresh session，且 MUST NOT 递增用户 `token_version`

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增用户 `token_version` 并撤销该用户的所有活跃 refresh session，使旧 token 无法继续刷新或访问
- **AND** PostgreSQL token version 递增成功后，旧 access token MUST 因 token version 不匹配而无法继续访问受保护资源
- **AND** 本地 token version cache 失效、Redis token version 投影刷新或 refresh session 删除失败时，系统 MUST 返回、记录或暴露可观察的投影失败信号，使调用方和测试能区分主事实成功与投影失败

#### Scenario: 全部会话后台清理

- **WHEN** 用户执行全部会话退出
- **THEN** 安全失效 MUST NOT 依赖后台 workerpool；Redis refresh session key 的批量物理删除 MAY 通过 auth 专用 purge workerpool 异步执行

#### Scenario: 重复退出

- **WHEN** 用户对已撤销或不存在的会话重复执行退出操作
- **THEN** 系统 MUST 返回稳定结果或明确错误，并 MUST NOT 恢复已撤销会话
