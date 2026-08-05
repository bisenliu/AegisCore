## MODIFIED Requirements

### Requirement: Runtime 执行与装配原语

系统 MUST 在 `common/runtime` 中提供业务中立的 ID、scheduler、workerpool、localcache、Redis key、timezone、logger、Fx provider 和依赖图原语。拥有后台执行的 primitive MUST 具有明确的容量、并发、失败处理、观测和关闭语义；localcache MUST 不拥有后台执行或关闭生命周期。构造函数、provider 和 Fx graph helper MUST 只消费真实运行时依赖或调用方显式提供的无副作用 Fx option，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。公开 provider 名称 MUST 表达其 runtime 能力或资源职责，MUST NOT 仅用模糊的 DI framework 术语隐藏能力语义。

#### Scenario: workerpool 与 scheduler 生命周期

- **WHEN** 调用方通过 `workerpool.New` 创建任务池并通过 `Stop(ctx)` 关闭
- **THEN** task pool MUST 作为不依赖 Fx 的普通 Go 资源创建并由拥有者显式关闭；Stop MUST 停止接收新任务、等待已登记或已接受任务 drain，并允许重复调用共享同一 drain 状态
- **AND** Stop 超时 MUST 返回包装 `context.DeadlineExceeded` 的错误，workerpool MUST NOT 承载 refresh session、token version、可靠消息、eventbus、outbox 或业务一致性语义
- **WHEN** scheduler 触发已注册任务
- **THEN** 系统 MUST 按本地 overlap gate、全局并发 gate、可选分布式锁、任务 context、可选锁续租、任务执行和 cleanup 的顺序处理，并 MUST 记录跳过、开始、完成、失败、拒绝和 panic，在 shutdown 时优雅停止
- **AND** 多实例副作用任务 MUST 声明正数 TTL 的分布式锁策略，长任务 SHOULD 使用续租
- **AND** 即使任务未配置 timeout，scheduler MUST 创建可取消 context，并在自动续租失败时取消任务和记录失败

#### Scenario: loading cache 构造、读取与回源

- **WHEN** 服务通过 `NewLoadingCache` 创建 loading cache
- **THEN** 配置 MUST 只包含非空名称、正数 `uint64` 容量、正数固定 TTL 和正数 load timeout，并 MUST 提供 `Loader[V] func(context.Context, string) (V, error)`
- **AND** 容量 MUST 表示最大 item 数，不得表示字节、自定义 cost 或 Ristretto admission 参数；cache key MUST 为 `string`，value MUST 保持泛型
- **AND** 公开 API MUST 只提供 `NewLoadingCache`、`Get`、`Invalidate`、`InvalidateAll`、`Name` 和 `Stats`，MUST NOT 暴露底层 `ttlcache` 配置、主动 `Set`、`CloneFunc`、写入拒绝或关闭语义
- **WHEN** `Get` 命中未过期 item
- **THEN** cache MUST 返回该值并记录一次 hit，且读取 MUST NOT 延长该 item 的固定 TTL
- **WHEN** `Get` 未命中
- **THEN** cache MUST 为该公开调用记录一次 miss，并使用单个 `singleflight.Group` 按 string key 合并同 key 并发回源；loader 成功 MUST 记录 `LoadSuccess` 并同步写入 bounded TTL cache，失败 MUST 记录 `LoadError` 且不得缓存错误结果
- **AND** 内部 double-check 与 invalidation retry MUST NOT 增加业务 hit 或 miss，也 MUST NOT 成为公开统计字段
- **WHEN** 同 key 回源正在执行且任一 caller context 被取消
- **THEN** 该 caller MUST 立即返回其 context error，MUST NOT 因自身取消而终止其他等待者共享的 loader，也 MUST NOT 启动等待 loader 完成的 drain goroutine
- **AND** loader context MUST 保留发起请求的 context values、通过 `context.WithoutCancel` 解除 caller cancellation，并通过 `context.WithTimeout` 受 `LoadTimeout` 限制；loader MUST 协作遵守该 context
- **WHEN** 不同 key 同时 miss
- **THEN** cache MUST 允许不同 key 的 loader 并行执行，不得用全局回源锁串行化 loader 本体

#### Scenario: loading cache 强失效

- **WHEN** loader 开始前
- **THEN** cache MUST 在发布锁下记录当前 cache-wide revision
- **WHEN** loader 成功后准备返回或写入
- **THEN** cache MUST 在同一发布锁下比较当前 revision 与开始 revision；一致时 MUST 先执行 `DeleteExpired` 再以固定默认 TTL 写入，不一致时 MUST 禁止返回该旧值且 MUST 禁止写入
- **WHEN** 调用方执行 `Invalidate(key)`
- **THEN** cache MUST 在发布锁下先递增 cache-wide revision，再删除指定 key，并在方法返回时完成失效
- **WHEN** 调用方执行 `InvalidateAll()`
- **THEN** cache MUST 在发布锁下先递增 cache-wide revision，再删除全部 item，并在方法返回时完成失效
- **AND** 单 key 失效 MAY 抑制其他 key 的在途 loader，cache MUST NOT 为此维护 per-key revision map
- **WHEN** 一个公开 `Get` 的回源结果因 revision 变化被抑制
- **THEN** cache MUST 透明重试一次且不得增加请求 miss；第二次仍被失效时 MUST 返回 `ErrInvalidated`
- **AND** 被失效抑制的旧值在任何情况下 MUST NOT 返回给 caller 或写入 cache

#### Scenario: loading cache TTL、容量、统计与值所有权

- **WHEN** cache 存储或返回 slice、map、pointer 或包含引用字段的 value
- **THEN** common MUST NOT 执行业务 deep clone，消费 feature MUST 在 loader 写入和返回调用方的适当边界复制可变 value
- **WHEN** cache 运行
- **THEN** cache MUST 使用固定 TTL、强制正数容量和命中不 touch 的策略，MUST NOT 使用 `ttlcache.WithLoader`、`ttlcache.NewSuppressedLoader` 或 `ttlcache.Cache.Start`，也 MUST NOT 创建定时清理 goroutine
- **AND** 过期 item MUST 在读取时逻辑失效，并在成功写入前通过 `DeleteExpired` 惰性清理；物理 item 数 MUST 始终受配置容量约束
- **WHEN** 达到最大 item 数并发生 `EvictionReasonCapacityReached`
- **THEN** `Stats.CapacityEvictions` MUST 增加，`Stats.Capacity` MUST 返回配置容量
- **WHEN** item 因 TTL 到期、`Invalidate` 或 `InvalidateAll` 被移除
- **THEN** `Stats.CapacityEvictions` MUST NOT 增加
- **AND** `Stats` MUST 使用请求级手工计数并包含 `Hit`、`Miss`、`LoadSuccess`、`LoadError`、`CapacityEvictions` 和 `Capacity`，MUST NOT 直接导出 `ttlcache.Metrics()`

