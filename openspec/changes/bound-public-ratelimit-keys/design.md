## Context

当前 API 限流能力位于 `common/http/middleware/ratelimit.go` 和 user-service transport/router 组装层。`LocalRateLimiter` 使用分片 map 保存每个限流 Key 的 `rate.Limiter` 与 `lastSeen`，janitor 按清理间隔逐步扫描分片并删除超过 TTL 的 Key。

该模型可以限制单个 IP 或 User ID 的请求速率，但不能限制新 Key 的创建速率。公网匿名认证入口按 IP 限流时，攻击者可以通过大量源 IP 或 IPv6 地址轮换持续创建新 Key，使 `visitors` map 在 TTL 和完整扫描周期内持续膨胀，造成内存、GC 和认证入口可用性风险。

受影响路径包括 `common/http/middleware/ratelimit.go`、`user-service/internal/config/config.go`、`user-service/internal/providers/transport/ratelimit.go`、`user-service/internal/router/ratelimit_observability.go`、对应测试和 `api-rate-limiting` OpenSpec delta。不涉及数据库 migration、OpenAPI 生成物、业务 DTO、RBAC policy sync 或外部系统集成。

## Goals / Non-Goals

**Goals:**

- 为本地限流器增加可配置的进程内 Key 容量上限，保证唯一 Key 攻击下内存占用有硬边界。
- 容量耗尽时保留已有 Key 的 limiter 状态，避免攻击者通过不断制造新 Key 重置或绕过已有 Key 的限流窗口。
- 为容量耗尽、拒绝、overflow 和驱逐提供低基数日志/metrics 观测面。
- 保持 `common` 中限流 primitive 业务中立，服务私有默认值和策略选择留在 user-service 配置与 provider 层。
- 覆盖容量边界、并发和 race 测试，验证条目数不超过容量。

**Non-Goals:**

- 不提供跨副本全局精确配额；限流器仍是单实例本地状态。
- 不为每请求计数引入 Redis、PostgreSQL、消息队列或外部依赖。
- 不改变公开 HTTP API、认证响应 envelope、OpenAPI 文档或数据库 schema。
- 不引入 feature 业务语义到 `common/http/middleware`。

## Decisions

### Decision: 在 `LocalRateLimiter` 内实现分片有界容量

`LocalRateLimiterOptions` 增加 `MaxKeys`，构造时按分片数计算每个 shard 的容量预算。`Allow` 在创建新 visitor 前检查 shard 容量；已有 Key 始终使用原 visitor 并刷新 `lastSeen`。

选择分片容量而不是全局锁计数，是因为当前结构已经按 shard 加锁，分片容量可以避免每个请求竞争全局计数器，并让容量检查与 map 写入在同一临界区内完成。备选方案是全局 atomic 计数加每 shard map，但删除、驱逐和并发失败回滚更复杂，且仍需要处理 shard 热点。

### Decision: 容量耗尽默认使用共享 overflow bucket，而不是直接淘汰已有 Key

当新 Key 所属 shard 已满时，限流器不再把该 Key 插入 `visitors` map，而是使用该 shard 的共享 overflow limiter 进行判定，并返回可观察事件。这样 Key 数量保持有界，已有 Key 状态不会被新 Key 驱逐重置。

选择 overflow bucket 是为了在容量耗尽时仍对未知新 Key 保持限流能力，而不是 fail-open。备选方案包括 fail-closed 和 LRU 驱逐：fail-closed 对突发合法新用户更激进；LRU 在攻击流量下可能驱逐真实已有 Key，导致状态重置和公平性下降。实现可以保留 `overflow`/`reject` 策略配置，其中 user-service 公网匿名默认使用 `overflow`，高安全部署可配置为 `reject`。

### Decision: 容量事件通过稳定错误/事件类型向 router 观测层暴露

`common` 层提供业务中立的容量耗尽错误或结果 reason，例如 `ErrRateLimitCapacityExceeded`，并确保 `RateLimit` middleware 可以触发 `OnError` 或新的低基数事件回调。user-service 的 `rateLimitObserver` 将其映射到稳定 reason，例如 `capacity_exceeded`、`overflow`、`rejected`。

选择在 router 观测层注册服务指标，是为了保留 `common` primitive 的无业务语义，避免 `common` 依赖 user-service metric 命名。备选方案是在 common 内直接注册 Prometheus 指标，但会破坏共享 primitive 的边界并增加跨服务命名耦合。

### Decision: 配置面属于 user-service 私有 API 限流策略

`RateLimitPolicyConfig` 增加 `MaxKeys` 和容量耗尽策略字段，并在 `DefaultRateLimitPolicyConfig`、`Validate`、provider 构造和配置测试中同步。匿名策略默认容量应按公网认证入口预算设置为有限正数；认证策略也设置有限正数，避免大量 User ID 或测试租户造成同类风险。

选择服务私有配置，是因为不同服务公开入口和用户规模不同，容量预算不应固化在 `common`。备选方案是只在 common 设置默认无限/固定容量，但无法体现服务风险等级，也难以通过配置调整压测和生产预算。

## Risks / Trade-offs

- [Risk] 分片容量按 hash 分布，极端 Key 分布可能使单个 shard 提前进入 overflow。Mitigation: 默认 shard 数保持足够大，并用总容量换算分片预算；测试覆盖热点 shard 和总量边界。
- [Risk] overflow bucket 会让大量新 Key 共享同一 token bucket，容量耗尽时合法新客户端可能被更严格限流。Mitigation: 通过指标暴露 overflow 事件，允许运维提高容量或切换策略；该行为优先保护进程可用性。
- [Risk] fail-closed/reject 策略可能拒绝合法新 Key。Mitigation: 将其作为可配置高安全策略，不作为匿名入口默认策略，文档和配置校验明确语义。
- [Risk] 新指标标签如果包含原始 Key 会导致指标高基数。Mitigation: 所有日志/metrics 仅暴露固定 scope、event、reason 和 key_present，不记录原始 IP/User ID。

## Migration Plan

1. 扩展 common 限流器结构、选项、错误/事件语义和测试。
2. 扩展 user-service 限流配置默认值、校验和 provider 参数传递。
3. 扩展 router 限流观测 reason 和测试，必要时更新 Prometheus/Grafana 资产。
4. 运行相关 Go 测试、`go test -race` 覆盖限流包，并运行 `make user-service-architecture-lint` 验证结构边界。

回滚时可以把 user-service 容量配置调高以减少 overflow/reject 影响；代码回滚不涉及数据库或外部资源迁移。

## Open Questions

无。默认采用有限容量加 overflow bucket；实现阶段只需根据现有压测预算选择具体默认 `MaxKeys` 数值。
