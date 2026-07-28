## ADDED Requirements

### Requirement: Redis Cluster 健康检查与指标

Redis readiness/startup 检查和 Redis ping metrics MUST 支持 Redis Cluster client。健康响应和 metrics MUST 保持稳定低基数资源标签，MUST NOT 泄露 Redis seed endpoint、node address、slot、key、命令参数、secret 或原始错误文本。

#### Scenario: Cluster PING 健康检查

- **WHEN** `/readyz` 或 `/startupz` 检查 `redis.cache_redis`
- **THEN** health checker MUST 通过 Cluster-capable pinger 执行 PING
- **AND** Redis Cluster 不可用时响应 MUST 只返回稳定不可用消息，不得包含 endpoint、密码、key、slot 或底层错误文本

#### Scenario: Redis ping metrics 保持低基数

- **WHEN** metrics scrape 触发 Redis ping collector
- **THEN** collector MUST 支持 Cluster client 并继续导出既有 `aegiscore_redis_*` 指标契约
- **AND** 指标 label MUST 只使用稳定 resource 等低基数字段，MUST NOT 增加 node、addr、slot、mode 或错误文本 label

### Requirement: Redis Cluster tracing instrumentation

Redis tracing instrumentation MUST 支持 Redis Cluster client，并继续过滤或避免记录敏感 Redis 命令内容。Instrumentation 失败 MUST 阻止 Redis client 构造成功并关闭已创建 client。

#### Scenario: Cluster Redis 命令 tracing

- **WHEN** user-service 通过 Redis Cluster client 执行 Redis 命令
- **THEN** tracing MUST 使用服务注入的 tracer provider 创建低风险 span
- **AND** span MUST NOT 记录完整 key、参数、token、密码、seed endpoint 或连接 secret

#### Scenario: instrumentation 失败清理

- **WHEN** Redis Cluster tracing instrumentation 返回错误
- **THEN** constructor MUST 返回包含 `instrument redis tracing` 的错误并关闭已创建 client
- **AND** 系统 MUST NOT panic 或留下未关闭 Redis client
