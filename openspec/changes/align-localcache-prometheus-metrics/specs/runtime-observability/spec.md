## MODIFIED Requirements

### Requirement: 本地缓存运行时指标

系统 MUST 为 `common/runtime/localcache` 提供 Prometheus metrics collector，导出低基数本地缓存运行时指标，用于观察命中率、回源率、回源错误、singleflight 合并、内部 double-check、写入丢弃、准入拒绝、淘汰和容量。部署观测资产、真实 metrics load 脚本和运行手册 MUST 消费当前稳定的 `aegiscore_localcache_*` metric family，并 MUST NOT 保留旧 metric name、旧 label 或兼容 PromQL。

#### Scenario: 导出本地缓存请求指标

- **WHEN** metrics 配置启用且服务注册 localcache collector
- **THEN** 系统 MUST 导出本地缓存 hit 和 miss counter
- **AND** 指标标签 MUST 只包含固定缓存名和固定枚举结果

#### Scenario: 导出本地缓存回源指标

- **WHEN** `GetOrLoad` 因 miss 执行 loader 或 loader 返回错误
- **THEN** 系统 MUST 导出 loader 调用和 loader 错误 counter
- **AND** 指标 MUST NOT 包含用户 ID、角色 ID、权限 ID、token、raw key、SQL、Redis key 或原始错误

#### Scenario: 导出防击穿指标

- **WHEN** `singleflight` 合并同 key 并发 miss 或内部 double-check 命中
- **THEN** 系统 MUST 导出 shared result 和 double-check hit counter
- **AND** 这些指标 MUST NOT 计入业务缓存 hit ratio

#### Scenario: 导出淘汰与拒绝指标

- **WHEN** Ristretto 丢弃写入、拒绝准入或淘汰缓存项
- **THEN** 系统 MUST 导出 set dropped、admission rejected 和 evicted counter
- **AND** SRE MUST 能通过这些指标判断容量、TTL 或 key 基数是否需要调整

#### Scenario: dashboard 消费 localcache 指标

- **WHEN** Grafana dashboard 展示 user-service runtime 依赖状态
- **THEN** dashboard MUST 使用 `aegiscore_localcache_requests_total`、`aegiscore_localcache_loads_total`、`aegiscore_localcache_singleflight_total`、`aegiscore_localcache_writes_total`、`aegiscore_localcache_evictions_total` 和 `aegiscore_localcache_capacity`
- **AND** PromQL MUST 按 `cache`、`result` 或 `event` 等固定 label 聚合，不得引用旧 metric name、旧 label 或 raw cache key

#### Scenario: alert 消费 localcache 指标

- **WHEN** Prometheus alert rules 评估本地缓存异常
- **THEN** alert MUST 使用当前 `aegiscore_localcache_*` metric family 表达 loader error、set dropped、admission rejected 或 eviction pressure 等可行动信号
- **AND** alert annotation MUST 指向稳定 runbook，并说明优先检查容量、TTL、key 基数、回源依赖和缓存注册状态

#### Scenario: metrics load 验证 localcache 指标

- **WHEN** 执行真实 metrics load 脚本采样 `/metrics` 和 Prometheus 查询
- **THEN** 脚本 MUST 检查 localcache metric family 的服务端 presence
- **AND** Prometheus sample query MUST 覆盖请求、回源、singleflight、写入、淘汰和容量指标，确保 collector 缺失或 PromQL 漂移能够被发现

#### Scenario: metrics 禁用

- **WHEN** metrics provider 被禁用或未配置
- **THEN** 系统 MUST 不注册 localcache Prometheus collector
- **AND** localcache 自身 MUST 继续维护可通过 `Stats()` 读取的本地统计快照
