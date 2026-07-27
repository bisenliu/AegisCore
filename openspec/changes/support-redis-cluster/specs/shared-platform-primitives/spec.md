## ADDED Requirements

### Requirement: Redis mode-driven 资源契约

系统 MUST 将共享 Redis 资源契约定义为 mode-driven 配置，`mode: cluster` MUST 使用 `addrs`、`timeout` 和可选 `cluster.max_redirects`，`mode: standalone` MUST 使用 `addr` 和 `timeout`。`addrs` MUST 表示 Redis Cluster seed endpoints，并 MUST 允许只配置一个阿里云 Redis 集群访问地址。Redis DB MUST NOT 作为配置项暴露；Cluster 与 standalone 均 MUST 固定使用 Redis 0 号库。系统 MUST NOT 支持 Sentinel 或根据字段隐式推断 mode。

#### Scenario: 加载最小 Cluster 配置

- **WHEN** 配置包含 `resources.redis.cache_redis.mode=cluster`、至少一个 `addrs` 地址和正数 `timeout`
- **THEN** 配置加载、默认值应用和通用 validation MUST 成功
- **AND** 每个 `addrs` 元素 MUST 按 `host:port` 校验，错误路径 MUST 包含资源名和字段路径

#### Scenario: 按 mode 校验 Redis 配置

- **WHEN** `mode=cluster` 的配置包含 `addr` 或 `db`，或 `mode=standalone` 的配置包含 `addrs`、`cluster.max_redirects` 或 `db`
- **THEN** Redis resource validation MUST 在启动前失败并报告完整字段路径
- **AND** 未声明 mode、未知 mode、Sentinel 字段或未知 Redis 字段 MUST 在启动前失败

### Requirement: Redis Cluster client 构造与生命周期

系统 MUST 通过共享 datastore 构造可承载 Redis Cluster 的单资源 client，并在 Fx lifecycle 中执行启动 PING、tracing instrumentation 和关闭清理。Redis client 公开边界 MUST 支持 Cluster client，MUST NOT 要求 `*redis.Client` 单机 concrete type。

#### Scenario: 构造并探测 Cluster client

- **WHEN** 调用方使用有效 Redis Cluster 资源配置创建 `cache_redis`
- **THEN** datastore MUST 使用 Cluster client 初始化，并将 `timeout` 映射到 dial、read、write 和启动 PING timeout
- **AND** `cluster.max_redirects` 配置存在时 MUST 映射到 Cluster redirect 上限
- **AND** 启动 PING 或 tracing instrumentation 失败时 MUST 关闭已创建 client，并保留主错误和关闭错误

#### Scenario: client 所有权与 feature 边界

- **WHEN** feature 消费共享 Redis 资源
- **THEN** feature MUST 只消费 Cluster-capable client 或消费侧最小接口
- **AND** feature 自有 workerpool、watcher、cache 或 store 关闭时 MUST NOT 关闭共享 Redis client

### Requirement: Redis Cluster 测试基础设施

系统 MUST 提供可由集成测试复用的 Redis Cluster 测试能力，用于验证 hash slot、多 key Lua、Pub/Sub、PING 和 MOVED/ASK redirect 相关行为。普通单元测试 MAY 继续使用 mock 或轻量 Redis fixture，但 Cluster 兼容性 MUST 通过真实 Redis Cluster 覆盖。

#### Scenario: 真实 Cluster 集成测试

- **WHEN** `AEGISCORE_TEST_CONTAINERS=1` 启用真实依赖测试
- **THEN** Redis Cluster 相关集成测试 MUST 实际连接 Cluster fixture 并执行 Cluster-sensitive Redis 命令
- **AND** Docker daemon、Cluster fixture 启动、slot 初始化或连接失败 MUST 使相关集成测试失败而不是静默跳过
