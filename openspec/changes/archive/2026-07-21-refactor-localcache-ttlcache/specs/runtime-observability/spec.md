## MODIFIED Requirements

### Requirement: 本地缓存指标契约

系统 MUST 为 `common/runtime/localcache` 导出低基数 `aegiscore_localcache_*` 指标，且稳定契约 MUST 只覆盖请求、回源、自动驱逐和 item 容量，并由 dashboard、alert 和真实 metrics load 校验消费当前契约。

#### Scenario: 请求和回源指标

- **WHEN** cache 命中、未命中、loader 成功或 loader 失败
- **THEN** 系统 MUST 分别通过 `aegiscore_localcache_requests_total{cache,result="hit|miss"}` 和 `aegiscore_localcache_loads_total{cache,result="success|error"}` 导出累计值
- **AND** success MUST 直接来自 `Stats.LoadSuccess`，MUST NOT 通过 load 总数减去 error 计算
- **AND** 标签 MUST 仅使用固定 cache 名与固定 result 枚举，MUST NOT 包含 raw key、身份标识或原始错误

#### Scenario: 自动驱逐和容量指标

- **WHEN** TTL 过期或达到最大 item 数使 cache 自动移除条目
- **THEN** 系统 MUST 通过 `aegiscore_localcache_evictions_total{cache}` 导出累计自动驱逐值
- **WHEN** collector 读取 cache 容量
- **THEN** 系统 MUST 通过 `aegiscore_localcache_capacity{cache}` 导出配置的最大 item 数
- **AND** 显式 `Delete` 或 `Clear` MUST NOT 计入 eviction

#### Scenario: 删除底层实现细节指标

- **WHEN** collector、dashboard、alert、runbook 或 metrics load 脚本消费本地缓存指标
- **THEN** 系统 MUST NOT 导出或查询 `aegiscore_localcache_singleflight_total` 与 `aegiscore_localcache_writes_total`
- **AND** shared、double-check、set-dropped 和 admission-rejected MUST NOT 作为稳定统计字段、event label 或兼容 PromQL 保留

#### Scenario: metrics 禁用和观测资产

- **WHEN** metrics provider 被禁用
- **THEN** localcache collector MUST 不注册，但 localcache MUST 继续维护可由 `Stats()` 读取的本地快照
- **WHEN** Grafana、Prometheus alert 或 metrics load 脚本消费本地缓存指标
- **THEN** 其 MUST 使用 requests、loads、evictions 和 capacity 四组当前 metric family 及 `cache`、`result` 固定标签
- **AND** 源 dashboard、provisioning JSON、alert、metrics load 校验和 runbook MUST 在同一变更中保持一致
