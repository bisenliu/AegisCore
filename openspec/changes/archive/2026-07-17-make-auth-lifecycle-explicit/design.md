## Context

`auth-session-management` 已完成前置变更 `remove-fx-from-auth-adapters`，但 `user-service/internal/features/auth/infrastructure/redis/session_purge_pool.go` 仍通过 `SessionPurgePoolParams` 接收 `fx.Lifecycle`，并注入 `Redis *redis.Client \`name:"cache_redis"\`` 作为仅用于 Fx hook 排序的伪依赖。`user-service/internal/features/auth/fx.go` 中 token-version localcache 的构造也直接接收 `fx.Lifecycle` 并在构造器内注册 `OnStop`。

这些资源本质上属于 auth feature 自有运行时资源：session purge pool 拥有 goroutine 和 drain/stop 行为，token-version localcache 拥有进程内缓存和关闭状态；共享 Redis client 不属于 auth feature。实现应把资源所有权表达为普通 Go API，由 Fx module 在服务装配边界注册关闭 hook，并保证 auth 自有资源先于 Redis client 关闭。

## Goals / Non-Goals

**Goals:**

- 将 `NewSessionPurgePool` 改为普通构造函数，只接收 logger 等真实依赖，返回可显式 `Stop(context.Context) error` 的 pool 或等价接口。
- 删除 auth Redis infrastructure 正式代码中的 `fx.Lifecycle`、`go.uber.org/fx`、`go.uber.org/dig` 和 `name:"cache_redis"` ordering-only dependency。
- 将 token-version cache 构造结果改为统一对象，显式暴露 validator/cache、stats 和幂等 `Close`，enabled、disabled 和 direct 模式都遵守相同关闭契约。
- 由 `user-service/internal/features/auth/fx.go` 在装配边界登记 auth 自有资源的 `OnStop` hook，并利用 Fx 正常依赖图保证 purge pool 和 token-version cache 先于共享 Redis client 关闭。
- 增加或更新 auth 测试，覆盖幂等关闭、超时、drain、关闭顺序和 goroutine 不泄漏。

**Non-Goals:**

- 不实现顶层 Runtime 或新的跨服务生命周期框架。
- 不改变 session 删除、refresh rotation、token version 查询/失效、cache TTL/容量或 metrics 语义。
- 不关闭共享 Redis client；auth 组件只关闭自身 pool、queue、localcache 等自有资源。
- 不修改 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署清单或观测资产。
- 不把 auth 专属生命周期 helper 放入 `common`、`internal/shared` 或 `internal/integration`。

## Decisions

1. session purge pool 使用显式构造和显式停止。

   `NewSessionPurgePool` 改为接收真实依赖，例如 `*zap.Logger` 或小型 `Options`，并返回实现 `PurgeTaskPool` 且可显式停止的资源。停止行为继续委托 `workerpool.Pool.Stop(ctx)`，保持已有 stop timeout 和 drain 语义。

   备选方案是保留 `fx.Lifecycle` 参数但删除 Redis 伪依赖；该方案仍会让 infrastructure adapter 依赖 Fx，不能满足普通 Go 组件边界，因此不采用。

2. token-version localcache 封装为带关闭契约的构造结果。

   在 auth feature 内引入最小 result 类型或接口，暴露 `Cache authvalidators.LocalTokenVersionCache`、`Stats localcache.StatsSource` 和 `Close() error`。enabled 模式关闭真实 localcache；disabled/direct 模式的 `Close` 为幂等 no-op。`Close` 不关闭 Redis projection store 或 PostgreSQL store。

   备选方案是让 `TokenVersionValidator` 自身实现 `Close`；该方案会把 validator 语义和缓存资源所有权耦合，且 metrics wrapper 与 invalidator 输出会变复杂，因此不采用。

3. Fx 生命周期只留在 auth module 装配边界。

   `user-service/internal/features/auth/fx.go` 可以继续导入 Fx，因为它是 feature 装配层。它负责把普通 Go 资源转换为 Fx provider，并在 `fx.Lifecycle` 中登记 `OnStop`。为确保关闭顺序，auth session store 保持对 purge pool 的真实依赖，auth module 的 provider 对 `cache_redis` 仍有真实 Redis 使用依赖；不再在 purge pool 构造器内注入 Redis。

   备选方案是构造自定义 stop aggregator；该方案接近顶层 Runtime，不在本 change 范围内，因此不采用。

4. 测试以行为和边界为准。

   Redis infrastructure 测试验证 pool stop drain、stop timeout、重复 stop 不 panic、不泄漏 goroutine，并证明关闭顺序为 purge pool stop 完成后才允许 Redis close。auth module 测试验证新 API 被 Fx 登记，但不把 Fx 重新引入 infrastructure 正式代码。

## Risks / Trade-offs

- [Risk] Fx hook 顺序如果只靠注释或伪依赖，后续改动可能重新引入 Redis 先关的问题。→ Mitigation：用测试记录 stop 事件顺序，并让 session store 对 purge pool、Redis client 保持真实依赖关系。
- [Risk] direct/disabled cache 的 no-op `Close` 未统一，调用方可能需要 nil 判断。→ Mitigation：所有构造分支返回非 nil closer，`Close` 幂等且可重复调用。
- [Risk] localcache 关闭后 invalidation 可能返回 `localcache.ErrClosed` 并影响撤销路径。→ Mitigation：保持现有 invalidator 对 `ErrClosed` 的容忍语义，并增加重复关闭与关闭后调用测试。
- [Risk] 删除旧 Params API 会造成内部测试和 provider 大面积编译失败。→ Mitigation：先调整构造器签名，再同步更新 auth module 与 auth Redis tests，运行 auth 包测试闭环。
- [Trade-off] Fx module 仍包含生命周期 hook。→ 这是有意边界：Fx 只在装配层存在，infrastructure adapter 和 application validator 保持普通 Go API。

## Migration Plan

1. 更新 session purge pool 构造器和接口，删除旧 `SessionPurgePoolParams`。
2. 更新 auth Fx module provider，把 logger 作为真实依赖传入，并在 module 层登记 pool `OnStop`。
3. 引入 token-version localcache 构造结果的显式 `Close`，更新 enabled 和 disabled/direct 构造分支。
4. 更新 token-version validator、metrics 和 auth module 输出，保持外部注入的 validator/cache/stats 行为不变。
5. 更新 auth Redis infrastructure 与 auth Fx 测试，覆盖关闭顺序、幂等关闭、超时、drain 和 goroutine 泄漏。
6. 运行验收命令：`cd user-service && go test ./internal/features/auth/... -count=1`、指定 `rg` 检查、`openspec validate make-auth-lifecycle-explicit`、`make user-service-architecture-lint`。实现完成并暂存预期变更后再运行 `make lint` 和 `make verify`。

Rollback 方式：在未发布前回退本 change 代码与规格；发布后若发现生命周期回归，回退到上一版本镜像即可恢复旧 Fx lifecycle ordering 行为。本 change 不含数据迁移或外部契约变更，无需数据库或 API 回滚。

## Open Questions

- 无待决问题。当前范围和验收条件足以进入实现。
