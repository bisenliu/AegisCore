## Context

`common/runtime/scheduler` 当前以 `robfig/cron/v3` 为触发内核，并提供任务级本地防重叠、调度器全局并发限制、全局 `skip/wait`、任务级分布式锁 `skip/wait`、Redis owner token 锁、重试退避与 jitter、锁自动续租、续租失败处理、任务 timeout、metrics、日志和优雅关闭。这些功能继续属于本次目标，不能因为当前没有生产任务注册者而被删除。

复杂度主要来自实现结构而不是功能数量：`runJob` 使用集中式 `jobRunState` 记录 local gate、global gate、lock、context cancel、renew goroutine、started 和 error 等状态，再由统一 cleanup 根据布尔标志逆序释放。该结构能工作，但每个新策略都扩大共享状态和 cleanup 组合；同时当前 completed/failed duration 从准入前开始计算，首次 shutdown 等待超时后再次调用会直接返回成功。

`robfig/cron` 已提供 parser、动态增删、`JobWrapper` chain、panic recovery、logger 和返回运行任务 drain context 的 `Stop`。本设计继续使用这些能力，并在其内构建一个不可导出的 AegisCore invocation pipeline。pipeline 不作为公共扩展框架，只用于让每个 stage 获得、释放和观测自己拥有的资源。

受影响边界：

- `common/runtime/scheduler/` 拥有 scheduler、并发 gate、通用 lock port、Redis lock/retry/renew adapter 和执行 pipeline。
- `common/runtime/observability/metrics/` 继续实现 scheduler metrics，不承载 lock 或任务业务逻辑。
- `deployments/observability/`、`deployments/compose/` 和 `docs/observability/` 保留现有 scheduler 失败与续租失败观测，并同步 duration 定义。
- `docs/openspec` 和 common 文档同步功能保持、不兼容 API 以及安全边界。
- `user-service` 当前没有 scheduler 注册者，本变更不接入 Fx graph，也不改变 RBAC revision 补偿、事务 outbox 或 feature 后台循环。

## Goals / Non-Goals

**Goals:**

- 完整保留现有 overlap、全局并发、锁等待、Redis 重试、自动续租和观测功能。
- 用内部 pipeline 与局部 `defer` 替代集中式资源状态机，使各阶段所有权和清理顺序可独立验证。
- 压缩可产生矛盾状态的公开配置，不保留旧 API 兼容层。
- 复用 `robfig/cron` 的 parser、chain、panic recovery、动态增删和 drain。
- 修正执行耗时起点和可重复 Stop 的关闭语义。
- 保持低基数 metrics、结构化日志和 Redis owner token 安全检查。

**Non-Goals:**

- 不删除或降级全局并发、分布式锁、重试、续租和 `ContinueOnFailure` 功能。
- 不新增 exactly-once、可靠投递、持久化 job store 或 fencing token 保证；现有 Redis lock 仍是带 owner token 的 lease。
- 不创建公开 middleware/plugin 框架，也不允许调用方改变安全关键 stage 顺序。
- 不把 scheduler 用于 RBAC outbox、policy sync、auth session 清理或 feature orchestration。
- 不新增 Fx provider，不升级或替换 `robfig/cron`/`go-redis`，不改变 HTTP API、数据库、OpenAPI 或部署拓扑。

## Decisions

### Decision: 保留功能并压缩公开配置状态

公开 API 以 `Config`、`Job`、`LockPolicy`、`RenewPolicy`、`RetryPolicy`、`Locker`、`Lock`、`RedisLocker`、`Metrics` 和 `Scheduler` 表达现有能力。核心任务模型调整为：

```go
type Job struct {
	Key          string
	Spec         string
	Timeout      time.Duration
	AllowOverlap bool
	Lock         *LockPolicy
	Task         func(context.Context) error
}

type LockPolicy struct {
	Key         string
	TTL         time.Duration
	WaitTimeout time.Duration
	Renew       *RenewPolicy
}

type RenewPolicy struct {
	Interval          time.Duration
	Timeout           time.Duration
	ContinueOnFailure bool
}
```

`Lock=nil` 表示不使用分布式锁；非 nil 表示启用。`WaitTimeout=0` 表示只尝试一次并在锁忙时跳过，正数表示在上限内按 `RetryPolicy` 等待。`Renew=nil` 表示不续租，非 nil 表示启用续租。这样保留现有所有策略，同时消除 `Enabled=false` 但填写锁字段、`Mode=skip` 但填写等待时间、`AutoRenew=false` 但填写续租字段等矛盾组合。

`Config` 继续持有 timezone、logger、locker、metrics、默认 lock TTL、全局并发上限和全局 `skip/wait` policy。`RedisLockerOptions` 与 `RetryPolicy` 继续持有 namespace、scope、初始/最大间隔、最大尝试次数和 jitter。

`Scheduler` 公开 `New`、`Add`、`Remove`、`Start` 和 `Stop`；任务按固定 key 删除，`Add` 只返回 error，不暴露底层 `cron.EntryID`。这是允许的不兼容 API 简化，不改变动态注册与删除功能。

备选方案是保留所有旧字段只重排文件。该方案不能减少无效状态和 validation 分支，因此不采用。另一个备选方案是删除 lock/retry/renew；这违反本次功能保持目标，不采用。

### Decision: 保持现有 cron 解析和时区功能

继续使用当前 `SecondOptional|Minute|Hour|Dom|Month|Dow|Descriptor` parser，保留标准五字段、可选 seconds、descriptors、scheduler timezone 和 `CRON_TZ` 功能。全局使用现有 cron logger adapter，并通过 `cron.WithChain(cron.Recover(...))` 统一记录未处理 panic 与 stack。

不直接使用 `cron.SkipIfStillRunning`，因为它没有 metrics callback，且资源释放依赖 wrapper 排序。AegisCore 保留自己的本地 overlap stage，以记录 `local_overlap` 并通过 `defer` 保证 panic 后释放。

### Decision: 使用内部 invocation pipeline 替代集中式 jobRunState

注册任务时构建不可变的内部 pipeline，执行顺序保持：

```text
triggered
→ local overlap gate
→ global concurrency gate
→ distributed lock acquire/retry
→ task context/timeout
→ optional lock renewal
→ started/result observation
→ task
→ reverse cleanup
```

内部使用最小的 `invocation` 传递 root/task context、cancel 和已获取 lock；使用不可导出的 `handler` 与 `middleware` 组合固定 stages。每个 stage 只拥有一种资源并用词法 `defer` 释放：local stage 归还 local token，global stage 归还并发 token，lock stage 解锁，context stage cancel，renew stage 停止并等待续租 goroutine，observation stage 记录结果。由调用栈自然形成逆序 cleanup，不再维护 `gateAcquired`、`globalGateAcquired`、`started` 等跨阶段布尔状态。

pipeline 顺序由 scheduler 内部构造，调用方不能插入或重排 middleware。早期 skip/error 由对应 stage 记录固定 metrics 和日志并终止后续调用；task error 沿调用链返回。panic 穿过所有局部 defer 完成 cleanup，observation stage 记录 failed 后重新 panic，最终由 `cron.Recover` 记录 stack。

备选方案是直接使用多个 `cron.JobWrapper`。其 `Job.Run()` 没有 context/error 通道，锁续租错误与 task error 难以组合，因此只在 cron 最外层使用 recovery，AegisCore 内部保留带 error 的最小 pipeline。

### Decision: 全局并发和等待策略保持不变

`MaxConcurrentJobs=0` 继续表示无限制，正数使用有界 channel semaphore。`GlobalConcurrencySkip` 继续做非阻塞 acquire 并记录 `global_concurrency_limit`；`GlobalConcurrencyWait` 继续等待配额或 scheduler root context 取消。local overlap 仍先于 global gate，因此默认不允许 overlap 的同一任务最多只有一个 invocation 进入全局等待。

允许 overlap 且使用 global wait 时，cron 每次触发仍可能形成等待 goroutine，这是现有 wait 功能的固有语义。本次不静默改变为 skip、不新增 pending queue，也不声称该组合有队列容量保证；文档明确高频调用方必须审慎配置。未来若要增加 pending 上限，应单独修改稳定行为。

### Decision: Redis lock、重试和续租保留并局部简化

`Locker.Acquire` 与 `Lock.Unlock/Renew` port 保持。`RedisLocker` 继续使用 `SET NX PX`、随机 owner token 和 owner 校验 Lua 完成 acquire、unlock 与 renew；Lua script 提升为复用的 package-level `redis.Script`，避免每次操作重复构造脚本文本。

等待锁时使用派生 deadline context 表达总 `WaitTimeout`，循环仍同时受 parent cancellation、最大尝试次数、指数退避上限和 jitter 控制。等待 deadline 到期表示锁忙并返回 `(nil,false,nil)`；parent context 取消或 Redis 命令失败返回 error。timer 每轮显式停止或耗尽，重试 delay 不超过剩余等待时间。

续租封装为 invocation 局部 guard：按默认或显式 interval/timeout 调用 `Renew`，失败时记录 `JobLockRenewFailed`；`ContinueOnFailure=false` 取消任务 context，`true` 保持任务继续。task 返回后先停止并等待 renew goroutine，再按现有规则将续租错误合并为最终失败，随后取消 context 并释放锁。unlock 继续使用独立有界 context，避免已取消 task context 阻止释放。

这些调整只改变代码组织，不删除重试字段、锁模式、续租策略或 owner 校验。现有 Redis lease 不提供 fencing/强杀语义；任务仍必须协作响应 context，多实例副作用仍应幂等。

### Decision: 修正 duration 与保持观测集合

`Metrics` 完整保留 `JobTriggered`、`JobStarted`、`JobCompleted`、`JobFailed`、`JobSkipped` 和 `JobLockRenewFailed`。稳定 event 继续包括 `triggered|started|completed|failed|skipped|lock_renew_failed`，skip reason 继续包括 `local_overlap|global_concurrency_limit|lock_busy|lock_error`，无 reason 使用 `none`。

唯一行为修正是 duration 从 `JobStarted` 紧邻的实际任务执行起点计算，不包含 local/global gate 或分布式锁等待；completed/failed histogram 因而表达真实执行耗时。trigger、skip、lock renew failure 计数、现有 Prometheus alert、Grafana dashboard 和 runbook 均保留。

### Decision: Stop 共享单一 drain 且 scheduler 不可重启

首次 `Stop(ctx)` 原子地进入 stopping：先调用底层 cron 停止新触发，再取消 scheduler root context，最后等待 cron 返回的运行任务 drain。所有 Stop 调用等待同一个内部 done 状态；调用方 context 超时时返回包装后的 context error，但不终止实际 drain，后续 Stop 可继续等待。drain 完成后状态转为 stopped。

`Start` 在 running 状态重复调用保持幂等；进入 stopping/stopped 后不能重新启动或新增任务。`Remove` 只阻止未来触发，不终止已运行实例。该状态机只管理 scheduler lifecycle，不再参与单次 invocation 资源 cleanup。

### Decision: 所有权与跨模块同步

所有 scheduler、lock、retry 和 renewal primitive 留在 `common/runtime/scheduler`，不得移入 `user-service/internal/shared`、feature、integration 或 observability adapter。Prometheus collector 留在 `common/runtime/observability/metrics`，alert/dashboard/runbook 留在 deployments/docs。业务 schedule、任务 key、幂等和副作用规则由实际服务或 feature 拥有，common 不读取服务私有配置。

本变更不触碰 `user-service/internal/shared`、`internal/integration`、RBAC policy sync、Ent schema、Atlas migration、OpenAPI、HTTP route 或 Docker/Kubernetes/Helm 清单。

## Risks / Trade-offs

- [Risk] 内部 middleware 抽象可能只是移动复杂度。 -> Mitigation：pipeline 不导出、不支持任意插入，每个 stage 必须只拥有一种资源并以单元测试验证 acquire/defer/release；若实现不能减少共享状态和 cleanup 分支则不接受该重构。
- [Risk] 不兼容配置会影响仓库外调用方。 -> Mitigation：仓库内无生产注册者且用户明确不要求兼容；文档提供新结构，不增加双 API。
- [Risk] `AllowOverlap=true` 与 global/lock wait 仍可能积累 goroutine。 -> Mitigation：保持既有功能语义并明确配置风险；本次不引入隐式队列或功能裁剪。
- [Risk] 续租失败后任务可能不响应取消。 -> Mitigation：保持现有 cooperative cancellation 说明、续租失败指标和告警；不夸大 Redis lease 为 fencing 或强制互斥。
- [Risk] pipeline 中 panic 或早期返回导致资源泄漏。 -> Mitigation：所有资源在获取成功后立即注册局部 defer，并用 panic、error、skip、shutdown 和 race 测试覆盖逆序 cleanup。
- [Trade-off] 保留全部功能意味着代码量不会缩减到单纯 cron wrapper 的规模。 -> Mitigation：成功标准是减少状态组合、共享可变状态和重复 cleanup，而不是追求最少行数。

## Migration Plan

1. 先更新 OpenSpec delta，锁定功能保持、配置压缩、pipeline 顺序、duration 和 Stop 语义。
2. 重构 scheduler 公共模型与 lifecycle，构建内部 pipeline 并迁移所有现有执行策略。
3. 简化 Redis retry/Lua 与 renewal guard，保持原有接口能力和观测事件。
4. 迁移并扩展单元测试，逐项证明 overlap、global gate、lock wait/retry、renew、panic cleanup 和 shutdown 行为未丢失。
5. 同步 common、架构和观测文档，运行 OpenSpec、race、metrics、dashboard、architecture lint、`make lint` 和 `make verify`。

回滚时整体回退该 change 的代码、规格和文档。没有数据库、持久化任务或生产 scheduler 注册者，不需要数据迁移、双写或发布顺序；现有 metrics/alerts 未删除，因此无需观测迁移。

## Open Questions

无。实现不得以“简化”为由删除 proposal 明确保留的功能；新的 pending queue、fencing 或可靠任务能力必须另行提出 change。
