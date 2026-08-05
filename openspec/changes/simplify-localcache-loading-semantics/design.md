## Context

`common/runtime/localcache` 当前以 `LoadingCache[K, V]` 包装 `ttlcache`，除固定 TTL 和容量外，还自行维护按 key 的 `flightEntry` 引用计数、TTL 清理 goroutine、关闭状态与 eviction unsubscribe 生命周期。caller 取消时会额外启动 goroutine drain `DoChan` 结果。`Delete`/`Clear` 只删除已发布 item，不能阻止此前开始的 loader 在失效后返回并重新写入。

这个缺口直接影响两个安全敏感 consumer。auth 的 token-version validator 必须保证撤销后不再接受旧版本；permission 的 user-role resolver 已在 feature 内维护 per-user/global generation 来抑制旧角色集合回填。两处还为 localcache 的 `Close`/`ErrClosed` 维持 adapter、holder 状态和 Fx lifecycle。与此同时，当前 `Evicted` 同时统计 TTL 到期和容量驱逐，Prometheus 告警无法区分自然过期与容量压力。

受影响路径包括：

- `common/runtime/localcache/` 与 `common/runtime/observability/metrics/`
- `user-service/internal/features/auth/` 的 token-version cache、validator、撤销编排与测试
- `user-service/internal/features/permission/` 的 user-role resolver、composition/lifecycle 与测试
- `deployments/observability/`、`deployments/compose/grafana/`、dashboard 生成脚本和观测文档
- 本 change 的四份 capability delta

本变更不涉及 `common/security/auth`、`internal/shared`、`internal/integration`、Ent schema、Atlas migration、HTTP 路由或 OpenAPI 生成物。

## Goals / Non-Goals

**Goals:**

- 以更小的公开 API 和单个 `singleflight.Group` 提供固定 TTL、强制 item 容量、同 key 回源合并和 caller 独立取消。
- 让 `Invalidate`/`InvalidateAll` 与 loader 发布形成明确线性化顺序，失效前开始但未发布的旧 loader 不得在失效后返回或回填。
- 由通用、业务中立的 cache-wide revision 取代 auth/RBAC consumer 内重复的旧回源门禁。
- 让一个公开 `Get` 只统计一次 hit 或 miss，实际 loader 执行分别统计 success/error，容量驱逐与 TTL/显式失效严格分离。
- 删除 localcache 自有后台 goroutine、关闭状态以及 consumer 中只为这些机制存在的生命周期代码。

**Non-Goals:**

- 不提供旧 API、旧错误、旧统计字段或旧 Prometheus 指标名的兼容层。
- 不提供主动 `Set`、业务 value clone、per-key revision map、可选容量、动态 TTL、refresh-ahead 或分布式缓存一致性。
- 不使用 `ttlcache.WithLoader`、`ttlcache.NewSuppressedLoader` 或 `ttlcache.Cache.Start`，也不新增定时清理 goroutine。
- 不改变 Redis token-version 投影、PostgreSQL 主事实、Casbin policy revision 或多副本同步协议。

## Decisions

### Decision: 固定 string key 与最小公开 API

`Loader[V]` 固定为 `func(context.Context, string) (V, error)`，`LoadingCache` 仅保留 value 泛型。公开 API 为 `NewLoadingCache`、`Get`、`Invalidate`、`InvalidateAll`、`Name` 和 `Stats`。`Invalidate` 与 `InvalidateAll` 无失败返回，因为 cache 不再具有 closed 状态。

auth 在 validator 边界把 UUID 规范化为 string；permission 在调用 cache 时使用 `userID.String()`。角色 slice 的复制继续由 permission feature 的 `cloneRoleIDs` 负责。这样 common 获得稳定、可直接供 `singleflight` 使用的 key，而业务 ID 解析与可变 value 所有权仍留在 feature。

不保留 `GetOrLoad`、`Delete`、`Clear`、`Close`、泛型 key 或 key encoder：这些接口会延续旧语义与迁移分支，并使强失效和生命周期再次分叉。

### Decision: 单个 singleflight.Group 合并回源且 caller 独立取消

cache 内仅维护一个 `singleflight.Group`，直接以 string key 调用 `DoChan(key, fn)`。公开 `Get` 先读取 cache；miss 后只增加一次请求级 miss，再进入 singleflight。共享函数再次读取 cache，避免首个检查与注册 flight 之间的重复回源，但该 double-check 不增加 hit/miss。

实际 loader context 使用 `context.WithTimeout(context.WithoutCancel(ctx), LoadTimeout)`：保留首个发起 caller 的 context values，解除其 cancellation/deadline，并由 cache 配置施加协作式超时。每个 caller 通过 `select` 等待自己的 `ctx.Done()` 或 singleflight result；取消后直接返回，不 drain channel。`DoChan` 的 result channel 容量为 1，loader 可独立完成并通知其他等待者。

不继续维护每 key `flightEntry` 和引用计数，因为全局 group 已提供 key 隔离与结果发布，额外状态只用于旧的 drain/清理协议。

### Decision: cache-wide revision 线性化失效与发布

cache 增加 `publishMu sync.Mutex` 和 `revision uint64`。singleflight 共享函数在实际 loader 开始前于 `publishMu` 下读取 revision；loader 成功后再次获取同一锁。revision 未变化时，函数先调用 `DeleteExpired()`，再以 `ttlcache.DefaultTTL` 写入并返回值；revision 已变化时禁止写入并返回包内 invalidated 标记。

`Invalidate(key)` 在 `publishMu` 下递增 revision 后删除 key；`InvalidateAll()` 在同一临界区递增 revision 后执行 `DeleteAll()`。因此失效与 loader 发布具有单一线性化顺序：先发布的结果可被随后失效删除；失效先完成时，旧 revision loader 不能发布或返回其值。

采用 cache-wide revision 而不是 per-key map。单 key 失效会同时抑制其他 key 的在途 loader，这是有意的正确性优先取舍：失效低频，省去 revision map 的回收、锁顺序和 key 生命周期问题。revision 的 `uint64` 自然回绕不作为现实运行周期内的兼容语义。

### Decision: 失效竞态最多透明重试一次

公开 `Get` 收到包内 invalidated result 时重新执行一次 miss 回源流程，但不再次增加请求 miss。若第二次仍与失效并发，则向 caller 返回公开 `ErrInvalidated`。任何一次被 revision 抑制的 value 都不会返回或写入；auth validator 与 RBAC authorizer 对最终错误保持 fail-closed。

重试次数固定为一次，避免高频失效下请求无限循环并掩盖写压力。`ErrInvalidated` 是新契约，不与被删除的 `ErrClosed` 兼容。

### Decision: 无后台清理且只统计容量驱逐

底层继续使用 `ttlcache.New[string, V]`、固定 TTL、强制 capacity 和 `WithDisableTouchOnHit`。不调用 `Start`；过期 item 由 `Get` 逻辑判定为 miss，并在每次成功发布前调用 `DeleteExpired` 惰性清理。capacity 始终为正数，因此即使没有后台清理，物理 item 数仍有界。

cache 可保留底层 eviction callback，但无需 unsubscribe/Close 生命周期。callback 仅在 `EvictionReasonCapacityReached` 时增加 `CapacityEvictions`；TTL 到期、`Invalidate` 和 `InvalidateAll` 不增加该值。`Stats` 由手工原子计数生成，不直接导出 `ttlcache.Metrics()`。

一个公开 `Get` 只记一次 `Hit` 或 `Miss`。每次真实 loader 返回 nil error 增加 `LoadSuccess`，包括随后因 revision 变化而被抑制的成功值；loader 返回 error 增加 `LoadError`。内部 double-check 和 invalidation retry 不改变请求计数。

### Decision: auth 撤销只保留投影前后两次本地失效

token-version cache 直接实现 auth 消费侧读取/失效端口，UUID 在 validator 边界转为规范 string。删除 `localTokenVersionCacheAdapter`、localcache closed 状态与相应 resource/holder 分支；localcache 不注册 Fx close hook。

递增 PostgreSQL 主事实后，撤销编排在 Redis 投影更新前执行第一次 `Invalidate`，更新后执行第二次 `Invalidate`，封闭投影切换前后的回源窗口。删除 refresh sessions 后的第三次本地失效不再提供额外保证，予以移除。失效本身无 error；Redis 投影或 session 撤销失败仍返回既有撤销不完整错误。

### Decision: RBAC 删除 feature generation，保留值复制

user-role resolver 删除 `userRoleCacheGeneration`、generation token 和 stale generation error，`InvalidateUserRole`/`InvalidateAllUserRoles` 直接调用通用强失效 API。授权遇到连续 invalidation 或 loader 错误保持 fail-closed；一次 invalidation race 可由 localcache 透明重试并返回新角色集合。

`cloneRoleIDs` 仍在 loader 写入边界与 `RolesForUser` 返回边界执行。通用 revision 只表达发布有效性，不理解 UUID、角色集合或 RBAC policy revision，符合 `common/` 的业务中立边界。

### Decision: 指标显式命名为容量驱逐

`Stats.Evicted` 改为 `CapacityEvictions`，Prometheus 指标改为 `aegiscore_localcache_capacity_evictions_total`。collector help、单元测试、alerts、alert tests、Grafana 源 dashboard、Compose provisioning JSON、生成脚本、metrics load 校验和 runbook/PromQL 同步更新。

不保留 `aegiscore_localcache_evictions_total`，因为双写会让 dashboard 继续混淆旧口径，并扩大永久兼容面。

## Risks / Trade-offs

- [Risk] cache-wide revision 会让无关 key 的 loader 因单 key 失效而重试或失败 -> Mitigation：失效为低频写路径，只透明重试一次并以定向并发测试确认行为；若未来数据证明放大量不可接受，再独立提案 per-key revision。
- [Risk] loader 不遵守 context 会超过 `LoadTimeout` 占用 singleflight -> Mitigation：明确 timeout 为协作式约束，所有现有 loader 使用 context-aware Redis/PostgreSQL 调用，并用 timeout 测试固定契约。
- [Risk] 无后台清理会暂时保留过期对象 -> Mitigation：capacity 强制为正保证物理数量有界，读取逻辑不返回过期值，成功写入前惰性清理。
- [Risk] 首个 caller 提供的 context values 会被共享给同 key 其他 caller -> Mitigation：loader 不依赖身份授权或可变 request-local 控制值；只保留 tracing/logging values，并禁止以 caller cancellation 控制共享工作。
- [Risk] eviction callback 可能异步更新统计 -> Mitigation：测试使用最终一致断言，稳定契约只要求累计容量驱逐正确，不把即时 callback 时序暴露给调用方。
- [Risk] 指标重命名会造成升级瞬间的查询空窗 -> Mitigation：在同一发布单元同步 collector、alerts、dashboard 与 runbook；不双写旧指标，回滚必须整体回滚代码与观测资产。
- [Trade-off] 连续两次失效返回 `ErrInvalidated` 而非无限重试 -> 安全 consumer fail-closed，优先保证有界延迟和不返回旧值。

## Migration Plan

1. 先重构 `common/runtime/localcache` 及其 race tests，再更新 metrics collector 与指标测试。
2. 在同一代码变更内迁移 auth 和 permission consumer，删除旧 adapter、generation 与 cache lifecycle 接线，补齐 token-version 撤销和 user-role 失效并发测试。
3. 同步更新 Prometheus rules/tests、Grafana 源 dashboard、生成后的 Compose dashboard、metrics 校验和 runbook，运行对应生成与 drift 检查。
4. 运行定向 race tests、`make user-service-architecture-lint` 和 OpenSpec validate；将本次预期变更暂存后运行 `make lint` 与 `make verify`。
5. 无数据库、OpenAPI 或配置迁移。发布时应用代码和观测资产必须作为同一版本；回滚时整体回退两者，不保留 API 或指标双写。

## Open Questions

无。

## Verification

- `cd common && go test -race ./runtime/localcache ./runtime/observability/metrics`
- `cd user-service && go test -race ./internal/features/auth/... ./internal/features/permission/...`
- `make compose-dashboard-check`
- `make user-service-architecture-lint`
- `openspec validate simplify-localcache-loading-semantics`
- 预期变更暂存后运行 `make lint` 和 `make verify`
