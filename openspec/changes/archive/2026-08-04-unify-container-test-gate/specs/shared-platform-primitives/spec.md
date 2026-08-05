## MODIFIED Requirements

### Requirement: Redis Cluster 测试基础设施

系统 MUST 提供可由集成测试复用的 Redis Cluster 测试能力，用于验证 hash slot、多 key Lua、Pub/Sub、PING 和 MOVED/ASK redirect 相关行为。普通单元测试 MAY 继续使用 mock 或轻量 Redis fixture，但 Cluster 兼容性 MUST 通过真实 Redis Cluster 覆盖。

#### Scenario: 真实 Cluster 集成测试

- **WHEN** 模块容器测试 target 通过 `-args -aegiscore.testcontainers` 启用真实依赖测试
- **THEN** Redis Cluster 相关集成测试 MUST 实际连接 Cluster fixture 并执行 Cluster-sensitive Redis 命令
- **AND** Docker daemon、Cluster fixture 启动、slot 初始化或连接失败 MUST 使相关集成测试失败而不是静默跳过
- **AND** `common/testing/containers` 自身的 PostgreSQL 与 Redis 集成测试 MUST 包含在根 `make test-containers` 门禁中
