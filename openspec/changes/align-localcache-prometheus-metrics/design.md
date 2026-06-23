## Context

`common/runtime/localcache` 当前通过 `StatsSource` 暴露稳定快照，`common/runtime/observability/metrics.LocalcacheCollector` 已导出以下 Prometheus metric family：

- `aegiscore_localcache_requests_total{cache,result}`：`hit`、`miss`
- `aegiscore_localcache_loads_total{cache,result}`：`success`、`error`
- `aegiscore_localcache_singleflight_total{cache,event}`：`shared`、`double_check_hit`
- `aegiscore_localcache_writes_total{cache,event}`：`set_dropped`、`rejected`
- `aegiscore_localcache_evictions_total{cache}`
- `aegiscore_localcache_capacity{cache}`

user-service 已在 runtime dependency metrics provider 中注册 `auth_token_version` 和 `rbac_user_roles` 两个本地缓存 collector。现有 dashboard、告警和真实 metrics load 脚本覆盖 HTTP、auth/RBAC、PostgreSQL、Redis、workerpool、scheduler、RBAC watcher 和 runtime component，但 localcache PromQL 消费面不完整，导致重构后的缓存行为只能在原始 `/metrics` 中看到，难以在运行看板、告警和验证脚本中发现异常。

本 change 影响 `common` 的 collector 语义验证、`user-service` 的 collector 注册验证、`deployments` 的 Prometheus/Grafana 资产、`docs` 的运行手册和 `openspec` 规格。它不改变 HTTP API、数据库 migration、OpenAPI 生成物、安全授权边界或 feature 业务流程。

## Goals / Non-Goals

**Goals:**

- 对齐 localcache 重构后的 Prometheus 指标消费面，确保 dashboard、alert、metrics load 脚本和文档都使用当前稳定 metric family。
- 覆盖请求命中率、回源成功/错误、singleflight shared/double-check、写入丢弃、准入拒绝、淘汰和容量配置。
- 保持 label 低基数，只使用 `service`、`environment`、`cache`、`result`、`event` 等固定标签。
- 明确不保留旧 metric name、旧 label 或兼容 PromQL。
- 通过 `make compose-dashboard-check`、Prometheus rule 校验和相关 Go 测试确认资产与 collector 语义一致。

**Non-Goals:**

- 不重新设计 `common/runtime/localcache` 的 API、Ristretto 配置、TTL、singleflight 或 stats 计数语义。
- 不引入 Ristretto 原生高维 metrics，也不导出 raw cache key、用户 ID、角色 ID、权限 ID、token 或 Redis key。
- 不改变 `local_cache` 配置结构、缓存实例名、auth token version 校验流程或 RBAC user roles resolver 行为。
- 不新增 OpenAPI、Ent schema、Atlas migration、入站 gRPC、eventbus、outbox 或外部系统集成。

## Decisions

1. 使用当前 `aegiscore_localcache_*` metric family 作为唯一 PromQL contract。

   备选方案是为旧指标或历史 label 添加 `or` 查询、recording rule 或兼容 alias。该方案会让 dashboard 和告警长期携带已废弃语义，并削弱重构后缺失指标的可见性。本 change 直接迁移消费资产，缺失当前指标时让验证和看板暴露问题。

2. Grafana 以通用 dashboard 为源，Compose dashboard 继续作为生成产物。

   localcache 面板先更新 `deployments/observability/grafana/user-service-overview.json`，再通过 `make compose-dashboard-generate` 同步 `deployments/compose/grafana/dashboards/user-service-overview.json`。备选方案是只改 Compose 副本，但会破坏当前“通用源文件生成 Compose 产物”的交付规则。

3. Prometheus alert 聚焦可行动异常，而不是为每个 counter 建告警。

   告警应覆盖 loader error、写入丢弃/准入拒绝、淘汰压力这类需要容量、TTL、key 基数或回源依赖排查的信号。命中率和 singleflight 主要进入 dashboard 和 sample query，避免低流量环境产生噪声。

4. 验证脚本同时检查服务端 presence 和 Prometheus sample。

   `generate-real-metrics-load.sh` 应把 localcache metric family 加入 `/metrics` presence check，并在 Prometheus sample query 中按 `cache`、`result`、`event` 聚合。这样能区分“服务没注册 collector”和“Prometheus 尚未 scrape 到数据”。

5. Go 测试只在必要时补齐 collector 与 provider 语义，不新增兼容测试。

   如果审计发现当前 `localcache_test.go` 或 provider routes/metrics 测试未覆盖某个稳定 family、label 或 cache 实例，应补测试来锁定当前 contract。测试应读取结构化 metric family 或固定文本断言，不解析旧 metric。

## Risks / Trade-offs

- [Risk] 低流量环境中 localcache counter 长时间为 0，dashboard 面板可能显示空或 0。→ 使用 `rate`、`increase`、`clamp_min` 和说明文档区分“无流量”和“collector 缺失”，presence check 负责发现缺失 collector。
- [Risk] 新告警阈值过敏，缓存冷启动或小容量测试环境可能触发噪声。→ 告警使用持续窗口和可行动阈值，并在 runbook 中说明先看容量、TTL、key 基数和回源依赖。
- [Risk] 手工修改 Grafana JSON 容易造成 Compose 副本漂移。→ 只把通用 dashboard 作为编辑源，提交前运行 `make compose-dashboard-generate` 和 `make compose-dashboard-check`。
- [Risk] localcache 指标由 shared collector 提供，但消费资产属于 user-service 观测面，边界容易混淆。→ collector 语义留在 `common/runtime/observability/metrics`，服务实例注册留在 `user-service/internal/providers`，PromQL 消费留在 `deployments/observability` 和 `deployments/compose`。

## Migration Plan

1. 审计当前 `/metrics` 输出、collector 测试、provider 注册测试、Grafana dashboard、Prometheus alert 和 metrics load 脚本中的 localcache 覆盖情况。
2. 如 collector 或 provider 测试缺少稳定 family 覆盖，先补齐 Go 测试和必要实现修正。
3. 更新通用 Grafana dashboard 的 localcache 面板，生成 Compose dashboard 副本。
4. 更新 Prometheus alert rules 和 runbook，补充 localcache 排障路径。
5. 更新真实 metrics load 脚本，让 presence check 与 Prometheus sample 查询覆盖 localcache 指标。
6. 运行相关 Go 测试、`make compose-dashboard-check`，并在可用时运行 `promtool check rules deployments/observability/prometheus/user-service-alerts.yaml`。

回滚方式是回退本 change 对 dashboard、alert、脚本、文档和必要测试/collector 修正的提交；因为不涉及数据库、HTTP API 或配置结构，不需要数据迁移或运行时双写。

## Open Questions

- 告警阈值需要在实现时结合现有用户服务本地部署流量和缓存容量默认值微调；初版应偏保守，避免低流量噪声。
