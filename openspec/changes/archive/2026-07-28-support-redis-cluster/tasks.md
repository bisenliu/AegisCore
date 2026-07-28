## 1. 配置契约与共享资源

- [x] 1.1 重构 `common/runtime/resources.RedisConfig`，将 Redis 配置改为显式 `mode` 契约；`cluster` 使用 `addrs` 和可选 `cluster.max_redirects`，`standalone` 使用 `addr`，两种 mode 均不暴露 Redis DB 配置并固定使用 0 号库。
- [x] 1.2 更新 Redis 默认值和 validation，按 `mode` 校验 `cluster` 与 `standalone` 字段组合，并拒绝 Redis `db`、Sentinel、未知 mode 或未知字段。
- [x] 1.3 更新 user-service 配置默认值、strict decode 测试、config render 脱敏测试和 e2e 配置 fixture，使 Redis 示例使用 Cluster-only 契约。

## 2. Redis client 初始化与观测

- [x] 2.1 重构 `common/runtime/datastore` Redis constructor 和 Fx adapter，使用 Cluster client 初始化并返回 Cluster-capable client 边界，不再要求 `*redis.Client` 单机 concrete type。
- [x] 2.2 更新 Redis tracing instrumentation，使其支持 Cluster client；instrumentation 或启动 PING 失败时关闭已创建 client 并保留错误链。
- [x] 2.3 更新 `common/runtime/observability/metrics` Redis pinger 和 user-service health/metrics provider，使 PING 支持 Cluster client 且指标 label 保持低基数。
- [x] 2.4 更新相关 common 与 provider 单元测试，覆盖 timeout 映射、`cluster.max_redirects` 映射、失败清理、health 和 metrics 行为。

## 3. user-service Redis 消费方改造

- [x] 3.1 调整 `user-service/internal/providers` 中 `cache_redis` 的 Fx 注入类型和 factory，保证 auth、permission、health、metrics 共享同一个 Cluster-capable Redis resource。
- [x] 3.2 调整 auth Redis adapter 的 client 字段、constructor、Lua script 调用、pipeline 和批量删除调用，保持同一用户 hash tag 下的 Cluster 兼容性。
- [x] 3.3 调整 RBAC permission Redis store 的 client 类型，并将 policy version key 与 policy refresh channel 改为固定 hash tag。
- [x] 3.4 更新 auth 和 permission 测试，覆盖 hash tag、CROSSSLOT 防护、token version 投影、refresh rotation、退出全部会话、强制改密 session、policy version 发布和 watcher 版本补偿。

## 4. 测试基础设施与部署资产

- [x] 4.1 新增或改造 `common/testing/containers` 的 Redis Cluster fixture，使 Docker-backed 测试能启动真实 Redis Cluster 并完成 slot 初始化。
- [x] 4.2 为 Redis Cluster 增加集成测试路径，覆盖 auth 多 key Lua、RBAC Pub/Sub 快速路径、周期性 version check、health PING 和 metrics PING。
- [x] 4.3 更新 `deployments/nacos`、Compose、Kubernetes、Helm values 和相关 README，Redis 示例统一使用 `mode: cluster`、单元素 `addrs`、`timeout` 和 `cluster.max_redirects`。
- [x] 4.4 更新 docs 和能力地图中 Redis 资源说明，明确不迁移旧 Redis 数据、不保留单机或 Sentinel 兼容，并说明发布和回滚顺序。

## 5. 规格、生成与验证

- [x] 5.1 确认本 change 的 OpenSpec delta 与实现一致，必要时同步更新 `openspec/changes/support-redis-cluster/specs/**/*.md`。
- [x] 5.2 运行 `make user-service-architecture-lint`，修复因 Redis client 边界、common/user-service 归属或部署文档引入的架构问题。
- [x] 5.3 运行相关包测试、`make common-test` 和 `make user-service-test`，并在 `AEGISCORE_TEST_CONTAINERS=1` 下运行 Redis Cluster 集成测试。
- [x] 5.4 如果 OpenAPI 注解或健康检查文案变化，运行 `make user-service-openapi-generate` 并检查生成物 drift；若未变化，确认无需更新 OpenAPI 生成物。
- [x] 5.5 将本次预期代码、文档、OpenSpec 和生成物变更加到暂存区，再运行 `make lint` 和 `make verify`，确认最终无非预期 drift。
