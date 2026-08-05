## MODIFIED Requirements

### Requirement: Runtime 执行与装配原语

系统 MUST 在 `common/runtime` 中提供业务中立的 ID、scheduler、workerpool、localcache、Redis key、timezone、logger、Fx provider 和依赖图原语。拥有后台执行的 primitive MUST 具有明确的容量、并发、失败处理、观测和关闭语义；localcache MUST NOT 拥有后台执行或关闭生命周期。构造函数、provider 和 Fx graph helper MUST 只消费真实运行时依赖或调用方显式提供的无副作用 Fx option，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。公开 provider 名称 MUST 表达其 runtime 能力或资源职责，MUST NOT 仅用模糊的 DI framework 术语隐藏能力语义。

#### Scenario: workerpool 生命周期

- **WHEN** 调用方通过 `workerpool.New` 创建任务池并通过 `Stop(ctx)` 关闭
- **THEN** task pool MUST 作为不依赖 Fx 的普通 Go 资源创建并由拥有者显式关闭；Stop MUST 停止接收新任务、等待已登记或已接受任务 drain，并允许重复调用共享同一 drain 状态
- **AND** Stop 超时 MUST 返回包装 `context.DeadlineExceeded` 的错误，workerpool MUST NOT 承载 refresh session、token version、可靠消息、eventbus、outbox 或业务一致性语义

#### Scenario: Scheduler 注册与配置状态

- **WHEN** 调用方注册 scheduler job
- **THEN** job MUST 提供裁剪后非空且唯一的固定 key、有效 spec、非 nil `func(context.Context) error` task 和非负 timeout
- **AND** scheduler MUST 接受标准五字段、可选 seconds、descriptors、scheduler timezone 和 `CRON_TZ`，并 MUST 支持按固定 key 删除任务而不向调用方暴露底层 `cron.EntryID`
- **AND** nil lock policy MUST 表示不启用分布式锁，非 nil lock policy MUST 表示启用；零 `WaitTimeout` MUST 表示单次尝试，正数 MUST 表示在该上限内等待
- **AND** nil renew policy MUST 表示不续租，非 nil renew policy MUST 表示启用续租；公开配置 MUST NOT 同时保留可与 nil/non-nil 或 timeout 语义冲突的 lock enabled、lock mode 或 auto renew 开关

#### Scenario: Scheduler 分阶段执行与本地重叠

- **WHEN** scheduler 触发已注册任务
- **THEN** 系统 MUST 按 triggered、本地 overlap gate、全局并发 gate、可选分布式锁、任务 context、可选锁续租、started/result 观测、任务执行和逆序 cleanup 的顺序处理
- **AND** 每个内部 stage MUST 只释放自己成功取得的资源，error、panic、skip 和 cancellation MUST NOT 泄漏 local/global token、lock、context cancel 或 renew goroutine
- **AND** `AllowOverlap=false` 时同 key 前一实例仍运行的触发 MUST 立即跳过并记录 `local_overlap`，`AllowOverlap=true` 时不同触发 MAY 并行执行
- **AND** 即使任务未配置 timeout，scheduler MUST 创建可由 shutdown 取消的 context；配置 timeout 时 MUST 创建受 timeout 限制的 context
- **AND** 正常返回 MUST 记录 completed，返回 error 或 panic MUST 记录 failed，panic MUST 由 `robfig/cron` recovery 记录且不得永久占用已获取资源
- **AND** completed/failed duration MUST 从 started 时刻计算，MUST NOT 包含 overlap、全局并发或锁等待时间

#### Scenario: Scheduler 全局并发策略

- **WHEN** `MaxConcurrentJobs` 为零
- **THEN** scheduler MUST 不施加全局并发限制
- **WHEN** 并发上限已满且 policy 为 skip
- **THEN** 当前触发 MUST 立即跳过并记录 `global_concurrency_limit`
- **WHEN** 并发上限已满且 policy 为 wait
- **THEN** 当前触发 MUST 等待配额或 scheduler root context 取消，取得配额后 MUST 继续执行并在结束时释放
- **AND** scheduler MUST 保留 `AllowOverlap=true` 与 wait 组合的既有等待语义，文档 MUST 明确高频触发可能产生等待 goroutine，不得静默改为 skip 或无界持久化队列

#### Scenario: Scheduler 分布式锁、重试与续租

- **WHEN** job 配置分布式锁且锁未被其他 owner 持有
- **THEN** scheduler MUST 通过 `Locker` 使用正数 TTL 获取 owner lock，并在任务退出路径使用独立有界 context 释放
- **WHEN** lock `WaitTimeout` 为零且锁忙
- **THEN** 本轮 MUST 跳过并记录 `lock_busy`
- **WHEN** lock `WaitTimeout` 为正且锁忙
- **THEN** Redis locker MUST 在总等待上限内按 initial/max interval、最大尝试次数和可选 jitter 重试，等待到期或尝试耗尽 MUST 返回 lock busy，parent context 取消或 Redis 错误 MUST 返回 error
- **AND** Redis lock MUST 使用随机 owner token 与 Lua owner 校验执行 unlock/renew，非 owner 操作 MUST 返回 `ErrLockNotOwned`
- **WHEN** job 启用自动续租
- **THEN** scheduler MUST 按有效 interval 和 operation timeout 刷新 TTL，并在任务结束前停止且等待 renew goroutine
- **WHEN** 续租失败
- **THEN** scheduler MUST 记录 `lock_renew_failed`；`ContinueOnFailure=false` 时 MUST 取消任务 context，`true` 时 MUST 允许任务继续，最终任务结果 MUST 保留续租失败语义
- **AND** 多实例副作用任务 MUST 使用正数 TTL，执行时间可能超过 TTL 的任务 MUST 使用续租并协作响应 context；Redis lease MUST NOT 被描述为 exactly-once、fencing 或 goroutine 强杀保证

#### Scenario: Scheduler 关闭

- **WHEN** 调用方首次执行 `Stop(ctx)`
- **THEN** scheduler MUST 先停止新 cron 触发，再取消活动任务及 gate/lock wait context，并等待已触发任务 drain；进入 stopping 后 MUST 拒绝新增任务和重新启动
- **WHEN** Stop 的调用方 context 在 drain 完成前取消或超时
- **THEN** Stop MUST 返回包装后的 context error，实际 drain MUST 继续，后续 Stop MUST 继续等待同一个完成状态而不得提前返回成功
- **AND** 多次 Start 在 running 状态 MUST 幂等，多次 Stop MUST 共享单一 drain，任务不协作响应 context 时 scheduler MUST NOT 声称能够强杀 goroutine

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

#### Scenario: Redis key、timezone 与 logger 归属

- **WHEN** feature 需要 refresh session、token version、RBAC 或其他业务 Redis key
- **THEN** feature infrastructure MUST 拥有业务 key schema，并只能复用 `common/runtime/rediskey` 的通用构造规则
- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 优先使用平台 `TZ` 环境变量并在缺省时使用稳定默认值，MUST NOT 依赖核心 Config 或服务业务配置
- **AND** 如果通过 Fx 初始化，服务 composition root MUST 显式绑定初始化调用或服务级 runtime 初始化函数，common MUST NOT 仅为了包装 `Init` 暴露无额外运行时职责的 Fx provider
- **WHEN** 调用方通过 `logger.New`、`NewWithConfig` 或 Fx provider `NewLogger` 创建 logger
- **THEN** 系统 MUST 返回由调用方拥有的 logger，Fx provider MUST 注册既有 Sync 关闭 hook；构造过程 MUST NOT 隐式安装、覆盖或恢复进程级默认 logger
- **AND** 默认 logger 只能通过显式 `SetDefault` 修改，并 MAY 作为未注入 logger 时的兜底

#### Scenario: 共享 provider、fxgraph 与公开 API 边界

- **WHEN** 共享 provider 暴露依赖
- **THEN** provider MUST 只消费跨服务配置和 primitive，不得导入服务私有配置；公开命名 MUST 能区分 logger、metrics、tracing、datastore 或其他具体 runtime 能力，不得在多个 common 包中重复使用缺少能力语义的通用名称作为主要入口
- **WHEN** 服务将 Fx option 或 module 传入 `common/runtime/fxgraph`
- **THEN** helper MUST 输出稳定排序的 provider、invoke 和依赖关系图文本，只处理调用方显式传入的 graph-safe Fx option，MUST NOT 构造或要求服务私有配置、feature provider、Ent、Redis、PostgreSQL、OTLP 或 HTTP server 输入
- **AND** helper MUST NOT 通过服务完整 runtime module 间接执行生产 runtime `fx.Invoke`
- **WHEN** `common/runtime` 新增公开 constructor、method、option 或 hook
- **THEN** 入口 MUST 具有真实运行时职责或已定义的稳定共享契约；仅测试消费、暴露内部状态或绕过正常 lifecycle 的能力 MUST 留在包内、`_test.go` fixture 或 `common/testing`
- **AND** 仅包装另一个无参初始化函数且不提供额外资源、配置、错误处理、顺序控制或 lifecycle 语义的 Fx provider MUST NOT 作为 common 公开 API 新增或保留

### Requirement: 测试基础设施、Cluster fixture 与隔离

系统 MUST 在 `common/testing` 中提供可复用的 PostgreSQL 与 Redis Cluster 容器 fixture，并使用可重复、可观察且不污染生产 API 或进程全局状态的方式验证共享能力。

#### Scenario: 可重复且可观察的测试

- **WHEN** Go 测试需要真实 PostgreSQL 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers` 管理依赖生命周期，测试数据 MUST 使用稳定 fixture 或 feature-local builder，避免不可重复的随机输入
- **WHEN** 测试需要注入失败、固定返回、控制顺序或观察后台状态
- **THEN** 测试 MUST 使用消费侧最小接口、局部 fixture、通道或可观察状态，正式代码 MUST NOT 为测试新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter
- **WHEN** 测试验证缓存过期、workerpool drain、scheduler overlap/global gate/lock retry/renew/timeout/shutdown 或后台任务取消
- **THEN** 测试 MUST 使用通道、eventually-style 条件或其他可观察同步机制和明确 deadline，MUST NOT 只依赖固定 `time.Sleep` 判断状态已经变化

#### Scenario: 隔离进程级状态

- **WHEN** 测试必须修改默认 logger、`TZ`、`time.Local` 或包级初始化状态
- **THEN** 测试 MUST 在 package-local helper 中保存状态并通过 cleanup 恢复
- **AND** 环境变量 MUST 使用 `t.Setenv`，相关测试 MUST NOT 并行执行
- **AND** 非测试目标所需的日志捕获 MUST 使用 context logger 或局部 logger 注入

#### Scenario: 真实 Cluster 集成测试

- **WHEN** 模块容器测试 target 通过 `-args -aegiscore.testcontainers` 启用真实依赖测试
- **THEN** Redis Cluster 相关集成测试 MUST 实际连接 Cluster fixture 并执行 Cluster-sensitive Redis 命令
- **AND** Docker daemon、Cluster fixture 启动、slot 初始化或连接失败 MUST 使相关集成测试失败而不是静默跳过
- **AND** `common/testing/containers` 自身的 PostgreSQL 与 Redis 集成测试 MUST 包含在根 `make test-containers` 门禁中
