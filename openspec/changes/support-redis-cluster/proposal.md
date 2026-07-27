## Why

当前 Redis 资源契约和运行时连接模型只支持单机地址、可配置 `db` 和 `redis.NewClient`，无法根据配置接入 Redis Cluster，尤其不能满足阿里云 Redis 集群单 seed endpoint 的生产部署形态。为避免在认证会话、token version、RBAC policy sync、健康检查和 metrics 中继续扩散单机假设，需要重构 Redis 配置、客户端初始化和使用边界，使运行时按 `mode` 选择非集群或集群客户端，并统一固定使用 Redis 0 号库。

## What Changes

- 将 `resources.redis.<name>` 从隐式单机契约重构为显式 mode 契约；`mode: cluster` 使用 `addrs`、`timeout` 和可选 `cluster.max_redirects`，`mode: standalone` 使用 `addr` 和 `timeout`，两种 mode 均不暴露 Redis DB 配置并固定使用 0 号库。
- Redis 单机或托管代理场景通过 `mode: standalone` 明确配置，Redis Cluster 场景通过 `mode: cluster` 明确配置；不支持 Sentinel。
- 支持阿里云 Redis Cluster 单访问地址，将 `addrs` 定义为 seed endpoints，允许只配置一个地址。
- 将 Redis 客户端初始化从固定 `redis.NewClient` 调整为 Cluster client 初始化，并把 user-service 的具名 `cache_redis` 依赖改为可承载 Cluster client 的接口或统一客户端类型。
- 保持认证 Redis key 的用户维度 hash tag，确保同一用户 refresh session、password change session、token version 和会话索引的多 key 操作落在同一 hash slot。
- 调整 RBAC policy version 和 policy refresh channel key，引入固定 hash tag，避免未来同一同步域内的 Redis 原子操作出现跨 slot 风险。
- 更新 Redis health、metrics、tracing instrumentation、测试容器、Nacos fixture、Compose/Kubernetes/Helm 文档和配置示例。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `shared-platform-primitives`：Redis 资源配置、校验、默认值、datastore client 初始化和测试基础设施从隐式单机契约变更为 mode-driven 契约。
- `auth-session-management`：认证会话、强制改密会话和 token version cache 的 Redis 存储必须兼容 Redis Cluster hash slot 与 Cluster client。
- `rbac-access-control`：RBAC policy version 和 policy refresh 通知必须兼容 Redis Cluster，并以版本补偿保证 Pub/Sub 漏消息后的最终同步。
- `runtime-observability`：Redis readiness/startup 检查和 Redis ping metrics 必须支持 Cluster client，同时保持低基数指标标签。
- `delivery-operations`：Nacos、Compose、Kubernetes、Helm 和测试交付资产必须使用 Redis Cluster 配置契约。

## Impact

- 影响 `common/runtime/resources` 的 Redis 配置结构、默认值和校验逻辑。
- 影响 `common/runtime/datastore` 的 Redis client factory、Fx lifecycle、tracing instrumentation 和关闭逻辑。
- 影响 user-service `providers` 中 `cache_redis` 的 Fx 注入类型、健康检查和运行时依赖 metrics。
- 影响 `user-service/internal/features/auth/infrastructure/redis` 的客户端类型、Lua 脚本 Cluster 兼容性测试和多 key 操作验证。
- 影响 `user-service/internal/features/permission/infrastructure/redis` 的 RBAC policy sync key、channel、Pub/Sub 快速路径和版本补偿检查。
- 影响 Nacos `resources.yaml`、Compose Redis fixture、Kubernetes/Helm 配置说明、e2e 配置 fixture 和 Docker-backed Redis 测试能力。
- 不影响 HTTP API、OpenAPI 路由和 PostgreSQL schema。
