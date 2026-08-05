// Package scheduler 提供进程内 cron 调度、并发准入、可选分布式锁、自动续租和优雅关闭。
//
// # 配置层级
//
// scheduler 有三个不同的配置层级：
//
//   - Config 控制 scheduler 时区、全局并发、logger、metrics，并提供可选 Locker。
//   - RedisLockerOptions.Retry 控制同一 Redis locker 的锁竞争重试退避。
//   - Job 控制单个任务的 cron、timeout、本地 overlap 和可选 LockPolicy；RenewPolicy 只作用于该 job。
//
// Config.TimeZone 为空时使用 time.Local，否则必须是 time.LoadLocation 可识别的时区。Spec 支持标准
// 五字段、可选 seconds、@hourly 等 descriptor 和 CRON_TZ 前缀。Job.Key 会裁剪首尾空白，必须是
// 唯一、固定、低基数的任务名，不能包含 user ID、请求 ID 或其他动态业务标识。四种 cron 格式的
// 可执行注册代码参见 ExampleScheduler_Add_cronFormats。
//
// New 创建未启动的 Scheduler。Add 可以在 Start 前后调用；Remove 只阻止后续触发，不中断已经开始的
// invocation。Start 在 running 状态幂等，Scheduler 一旦进入 stopping 就不能重新启动或新增任务。
// 基本注册、启动、删除和停止代码参见 ExampleScheduler。
//
// # Job 配置所有权
//
// Add 会复制 Job 的 LockPolicy 与 RenewPolicy，在 scheduler 自有副本上裁剪输入、填充默认值并校验。
// Add 不会修改调用方对象，调用方在 Add 返回后修改原 job 或嵌套策略也不会影响已注册任务。Task 函数
// 及其闭包捕获对象仍由调用方负责并发安全；scheduler 不会复制函数闭包中的业务状态。
//
// Timeout 为零表示不设置单任务 deadline，负数返回 ErrInvalidJob。即使 Timeout 为零，scheduler 也
// 会为任务创建可取消 context；Stop 或续租失败策略可以通知任务退出。取消只是协作信号，不会强杀
// goroutine，Task 必须检查 ctx.Done() 并将 context 传给数据库、Redis、HTTP 等下游。
//
// # 执行与并发
//
// 每次 cron 触发按固定顺序经过 triggered、本地 overlap gate、全局并发 gate、可选分布式锁、任务
// context、可选续租、started/result、Task 和逆序 cleanup。每个内部 stage 只释放自己取得的资源，调用
// 方不能插入或重排安全关键步骤。completed/failed duration 只度量 Task 实际执行，不包含 gate 或锁等待。
//
// AllowOverlap 默认为 false：同一 job 上一次 invocation 尚未退出时，本轮以 local_overlap 跳过。设为
// true 后允许同一 job 并发，但仍受全局并发和分布式锁约束。
//
// MaxConcurrentJobs 为零表示不限制全局并发，负数无效。GlobalConcurrencyPolicy 为空时默认 skip：
// 满载触发以 global_concurrency_limit 跳过；wait 会等待配额或 scheduler root context 取消。wait 与
// AllowOverlap=true 组合时，高频触发可能积累等待 goroutine；scheduler 不是持久队列，也没有 pending
// capacity 或崩溃恢复能力。全局并发满载时的真实 skip 代码参见 ExampleScheduler_globalConcurrency。
//
// # 分布式锁
//
// Config.Locker 只提供锁能力，不会自动给所有 job 加锁。Job.Lock 为 nil 时不访问 locker；非 nil 时
// 必须存在 Locker，并使用正数 TTL。LockPolicy.Key 为空时使用 Job.Key。TTL 为零时使用
// Config.DefaultLockTTL；两者都为零时 Add 返回 ErrInvalidLock。负数 default TTL 或 job TTL 始终无效。
// 同一个 scheduler 混合注册无锁与有锁任务的代码参见 ExampleNewRedisLocker。
//
// WaitTimeout 为零时只尝试一次，锁忙以 lock_busy 跳过；正数表示在总时限内使用 locker retry policy
// 等待。负数无效。Redis 命令错误以 lock_error 跳过，Task 不执行。unlock 使用独立的有界 context，
// 避免任务 context 已取消后无法收尾。单次抢锁和限时重试分别参见 ExampleScheduler_Add_lockOnce 与
// ExampleScheduler_Add_lockWait。
//
// NewRedisLocker 使用 Redis SET NX PX 获取锁，并以每次 acquire 生成的随机 owner token 配合 Lua 执行
// renew 和 unlock。非 owner 操作返回 ErrLockNotOwned。Namespace 与 Scope 通过 rediskey.Builder 形成
// 稳定 key；完整 key 和 owner token 不得写入日志或 metrics。直接使用 owner lock 的完整代码参见
// ExampleRedisLocker_Acquire。
//
// RetryPolicy 的 InitialInterval 和 MaxInterval 为零时分别默认 50ms 与 1s；负数无效。
// MaxAttempts 包含首次立即尝试：1 表示只尝试一次，0 表示不限制次数但仍受 WaitTimeout 约束，负数
// 无效。退避从 InitialInterval 指数增长并截断到 MaxInterval；Jitter=true 时每轮等待落在当前上限的
// 1/2 到完整值之间。Redis locker 的 owner lock 用法参见 ExampleNewRedisLocker。
//
// # 自动续租
//
// Renew 为 nil 时不续租。未配置 Renew 且 Job.Timeout 为正时，TTL 必须大于 Timeout，避免已知最长
// 执行时间超过 lease。执行时间可能超过 TTL 的任务必须配置 Renew。
//
// Renew.Interval 为零时默认 TTL/3，Renew.Timeout 为零时默认 5s；二者必须小于 TTL，负数无效。每次
// 续租成功都会把剩余有效期重置为 LockPolicy.TTL。任务结束时 scheduler 先停止并等待 renew goroutine，
// 再释放锁。协作响应 context 的长任务续租代码参见 ExampleScheduler_Add_lockRenew。
//
// 续租失败总会记录 lock_renew_failed 并合并到最终失败结果。ContinueOnFailure=false 时同时取消 Task
// context；true 时 Task 可以继续，但 invocation 最终仍按失败观测。进程崩溃后续租停止，Redis key
// 最迟在当前 TTL 到期后释放。两种失败策略分别参见 ExampleScheduler_Add_lockRenewFailure 与
// ExampleScheduler_Add_lockRenew_continueOnFailure。
//
// Redis owner-token lock 是 lease，不提供 exactly-once 或 fencing。所有可能产生外部副作用的任务仍
// 必须幂等，并在下游系统需要严格顺序时使用业务版本、fencing token 或事务约束。
//
// # 优雅关闭与观测
//
// 第一次 Stop 会先停止新 cron 触发，再取消活动任务、gate wait 和 lock wait，随后在后台等待所有已触发
// invocation drain。调用方 context 先取消或超时时，当前 Stop 返回包装后的 context error，但后台 drain
// 不停止；后续 Stop 继续等待同一完成状态。任务忽略 context 时，Stop 无法强制结束它。
// Stop 取消活动任务和重复等待同一 drain 的代码分别参见 ExampleScheduler_Stop 与
// ExampleScheduler_Stop_retryDrain。
//
// Metrics 按固定 job key 记录 triggered、started、completed、failed、skipped 和 lock_renew_failed。
// skipped reason 只使用 local_overlap、global_concurrency_limit、lock_busy 和 lock_error。未提供 Metrics
// 时使用 NopMetrics；未提供 logger 时使用 zap no-op logger。原始错误、cron spec、Redis key、owner
// token 和动态业务标识不得进入 metrics label。
//
// # 能力边界
//
// scheduler 负责进程内触发和通用 lease 协调，不持久化执行状态，不保证投递、顺序、exactly-once 或
// 进程崩溃恢复。可靠业务消息应使用 outbox/MQ，跨实例严格互斥应结合具备 fencing 的业务协议，feature
// orchestration 与业务一致性语义必须留在消费 feature。
package scheduler
