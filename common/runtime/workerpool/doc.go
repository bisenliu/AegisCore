// Package workerpool 提供固定最大并发数的进程内后台任务池。
//
// # 配置
//
// Options 的配置语义如下：
//
//   - Name 是任务池的稳定名称，会裁剪首尾空白且不能为空。它会进入日志和 metrics，应使用
//     auth.session_purge 等低基数名称，不能包含 user ID、请求 ID 等动态内容。
//   - Workers 是最多同时执行的任务数，必须大于零。ants.WithPreAlloc 只预分配内部 worker queue
//     的内存，不会预先启动 worker goroutine。所有 worker 忙碌时 Submit 会阻塞等待。
//   - StopTimeout 是每次 Stop 调用等待 drain 的内部上限。大于零时，它与 Stop 调用方 context
//     中更早到期的 deadline 共同生效；小于等于零时只使用调用方 context。
//   - logger 可以为 nil，此时使用 zap no-op logger。生产环境应传入真实 logger，以观察任务失败、
//     panic 和停止异常。
//
// Pool 创建后立即可用，不需要 Start。创建方拥有其生命周期，即使尚未提交任务，也必须在应用、
// Fx provider 或其他资源所有者的关闭边界显式调用 Stop，不能只依赖进程退出回收资源。
//
// # 基本用法
//
// 创建、提交、等待完成和显式停止的完整可编译代码参见 ExamplePool。Stop 应在应用或拥有该 Pool 的
// 组件停止时调用，不能在每次 Submit 后调用。
//
// # 提交与执行结果
//
// Submit 返回 nil 只表示任务已被任务池接收，不表示 Task.Run 已成功完成。任务的执行错误通过日志和
// Stats 暴露，不会异步返回给 Submit 调用方。需要让业务调用方获知执行结果时，应在业务层设计明确
// 的结果通道或状态存储，不能把 Submit 的 nil 当成业务成功。
//
// Task.Name 裁剪后必须非空，Task.Run 不能为空，否则 Submit 返回 ErrInvalidTask。Submit 调用前
// context 已取消时直接返回 ctx.Err()；Pool 已开始 Stop 后返回 ErrClosed。
//
// Task.Run 的结果分类如下：
//
//   - 返回 error 时，任务计入 Failed，并以 Task.Fields 记录 worker pool task failed 日志；
//     workerpool 不自动重试。
//   - panic 会在 worker 边界恢复，计入 Panicked 并记录 panic 和 stacktrace；该任务不计入
//     Completed 或 Failed，也不会自动重试。
//   - 返回 nil 时，任务计入 Completed。
//
// Task.Name 应是稳定操作名，Task.Fields 用于提供本次任务的定位字段。Fields 不得包含密码、token、
// 完整 Redis key 或其他敏感值。
//
// # Context 所有权
//
// Task.Run 收到的 context 同时关联 Submit 的 context 和 pool 生命周期。提交方在任务运行期间取消
// context 时，taskCtx.Done() 会关闭；取消只是协作通知，不会强杀 goroutine，因此 Task.Run 必须检查
// context，并把它传给数据库、Redis、HTTP 等下游调用。
//
// 让任务跟随请求取消的完整代码参见 ExamplePool_Submit_requestContext。
//
// 如果任务本来就应该在 HTTP 请求结束后继续运行，不要直接传 requestCtx。可以使用
// context.WithoutCancel(requestCtx) 保留 trace 等 context value、移除请求取消，再在 Task.Run 内
// 设置任务自己的 deadline。不要在 Submit 返回后立即 cancel 作为 parent 传入的 context，否则刚被
// 接收的异步任务也会被取消。完整代码参见 ExamplePool_Submit_detachedContext。
//
// # 满载背压
//
// Workers=2 时最多同时运行两个任务。第三个及后续 Submit 会阻塞，直到有 worker 空闲或 Pool 被
// Stop。Options 没有 queue capacity、非阻塞提交或 admission timeout 配置。Submit 的 context 会在
// 进入等待前检查，但在等待空闲 worker 的过程中取消 context 不会中断该次阻塞；任务最终被接收后会
// 携带已经取消的 context 开始执行，或者在 Pool 停止竞态中返回 ErrClosed。
//
// 因此只能在允许同步背压的调用路径直接 Submit，不能把它当成“请求立即返回、后台无限排队”的队列。
// 如果入口不能阻塞，应由调用方设计明确的过载策略，或选择具备有界队列和可取消 admission 的组件；
// 不要通过无界 goroutine 包裹 Submit。ErrQueueFull 保留用于映射 ants 过载错误，但当前阻塞配置不会
// 仅因为所有 worker 正忙就返回 ErrQueueFull。可观察的背压示例参见 ExamplePool_Submit_backpressure。
//
// # 优雅停止
//
// 第一次 Stop 会原子地关闭准入，使后续 Submit 返回 ErrClosed，并启动一份由所有 Stop 调用共享的
// drain。正常停止会等待已经登记或接收的任务自然结束，不会立即取消正在运行的 Task context。
//
// 如果 StopTimeout 或调用方 context 先到期，本次 Stop 返回包装后的 context.Canceled 或
// context.DeadlineExceeded，同时取消 pool 生命周期 context，通知仍在运行的任务尽快退出。Stop 不会
// 强杀忽略 context 的任务，后台 drain 仍会继续；后续 Stop 可以重新等待同一份 drain 完成状态。
// 完整代码参见 ExamplePool_Stop_retryDrain。
//
// Stop 可重复并发调用。不要从 Pool 自己的 Task.Run 内调用 Stop：Stop 要等待包括当前任务在内的
// in-flight 任务，可能形成自等待；应由任务池的资源所有者统一停止。StopTimeout<=0 且调用方使用
// context.Background() 时，如果某个任务永久忽略 context，Stop 也可能永久等待。
//
// # 统计
//
// Submitted、Rejected、Started、Completed、Failed 和 Panicked 是累计计数；Queued、Running、Free、
// Waiting 和 Closed 是当前状态。Submit 阻塞等待 worker 时会临时登记到 Submitted 和 Queued；如果
// ants 最终拒绝该任务，会回滚这两个计数并增加 Rejected。ErrInvalidTask 和进入准入前已经取消的
// context 不计入 Rejected。Waiting 是 ants 报告的等待提交方数量。
//
// Stats 的字段分别读取，在高并发变化期间不保证对应同一个精确时间点。Pool 实现 StatsSource，可直接
// 交给 metrics collector。读取运行中和停止后快照的完整代码参见 ExamplePool_Stats。
//
// # 能力边界
//
// workerpool 是单进程、内存内的并发限制 primitive，不持久化任务，不保证执行顺序、重试、
// exactly-once 或进程崩溃后的恢复。可靠投递应使用 outbox、MQ 或对应业务基础设施；周期任务应使用
// scheduler。
package workerpool
