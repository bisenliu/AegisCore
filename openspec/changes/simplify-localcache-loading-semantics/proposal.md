## Why

当前 `common/runtime/localcache` 同时维护 TTL 清理 goroutine、关闭状态、每个 key 的自建 flight 状态和底层 eviction 订阅，生命周期与并发路径复杂；更关键的是，显式失效无法阻止失效前已经开始的 loader 在之后返回或回填旧值，认证撤销与 RBAC 授权因此需要在 feature 内重复实现 generation 门禁。需要把业务中立的强失效语义收敛到共享 loading cache，并删除不再必要的生命周期与兼容负担。

## What Changes

- **BREAKING**：将 loading cache key 固定为 `string`，公开读取 API 改为 `Get`，移除泛型 key 参数、`GetOrLoad`、`Delete`、`Clear`、`Close`、`ErrClosed` 及其生命周期兼容行为。
- 使用单个 `singleflight.Group` 合并同 key 并发 miss；loader 接收保留 context values、与 caller cancellation 解耦且受 `LoadTimeout` 限制的 context，caller 取消只终止自身等待。
- 通过 cache-wide revision 与发布临界区实现 `Invalidate`、`InvalidateAll` 的强失效：失效前开始的 loader 不得在失效后返回旧值或回填，首次竞态透明重试，连续竞态返回明确的 `ErrInvalidated`。
- 保持固定 TTL 和强制容量上限，不启动后台清理；过期项读取时逻辑失效，并在成功写入前通过 `DeleteExpired` 惰性清理。
- auth token-version cache 直接使用新 API 和强失效语义，删除仅为 localcache 关闭状态存在的 adapter/resource 状态，并把撤销编排收敛为 Redis 投影更新前、更新后两次本地失效。
- permission user-role resolver 使用字符串化 UUID 与通用强失效语义，删除 feature 内 `userRoleCacheGeneration`、token 和 stale generation error，继续在 feature 边界复制角色 slice。
- **BREAKING**：将 localcache eviction 观测收窄为纯容量驱逐，使用 `aegiscore_localcache_capacity_evictions_total`，同步 collector、测试、Prometheus 告警、Grafana dashboard、生成资产和全部 PromQL/runbook 引用。
- 不保留旧 API、错误、统计字段或指标名兼容层，也不使用 `ttlcache.WithLoader`、`ttlcache.NewSuppressedLoader`、`ttlcache.Cache.Start` 或自建定时清理 goroutine。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 修改通用 loading cache 的 key、公开 API、singleflight、context、强失效、TTL 清理、容量和统计契约。
- `auth-session-management`: 修改 token-version 本地缓存的失效竞态与撤销编排，确保旧回源不能在撤销后返回或回填。
- `rbac-access-control`: 将 user-role cache 的强失效门禁下沉为业务中立 localcache 能力，删除 permission feature 自有 generation，并调整 key 与生命周期契约。
- `runtime-observability`: 将 localcache eviction 指标明确为仅容量驱逐，并同步稳定指标名与观测资产契约。

## Impact

- 共享代码：`common/runtime/localcache/` 的实现、公开 Go API、错误、统计与测试，以及 `common/runtime/observability/metrics/` 的 collector 和测试。
- Auth：`user-service/internal/features/auth/` 的 token-version cache composition、validator/invalidator port、撤销编排和并发安全测试。
- RBAC：`user-service/internal/features/permission/` 的 user-role resolver、Fx lifecycle、generation 门禁和授权并发测试。
- 观测资产：`deployments/observability/`、`deployments/compose/grafana/`、dashboard 生成脚本与相关文档中的 localcache PromQL 和说明。
- 依赖仍使用现有 `ttlcache` 与 `golang.org/x/sync/singleflight`，不引入数据库 schema、Atlas migration、HTTP API、OpenAPI 或部署拓扑变化。
- 安全语义收紧：token 撤销和 RBAC 绑定失效并发期间必须 fail-closed，或透明重试后只返回失效后的新值。
