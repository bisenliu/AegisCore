## Context

`common/runtime/localcache` 当前实现为 `sync.Map` 加固定 TTL entry，构造函数只接收 `ttl`，不提供容量上限、主动清理、回源合并或稳定统计。它已被 auth token version validator 和 RBAC user role resolver 用在认证授权热路径，当前 TTL 很短，但高基数有效用户或流量放大时仍可能在进程内持续累积不可控 key。

AegisCore 的长期目标包括 100+ REST API/gRPC 服务、单机 10k+ QPS、Kubernetes 云原生部署和 3 年以上频繁迭代。作为跨服务 runtime primitive，localcache 需要从“短 TTL helper”升级为“有明确内存边界、回源保护、生命周期和观测指标的本地缓存底座”。

## Goals / Non-Goals

**Goals:**

- 通过 Ristretto v2 提供 bounded TTL cache，默认以 `cost=1` 和 `IgnoreInternalCost=true` 将 `Capacity` 解释为最大条目预算。
- 在 `common/runtime/localcache` 内提供 `Get`、`GetOrLoad`、`Set`、`Delete`、`Clear`、`Stats` 和 `Close`。
- 在 `GetOrLoad` 内封装 `singleflight`，合并同 key 并发 miss，默认不缓存 loader 错误。
- 通过 `CloneFunc` 隔离 loader 返回对象、缓存对象和调用方对象，避免 slice/map/pointer value 被调用方污染。
- 通过 `LoadTimeout` 为回源路径提供独立上限，并在注释中明确回源上下文会解除请求取消信号，避免 singleflight leader 取消导致 follower 全部失败。
- 将 auth token version 和 RBAC user role resolver 迁移到新 localcache，不保留旧 `localcache.New(ttl)` 兼容入口。
- 提供 localcache 统计快照和 Prometheus collector，指标 label 只使用固定缓存名和枚举值。

**Non-Goals:**

- 不新增 HTTP API、OpenAPI 文档、数据库 schema 或 Atlas migration。
- 不引入 eventbus、outbox、MQ、外部 gRPC client 或新的 integration 边界。
- 不在 `common/runtime/localcache` 直接依赖 Fx、Gin、Ent、Redis、Prometheus 或 user-service feature 包。
- 不在第一版实现 stale-while-revalidate；该语义涉及 stale TTL、后台刷新和错误保旧值，可作为后续单独 wrapper 或 option 演进。

## Decisions

1. 使用 Ristretto v2 作为默认底层，而不是手写 LRU、`expirable` 或现有 `/Users/liubisen/Desktop/sander/Project/common/cache/v2`。

   - Ristretto 面向高并发大容量缓存，提供 TinyLFU admission、Sampled LFU eviction、TTL、容量预算、`OnReject` 和 `OnEvict`。
   - `expirable` 行为更确定，适合当前小缓存，但长期作为 100+ 服务共享 primitive 时需要自行补充更多高并发和指标能力。
   - 现有 `cache/v2` 有 loader、singleflight 和 janitor，但底层 TTL map 没有容量上限，不能解决本次风险。
   - Ristretto 的淘汰不是精确 LRU，业务 MUST 将本地缓存视为优化层；miss、准入拒绝和淘汰后 MUST 能回源恢复。

2. `common/runtime/localcache` 保持无 Fx、无 Prometheus 依赖。

   - 核心包只暴露稳定 API 和 `Stats()`。
   - Fx provider 单独放在 user-service feature/provider 组装层，负责实例配置、loader 注入和 `Close` 生命周期。
   - Prometheus collector 放在 `common/runtime/observability/metrics`，读取 `StatsProvider`，避免 localcache primitive 反向依赖观测实现。

3. `Capacity` 而不是 `MaxEntries` 作为配置字段。

   - 第一版使用 `cost=1`，`Capacity` 表示最大条目预算。
   - 命名避免未来扩展按字节或权重 cost 时误导使用者。
   - `NumCounters` 默认使用 `Capacity * 10`，但允许显式覆盖；这是推荐默认，不作为不可调整铁律。

4. `Get` 统计业务 hit/miss，内部 double-check 使用 `lookup` 不污染 hit ratio。

   - `GetOrLoad` 首次 lookup 记 hit/miss。
   - `singleflight` 内部 double-check 命中只记 `DoubleCheckHit`。
   - `Shared` 统计 singleflight shared result，作为防击穿效果观察指标，不混入业务命中率。

5. `GetOrLoad` 在回源时使用独立 timeout。

   - 当 `LoadTimeout > 0` 时，loader 使用 `context.WithTimeout(context.WithoutCancel(ctx), LoadTimeout)`。
   - 这样避免 leader 请求取消导致所有 follower 共享失败，同时仍限制 DB/Redis 回源最长时间。
   - 当 `LoadTimeout <= 0` 时保留传入 ctx，不强行改变旧调用方可控的取消语义。

6. `Close` 后拒绝新请求。

   - `GetOrLoad` 和 `Set` MUST 返回 `ErrClosed`。
   - `Get` MUST 返回 miss。
   - `Delete` 和 `Clear` MUST 成为 no-op，避免服务停止时继续触碰底层缓存。

## Risks / Trade-offs

- Ristretto `SetWithTTL` 写入是异步且可能被 admission policy 拒绝 → `SetDropped`、`Rejected` 和 `Evicted` 指标用于观察，业务必须把 cache 当优化层并支持回源。
- TinyLFU 不是精确 LRU，低 QPS 下也不能假设最近访问项一定保留 → 对 auth/RBAC 只缓存可回源投影，不承载授权正确性的唯一来源。
- `context.WithoutCancel` 解除请求取消可能延长 loader 执行 → 必须配置 `LoadTimeout`，且 DB/Redis 客户端仍应有自身超时。
- clone 大对象会增加 CPU 和分配 → 对可变 value 必须保留 clone；对不可变值可传 nil 使用原样返回。
- 新依赖增加供应链和升级维护成本 → 通过 common 封装隔离业务代码，后续替换底层时不影响 feature API。

## Migration Plan

1. 创建 OpenSpec delta 并完成 apply-ready artifacts。
2. 在 `common` 模块新增 Ristretto v2 依赖。
3. 重写 `common/runtime/localcache`，删除旧 `New(ttl)` API 和 `sync.Map` 实现，不保留兼容代码。
4. 增加 localcache 单元测试，覆盖容量、TTL、GetOrLoad singleflight、错误不缓存、Set、Delete、Clear、Close、clone 和统计。
5. 扩展 `common/runtime/config`，加入服务本地缓存配置并校验容量、TTL 和 load timeout。
6. 在 auth token version validator 和 RBAC user role resolver 中注入新的 loading cache，并删除各自手写 `singleflight.Group`。
7. 新增 localcache metrics collector 并注册到 user-service runtime dependency metrics。
8. 更新 `user-service/configs/config.yaml` 和相关测试配置。
9. 运行 `make user-service-architecture-lint`、相关包测试和必要的 `make verify`。

回滚策略：本变更不涉及数据库、OpenAPI 或外部 API。若新缓存引发运行时异常，可通过代码回滚回旧提交并重新部署；配置容量可在不改外部契约的情况下调低或调高。由于不保留兼容代码，回滚需要随代码一起回滚调用点。

## Open Questions

- 第一版默认容量建议：`auth_token_version` 和 `rbac_user_roles` 暂定 100000，后续可根据压测和生产指标调整。
- Grafana dashboard 是否在本 change 同步添加 localcache 面板：本次优先提供 Prometheus 指标，dashboard 面板可作为后续 observability change。
