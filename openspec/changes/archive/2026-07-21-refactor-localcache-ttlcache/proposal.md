## Why

当前 `common/runtime/localcache` 的公开契约泄露了 Ristretto 的准入、异步写入和调优细节，并让通用层承担业务 key 编码与 value clone 责任，导致 auth、RBAC 和观测代码持续依赖底层实现语义。需要通过一次有意不兼容的重构，将其收敛为短 TTL、bounded、可诊断且可替换底层实现的通用 loading cache primitive。

## What Changes

- **BREAKING** 将底层缓存从 Ristretto 替换为 `ttlcache/v3`，公开类型改为 `LoadingCache[K, V]`，构造函数改为 `NewLoadingCache`。
- **BREAKING** 将泛型 `Config[K]` 改为非泛型 `Config`，容量改为 `uint64` 最大 item 数，并删除 `KeyString`、`NumCounters` 和 `BufferItems`。
- **BREAKING** 删除 `CloneFunc`，可变 value 的防御性复制由 auth、RBAC 等 feature 在 loader 与读取边界负责；业务 key 类型或编码同样由 feature 自行决定。
- **BREAKING** 删除未被生产调用方使用的独立 `Get` 与主动 `Set`，以及 Ristretto 专属写入拒绝语义、`Wait()` 可见性依赖和 `ErrKeyStringRequired`；关闭后的 `GetOrLoad`、`Delete` 与 `Clear` 返回语义更新后的 `ErrClosed`，`Close` 保持幂等。
- 保留同 key miss 的 `singleflight` 回源合并、loader error 不缓存、独立 `LoadTimeout` 和调用方 context 取消语义，但不再把 shared 与 double-check 作为稳定指标契约。
- **BREAKING** 将 `Stats.Load` 改为 `Stats.LoadSuccess`，容量类型改为 `uint64`，并删除 `Shared`、`DoubleCheckHit`、`SetDropped` 和 `Rejected`。
- **BREAKING** Prometheus 本地缓存指标缩减为 requests、loads、evictions 和 capacity 四组，删除 `aegiscore_localcache_singleflight_total` 与 `aegiscore_localcache_writes_total`，并同步 collector、测试、dashboard、alert 和 metrics load 校验。
- 迁移 auth token version cache 与 RBAC user-role cache；RBAC 继续使用 `uuid.UUID` 业务 key，并把 `[]uuid.UUID` clone 下沉到 permission feature 边界。
- 移除通用 local cache 配置中的 Ristretto 调优字段及其校验，不提供旧 API、旧配置或旧指标兼容层。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 重定义 localcache 的公开 API、配置、容量、回源、value 所有权、统计和关闭契约，并移除通用 runtime 配置中的 Ristretto 专属字段。
- `runtime-observability`: 将本地缓存指标缩减为请求、回源、驱逐和 item 容量四组稳定低基数指标。
- `auth-session-management`: 迁移 token version cache 到新 loading cache 契约，并删除对准入拒绝语义的依赖。
- `rbac-access-control`: 将 user-role cache 迁移到新契约，并明确可变角色 ID slice 的防御性复制由 RBAC feature 负责。

## Impact

- 共享 API 与实现：`common/runtime/localcache/`、`common/runtime/config/` 及对应测试。
- 依赖变化：`common` module 删除 Ristretto 依赖并新增 `ttlcache/v3`；`singleflight` 继续保留。
- 观测契约：`common/runtime/observability/metrics/localcache.go`、user-service collector 注册与 fake `StatsSource`、Prometheus/Grafana/alert/metrics load 资产及测试。
- 业务调用方：auth token version cache、permission/RBAC user-role cache、direct fallback、Fx graph、health 与 provider 测试需要同步迁移。
- 这是内部 Go API、配置字段和 Prometheus metric family 的 breaking change；不改变 HTTP API、OpenAPI 或数据库 schema，也不新增兼容 wrapper。
