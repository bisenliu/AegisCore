## Context

`common/runtime/localcache` 当前用 Ristretto 实现 bounded TTL loading cache。公开 API 暴露了 `KeyString`、`NumCounters`、`BufferItems`、`CloneFunc`、异步写入拒绝和 `Wait()` 可见性，统计与 Prometheus 也包含 admission、write buffer 和 singleflight 内部事件。这些细节已经渗透到 auth token version cache、RBAC user-role cache、provider 测试、dashboard、alert 和 runbook。

本次变更跨越 `common`、user-service feature 和 `deployments` 观测资产，但不改变 HTTP API、OpenAPI、Ent schema、Atlas migration 或外部安全决策。主要协作者是共享 runtime、auth、permission/RBAC 和 observability 的维护者。

## Goals / Non-Goals

**Goals:**

- 提供只表达短 TTL、最大 item 数、同 key 回源合并和明确生命周期的 `LoadingCache[K, V]`。
- 使用 `ttlcache/v3` 替换 Ristretto，同时不向调用方暴露底层 option、loader 或 metrics 类型。
- 将业务 key 类型选择和可变 value clone 责任放回消费 feature。
- 让成功写入立即可见，不再存在 admission rejected、set dropped 或 `Wait()` 契约。
- 只保留 requests、loads、automatic evictions 和 capacity 四类可行动指标。
- 通过类型和构造函数改名强制 auth、RBAC 与测试显式迁移，不保留旧兼容层。

**Non-Goals:**

- 不提供分布式缓存、一致性缓存、业务失效策略、业务 key 编码或通用 deep clone。
- 不提供按字节或自定义 cost 的容量策略，也不暴露 `ttlcache` option。
- 不保留独立 `Get`、主动 `Set`、可配置 sliding TTL 或业务预热入口；当前生产调用方没有这些需求。
- 不改变 token version 权威来源、RBAC fail-closed、policy sync、HTTP API、数据库结构或 OpenAPI。

## Decisions

### 1. 公开 API 收敛为 loading cache 最小集合

公开类型为 `LoadingCache[K comparable, V any]`，构造函数为 `NewLoadingCache(cfg Config, loader Loader[K, V])`。`Config` 只包含 `Name string`、`Capacity uint64`、`TTL time.Duration` 和 `LoadTimeout time.Duration`。实例只公开 `GetOrLoad`、`Delete`、`Clear`、`Close`、`Name` 与 `Stats`。

选择最小集合是因为生产 auth 与 RBAC 只需要读取回源和失效；删除 `Get` 与 `Set` 可避免把未来预热、写穿或读取统计歧义提前固化为共享契约。备选方案是保留 `Get(ctx, key)` 和 `Set(key, value) error`，但当前没有真实调用方，且会扩大关闭、统计和 value ownership 的稳定表面积，因此不采用。

### 2. `ttlcache/v3` 只作为未导出的 item store

内部 cache 使用 `ttlcache.WithTTL`、`ttlcache.WithCapacity` 与 `ttlcache.WithDisableTouchOnHit`。`Capacity` 明确定义为最大 item 数；达到上限时由 `ttlcache` 按其 LRU 容量策略移除条目。读取不延长过期时间，TTL 从成功写入时开始计算，保证固定短 TTL，而不是 sliding expiration。由于 `ttlcache.Start()` 不提供可靠的启动屏障，wrapper 自有清理 goroutine 按 TTL 调用 `DeleteExpired()`；`Close()` 通过停止 channel 和完成 channel 同步等待该 goroutine 退出，避免 constructor 与立即关闭之间的竞态。

不使用 `ttlcache` 自带 loader，因为其签名不能传播 `context.Context` 或 loader error；回源仍由 wrapper 控制。备选方案是直接暴露 `ttlcache` options 或使用其 `SuppressedLoader`，但这会泄露底层类型，并丢失当前错误与 context 契约，因此不采用。

### 3. 使用业务 key 原类型进行无碰撞 flight 合并

`ttlcache` 直接以 `K` 存储 item，不再字符串化 key。回源合并继续使用 `golang.org/x/sync/singleflight`，但不通过 `fmt.Sprint(key)` 生成 group key。wrapper 维护受 mutex 保护的 `map[K]*flightEntry`，每个活跃业务 key 对应独立 `singleflight.Group`，组内使用固定私有 key；引用计数在所有等待者离开后删除 entry。

这样既保留经验证的 `singleflight` 行为，也避免不同 key 字符串表示碰撞或永久 key registry。备选方案包括重新引入 `KeyString`、使用 `fmt.Sprint`、依赖 `ttlcache.SuppressedLoader` 或自行实现完整 generic singleflight；它们分别会泄露业务编码、存在碰撞、无法传播 error/context，或重复实现并发原语，因此不采用。

### 4. miss、loader context 与关闭并发语义

`GetOrLoad` 首次查找命中时递增 `Hit`；未命中时每个业务调用递增 `Miss`。进入同 key flight 后 leader 执行一次内部 double-check，但该命中不递增 `Hit`，也不导出单独指标。真正执行 loader 成功时递增 `LoadSuccess` 并写入 cache，失败时递增 `LoadError` 且不写入。

loader 使用保留 context values 但解除 caller cancellation 的 context，并始终应用正数 `LoadTimeout`。每个等待者通过自己的 `ctx.Done()` 与 flight result 竞争，因此 caller 取消只终止自身等待，不取消其他等待者共享的 loader。该行为保持现有防击穿安全性，且避免首个断开请求取消全部 follower。

实例用关闭状态和生命周期锁保护底层访问。`Close()` 幂等，阻止新的 `GetOrLoad`、`Delete` 与 `Clear` 并停止清理 goroutine；关闭后这些方法返回 `ErrClosed`。已经开始的 loader 可以在其 timeout 内完成，但关闭后不得再写入底层 cache；等待结果的 caller 仍可获得 loader 已产生的值或自身 context error。`Name()` 和 `Stats()` 在关闭后继续返回稳定快照。

### 5. value ownership 属于 feature

`LoadingCache` 只保证容器并发安全，不复制 `V`。不可变值可直接使用；slice、map、pointer 或含引用字段的 struct 必须由消费 feature 在写入前和返回调用方前防御性复制。

auth 的 `int64` 无需复制。RBAC 保留 `uuid.UUID` key；loader 将数据库结果 clone 后交给 cache，`RolesForUser` 再 clone 后返回，使数据库 adapter、cache 与授权调用方不共享 `[]uuid.UUID` backing array。disabled/direct 路径也返回同样的独立 slice 语义。备选方案是继续提供 `CloneFunc`，但它会让 common 持有业务 value 语义并增加每次 cache 操作的隐式成本，因此不采用。

### 6. Stats 与 Prometheus 使用 wrapper 稳定语义

`Stats` 只包含 `Hit`、`Miss`、`LoadSuccess`、`LoadError`、`Evicted` 和 `Capacity uint64`。前四项由 wrapper 原子计数，避免依赖底层 metrics 定义。`Evicted` 只统计 TTL 过期或容量达到上限造成的自动移除；显式 `Delete` 和 `Clear` 不计入 eviction，避免业务失效流量污染容量诊断。实现通过 `ttlcache.OnEviction` 按 reason 过滤，并在测试中使用可观察条件处理异步 callback。

Prometheus collector 继续导出：

- `aegiscore_localcache_requests_total{cache,result="hit|miss"}`
- `aegiscore_localcache_loads_total{cache,result="success|error"}`
- `aegiscore_localcache_evictions_total{cache}`
- `aegiscore_localcache_capacity{cache}`

success 直接读取 `LoadSuccess`，不再计算 `Load - LoadError`。删除 singleflight 和 writes family 后，同步删除 dashboard panel、alert、metrics load assertion 和 runbook 中的旧查询；不保留兼容 PromQL。

### 7. 配置与代码归属

`common/runtime/localcache` 拥有通用 API、实现、错误和统计；`common/runtime/observability/metrics` 只负责把稳定 `StatsSource` 转为 Prometheus。user-service auth 拥有 string token version key，permission/RBAC 拥有 UUID key、slice clone 和业务失效时机。不得把 auth/RBAC key schema、clone helper、Casbin 或 Ent 依赖放入 `common` 或 `internal/shared`。

当前服务配置只暴露 feature 的 `size`、`ttl` 与 `load_timeout`，没有公开 `num_counters` 或 `buffer_items`；因此本次只删除 localcache Go `Config` 中的旧字段及共享主规格中的旧校验要求，不新增配置兼容或部署配置迁移。

## Risks / Trade-offs

- [底层 LRU 与 Ristretto admission 策略不同，命中率可能变化] -> 明确 capacity 为最大 item 数，保留 hit/miss/load/eviction 指标，并在容量与并发测试中验证上限和回源行为。
- [固定 TTL 与 `ttlcache` 默认 touch-on-hit 不一致] -> 强制使用 `WithDisableTouchOnHit`，用重复读取后仍按首次写入时间过期的测试锁定语义。
- [eviction callback 异步导致统计短暂滞后] -> `Stats` 文档声明 eviction 最终可见，测试使用明确 deadline 的 eventually 断言；请求和 loader counter 保持同步可见。
- [Close 与在途 loader 竞态导致关闭后写入] -> loader 完成后在生命周期锁内再次检查关闭状态，关闭后跳过 cache write；并发关闭测试覆盖该路径。
- [删除 `Get`/`Set` 降低通用性] -> 当前无生产调用方；未来只有出现真实主动预热或 cache-only lookup 需求时，另行定义可测试的稳定契约。
- [删除旧 metric family 会使旧 dashboard 或 alert 查询失效] -> 源 dashboard、provisioning JSON、alert、load 脚本和 runbook 在同一 change 原子更新，并运行 dashboard drift 校验。
- [RBAC clone 下沉后遗漏某个返回边界会产生共享 slice] -> 在 loader 入缓存和 `RolesForUser` 出缓存两侧显式 clone，并增加修改返回 slice 不污染后续读取的 feature 测试。

## Migration Plan

1. 在 `common` module 引入 `github.com/jellydator/ttlcache/v3` 并删除 Ristretto，重写 localcache 类型、实现、错误和单元测试。
2. 更新共享 localcache Prometheus collector 与测试，只导出四组 metric family。
3. 迁移 auth token version cache、direct fallback stats、相关 fixture 与 Fx 测试到新 API。
4. 迁移 RBAC user-role cache，保留 `uuid.UUID` key，并把 clone 放入 loader 和 resolver 返回边界。
5. 更新 provider/health/Fx graph fake `StatsSource`，同步 dashboard source、provisioning JSON、alert、metrics load 脚本和 runbook。
6. 运行 `make common-test`、auth/permission 相关 package 测试、`make compose-dashboard-check`、`make user-service-architecture-lint`，最后运行 `make user-service-test` 和 `make verify`。

这是单仓库编译期 breaking change，common 与 user-service 必须在同一提交中迁移。回滚方式是整体回滚该提交及依赖锁文件和观测资产；不需要数据回滚、数据库 migration 或双版本运行策略。

## Open Questions

无。主动 `Set` 与独立 `Get` 明确不进入本次稳定契约；如未来出现真实预热需求，再通过独立 change 评估。
