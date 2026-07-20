## MODIFIED Requirements

### Requirement: Token version 权威来源与缓存投影

系统 MUST 以 PostgreSQL 当前 `token_version` 为主事实，并通过有界本地 loading cache 和 Redis 投影加速受保护访问校验。校验链路 MUST 在本地缓存或 Redis 未命中时回源 PostgreSQL，MUST NOT 缓存错误结果，且缓存关闭、TTL 过期或容量驱逐 MUST 只影响性能、不得改变认证和撤销语义。

#### Scenario: 受保护访问校验

- **WHEN** access token 已通过签名、subject 和过期校验
- **THEN** 系统 MUST 按有界本地缓存、Redis token version 投影和 PostgreSQL 当前值的顺序解析当前版本
- **AND** token claims 版本与当前版本不一致时 MUST 拒绝访问
- **AND** Redis miss 后系统 MAY 回源 PostgreSQL并回填 Redis 和本地缓存

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
