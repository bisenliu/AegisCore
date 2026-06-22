## Why

当前 `common/runtime/localcache` 只提供无容量上限的短 TTL `sync.Map` 缓存，过期项依赖同 key 读取惰性删除。面对高基数用户 ID、token version 或 RBAC 授权热路径，单实例在流量放大时缺少可证明的内存边界，也缺少命中、回源、淘汰和拒绝准入指标。

项目目标是支撑 100+ REST API/gRPC 服务、单机 10k+ QPS 和 3 年以上长期维护，本地缓存需要升级为可配置容量、可观测、可防击穿的跨服务 runtime primitive。

## What Changes

- **BREAKING**：移除旧的 `localcache.New(ttl)` 兼容入口，调用方必须通过显式 `Config` 创建缓存实例。
- **BREAKING**：`localcache` 不再只暴露裸 `Get/Set/Delete` TTL map，而是提供基于 Ristretto v2 的 bounded TTL loading cache。
- 新增容量上限 `Capacity`、默认 TTL、回源超时、key string 编码、value clone、`GetOrLoad`、`Set`、`Delete`、`Clear`、`Stats` 和 `Close` 能力。
- 在 `localcache` 内封装 `singleflight`，统一处理同 key 并发 miss 的回源合并，默认不缓存 loader 错误。
- `localcache` 保持无 Fx、无 Prometheus 依赖；依赖注入、生命周期关闭和 metrics collector 由服务或 observability provider 组装。
- 将 auth token version 本地缓存和 RBAC user role resolver 本地缓存迁移到新的 bounded loading cache，并配置明确容量与 TTL。
- 增加低基数指标维度，支持观测 hit/miss、loader 调用、loader 错误、singleflight shared、double-check hit、准入拒绝、写入丢弃和淘汰。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `shared-platform-primitives`：`common/runtime/localcache` 从无容量短 TTL primitive 升级为 bounded TTL loading cache，并要求容量、过期、回源、关闭和统计语义明确。
- `auth-session-management`：受保护路由的 token version 本地校验缓存使用 bounded localcache，容量、TTL 和回源超时可治理，缓存错误不落地。
- `rbac-access-control`：授权热路径的用户角色本地缓存使用 bounded localcache，支持容量控制、主动失效、全量清空和 value clone 防污染。
- `runtime-observability`：新增本地缓存运行时指标，要求指标 label 保持低基数并可用于 SRE 观察命中率、回源率、淘汰率和准入拒绝率。

## Impact

- 代码影响：
  - `common/runtime/localcache/`
  - `common/runtime/config/`
  - `common/runtime/observability/metrics/`
  - `user-service/internal/features/auth/`
  - `user-service/internal/features/permission/infrastructure/casbin/`
  - `user-service/internal/providers/`
  - `user-service/configs/config.yaml`
- 依赖影响：`common` 模块新增 `github.com/dgraph-io/ristretto/v2` 依赖；`golang.org/x/sync/singleflight` 可由 `common/runtime/localcache` 直接使用。
- 配置影响：新增或调整本地缓存配置，明确 `auth_token_version` 与 `rbac_user_roles` 的容量、TTL 和回源超时；不保留旧 localcache 构造兼容行为。
- 观测影响：新增 localcache collector 和 Prometheus 指标；Grafana dashboard 可在后续 change 中补充面板，本次先保证 metrics 可采集。
- API 和数据库影响：不改变外部 HTTP API、OpenAPI、Ent schema 或 Atlas migration。
