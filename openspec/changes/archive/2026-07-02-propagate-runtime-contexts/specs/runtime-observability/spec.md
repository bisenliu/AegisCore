## ADDED Requirements

### Requirement: Redis metrics 探测上下文传播

系统 MUST 在 metrics endpoint 处理 HTTP scrape 时，将本次 scrape request context 传播到 Redis runtime metrics 的 PING 探测，并继续使用配置化或默认 timeout 约束单次探测耗时。Redis metrics 的 metric family、label key、label value 和数值语义 MUST 保持稳定。

#### Scenario: scrape 取消终止 Redis PING
- **WHEN** metrics endpoint 正在执行 Redis PING 探测且 HTTP scrape request context 被取消
- **THEN** Redis PING MUST 观察到取消信号并尽快结束
- **AND** 系统 MUST NOT 因已取消 scrape 继续持有无意义的 Redis PING IO 直到外部网络超时

#### Scenario: Redis 探测 timeout 保留
- **WHEN** HTTP scrape request context 未取消但 Redis PING 超过 collector 配置的 timeout
- **THEN** Redis PING MUST 按 collector timeout 结束并记录 Redis 不可用快照
- **AND** `aegiscore_redis_up`、`aegiscore_redis_ping_duration_seconds` 和 `aegiscore_redis_ping_failures_total` 的名称、标签和含义 MUST 保持不变

#### Scenario: 最小探测间隔保留
- **WHEN** 连续 scrape 发生在 Redis collector 的最小探测间隔内
- **THEN** collector MUST 复用最近一次 Redis PING 快照
- **AND** 复用快照不得改变既有 metric family 或 label 契约
