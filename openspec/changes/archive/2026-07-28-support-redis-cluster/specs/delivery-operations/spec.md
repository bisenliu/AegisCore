## ADDED Requirements

### Requirement: Redis Cluster 配置交付

Nacos、Compose、Kubernetes、Helm、README、E2E harness 和测试配置 fixture MUST 使用 Redis mode-driven 配置契约。交付资产 MUST 明确展示 `mode: cluster` 或 `mode: standalone`，MUST NOT 使用隐式 Redis mode、Redis DB 配置或 Sentinel 参数。

#### Scenario: Nacos 与部署配置使用 Cluster 契约

- **WHEN** 渲染或加载 user-service 运行时配置
- **THEN** Redis 资源 MUST 使用 `resources.redis.cache_redis.mode` 选择 `cluster` 或 `standalone`
- **AND** `addrs` MUST 允许单个阿里云 Redis 集群访问地址作为 seed endpoint
- **AND** Cluster 示例 MUST 使用 `addrs`，standalone 示例 MUST 使用 `addr`，配置示例 MUST NOT 展示 Redis `db` 或 Sentinel 字段

#### Scenario: Compose 与真实依赖测试

- **WHEN** Compose 或 Docker-backed 测试需要 Redis
- **THEN** 资产 MUST 提供 Redis Cluster fixture 或明确连接外部 Redis Cluster 的配置路径
- **AND** `AEGISCORE_TEST_CONTAINERS=1` 下的 Redis Cluster 兼容测试 MUST 覆盖 auth、RBAC、health 和 metrics 的 Cluster-sensitive 行为

### Requirement: Redis Cluster 发布与回滚

Redis Cluster 发布 MUST 以空 Cluster 和新配置切换为前提，不迁移旧 Redis 数据。回滚 MUST 同步回滚应用镜像和 Redis 配置契约，不要求从 Redis Cluster 回写旧 Redis。

#### Scenario: 发布顺序

- **WHEN** 发布 Redis Cluster 支持版本
- **THEN** 运维 MUST 先准备 Redis Cluster 并更新 Nacos、Kubernetes 或 Helm Redis 配置，再滚动发布 user-service
- **AND** 发布验证 MUST 覆盖 `/readyz`、`/startupz`、Redis metrics、登录、refresh、退出全部会话、强制改密和 RBAC 写后同步

#### Scenario: 回滚边界

- **WHEN** 需要回滚到旧 Redis 单机版本
- **THEN** 运维 MUST 同步回滚应用镜像和 Redis 配置为旧版本要求的契约
- **AND** 系统 MUST 接受 refresh session、password-change session、token version cache 和 RBAC policy version 在回滚过程中失效或重建
