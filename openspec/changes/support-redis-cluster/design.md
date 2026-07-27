## Context

当前 Redis 能力以单机资源为中心：`common/runtime/resources.RedisConfig` 使用 `addr`、可配置 `db` 和统一 `timeout`，`common/runtime/datastore` 固定通过 `redis.NewClient` 创建 `*redis.Client`，user-service 通过 Fx 以 `name:"cache_redis"` 注入同一 concrete client。认证会话、token version 投影、强制改密会话、RBAC policy version、Redis health、Redis metrics 和 Redis tracing 均消费这个单机 client。

目标部署需要既支持 Redis Cluster，也支持非集群 Redis 或托管代理，并以配置 `mode` 决定连接方式。Redis Cluster 需要支持阿里云 Redis 集群的单访问地址作为 seed endpoint。Redis Cluster 的约束会影响配置结构、客户端类型、Lua 多 key 操作、RBAC policy sync、健康检查、metrics、测试基建和部署资产。

## Goals / Non-Goals

**Goals:**

- 使用 mode-driven Redis 配置契约：`mode: cluster` 使用 `addrs`、`timeout` 和可选 `cluster.max_redirects`，`mode: standalone` 使用 `addr` 和 `timeout`；Redis DB 不再作为配置项，两种 mode 均固定使用 0 号库。
- `addrs` 表示 seed endpoints，MUST 允许单个阿里云访问地址。
- 根据 `mode` 校验 Redis 字段，Cluster 模式拒绝 `addr`，standalone 模式拒绝 `addrs/cluster.max_redirects`，任一 mode 均拒绝 `db` 字段。
- 使用 Redis Cluster client 初始化 `cache_redis`，并让 common 与 user-service 消费边界支持 Cluster client。
- 保证 auth 多 key Lua、transaction/pipeline 和批量删除仅操作同一 hash slot。
- 保证 RBAC policy version 与 refresh channel 在 Cluster 中可用，并通过周期性 version check 兜底 Pub/Sub 漏消息。
- 更新部署配置、测试容器、文档和 OpenSpec 主规格 delta。

**Non-Goals:**

- 不支持 Sentinel 或隐式字段推断。
- 不迁移旧 Redis 数据，不双写旧 Redis 与新 Redis Cluster。
- 不启用 Redis replica read、`read_only`、`route_by_latency` 或 `route_randomly`。
- 不改变 HTTP API、OpenAPI 路由、PostgreSQL schema 或 Ent migration。
- 不新增 eventbus、outbox、MQ、可靠消息或新的外部协议适配层。

## Decisions

### Decision: Redis 配置采用 mode-driven 契约

最终配置形态为：

```yaml
resources:
  redis:
    cache_redis:
      mode: cluster
      addrs:
        - r-xxx.redis.rds.aliyuncs.com:6379
      timeout: 5s
      cluster:
        max_redirects: 8
```

非集群配置形态为：

```yaml
resources:
  redis:
    cache_redis:
      mode: standalone
      addr: 127.0.0.1:6379
      timeout: 5s
```

`mode` 必须显式为 `cluster` 或 `standalone`。Cluster 模式下 `addrs` 至少包含一个 `host:port`，语义是 seed endpoints，不要求列出所有 Redis 节点；`cluster.max_redirects` 用于控制 MOVED/ASK redirect 上限。Standalone 模式下使用 `addr`，适用于单机 Redis 或隐藏分片细节的托管代理。Redis DB 不再作为运行时配置暴露；Cluster 只支持 0 号库，standalone 也固定使用 0 号库，避免两种 mode 出现数据隔离语义分叉。`username` 和 `password` 仍允许为空并继续支持 Redis ACL 与云厂商密码认证。

备选方案是根据 `addr` 或 `addrs` 自动推断模式，但隐式推断会让配置错误更晚暴露，因此使用显式 `mode`。

### Decision: common datastore 返回 Cluster-capable Redis client 边界

`common/runtime/datastore` 不再返回 `*redis.Client` 作为稳定边界。实现可选择 `redis.UniversalClient` 或项目内最小组合接口，但正式代码不应再要求单机 concrete type。Cluster 初始化使用 `redis.NewClusterClient`，并在 lifecycle 中执行 `PING`、tracing instrumentation 和 `Close`。

备选方案是把 Cluster client 包装成自定义 struct 并模拟 `*redis.Client`，但这会隐藏真实客户端类型并增加无业务价值的适配层。

### Decision: auth key 继续按 userID hash tag 聚合

auth Redis key 已按用户维度使用 `rediskey.HashTag(userID)`。改造后必须保留该模型，使 refresh session key、用户 session zset、purge zset、password-change session 和 token version projection 在同一用户范围内落入同一 slot。Lua 脚本和 pipeline 只允许在同一 hash tag 内执行。

备选方案是按 sessionID 分片，但 refresh session 数量限制、全部退出和 token version 投影都以 userID 为一致性边界，按 sessionID 分片会破坏现有原子语义。

### Decision: RBAC policy sync 使用固定 hash tag 与版本补偿

RBAC policy version key 和 refresh channel 使用固定 hash tag，例如 `{policy}` 或 `{rbac-sync}`，确保未来同一同步域内的 Redis 操作不会跨 slot。Pub/Sub 继续作为快速路径，周期性 `CurrentVersion` check 是必须兜底；Cluster 环境下不得把 Pub/Sub 视为唯一同步保证。

备选方案是切换 Redis Streams 或 Redis 7 sharded pub/sub。本次不引入新的同步机制，避免扩大行为面；如后续需要强通知语义，应独立提案。

### Decision: Redis health、metrics 和 tracing 使用最小接口

health 和 metrics 只需要 `Ping`，tracing 只需要对具体 go-redis client 安装 instrumentation。改造时应把 Redis ping collector 从 `*redis.Client` 适配为 Cluster-capable 最小 pinger，指标 label 仍只包含稳定 `resource`，不得增加 Redis node、addr、slot 或错误文本标签。

备选方案是在 metrics 中直接依赖 `*redis.ClusterClient`，但会把 common metrics 与具体拓扑绑定，降低复用性。

## Risks / Trade-offs

- Lua 脚本动态拼接 key 可能被部分 Redis Cluster 或托管代理拒绝 → 使用真实 Redis Cluster 集成测试覆盖 `create_session`、`rotate_session`、`detach_user_sessions`、`consume_password_change_session` 和 token version script；如目标环境拒绝未声明 key 访问，则将脚本改为只访问 `KEYS` 明确声明的 key 或拆出非原子清理步骤。
- Pub/Sub 在 Cluster 或云代理中的广播语义可能与单机不同 → 保持 policy version 周期性补偿为一致性兜底，readiness 暴露 watcher 最近错误，文档明确 Pub/Sub 只是快速路径。
- Cluster client 每节点维护连接池导致连接数放大 → 第一版不暴露复杂 pool 参数，但实施时需按 go-redis 默认和 Pod 副本数评估 Redis `maxclients`；如容量不足，后续独立增加 pool 配置。
- 旧配置在新版本启动失败 → 发布时先更新 Nacos/Helm/Kubernetes 配置，再发布应用镜像；回滚时配置和镜像必须整体回滚。
- 旧 Redis session 不迁移导致用户重新登录 → 接受该安全边界，token version cache 和 RBAC policy version 均可重建。
- 阿里云 Redis 集群可能是代理模式而非直连 Cluster 协议 → 本 change 按用户给定的 `mode: cluster` 落实；若实际实例不支持 Cluster topology，需要在部署验证阶段更换为支持 Cluster 协议的访问方式或由阿里云代理兼容 Cluster 命令。

## Migration Plan

1. 更新 OpenSpec delta、配置 fixture 和文档，明确 Redis 使用 `mode` 选择 Cluster 或 standalone。
2. 重构 `common/runtime/resources` 的 Redis 配置、默认值和 validation，按 `mode` 校验 `addrs` 或 `addr`，拒绝 Redis `db` 配置并固定使用 0 号库。
3. 重构 `common/runtime/datastore` 的 Redis client factory、Fx lifecycle、tracing instrumentation 和测试。
4. 调整 user-service provider、health、metrics、auth 和 permission 的 Redis client 类型。
5. 更新 auth 和 RBAC Redis key 相关测试，新增真实 Redis Cluster 集成测试覆盖 hash slot、多 key Lua、Pub/Sub 快速路径和版本补偿。
6. 更新 Nacos `resources.yaml`、Compose、Kubernetes、Helm 和 E2E harness 配置为 `mode/addrs/cluster.max_redirects`。
7. 发布时准备空 Redis Cluster，更新配置后滚动发布应用；不迁移旧 Redis 数据。
8. 发布后验证 `/readyz`、`/startupz`、Redis metrics、登录、refresh、退出全部会话、强制改密和 RBAC 写后同步。

回滚方式：回滚到旧应用镜像，并同步回滚 Nacos/Helm/Kubernetes Redis 配置为旧 Redis 配置结构；不从 Redis Cluster 回写旧 Redis。回滚后用户 refresh session、password-change session 和 token version cache 可失效，用户重新登录或重新发起改密流程；RBAC 可通过写操作、seed/reload 或滚动重启恢复 policy 投影。

## Open Questions

- 无。最终方案固定为 Cluster-only，不保留单机、Sentinel 或双模式兼容。
