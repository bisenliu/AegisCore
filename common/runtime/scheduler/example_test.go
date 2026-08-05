package scheduler_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"

	"github.com/aegiscore/common/runtime/scheduler"
)

// ExampleScheduler 展示普通无锁任务的完整生命周期。
//
// 该示例对应最常见的单实例任务：构造 scheduler、注册任务、启动 cron、按固定 key 删除任务，
// 最后由资源拥有者使用有界 context 显式停止。Add 也可以在 Start 之后调用；Remove 只阻止后续
// cron 触发，不会取消已经开始的 invocation。
func ExampleScheduler() {
	s, err := scheduler.New(scheduler.Config{
		// TimeZone 是 scheduler 默认时区；单个 Spec 仍可用 CRON_TZ 前缀覆盖。
		TimeZone: "Asia/Shanghai",
	})
	if err != nil {
		panic(err)
	}

	err = s.Add(scheduler.Job{
		// Key 会进入日志和 metrics，必须是稳定、唯一且低基数的名称。
		Key: "cleanup_expired_data",
		// 六字段格式的第一列是秒；这里表示每五分钟的第零秒触发。
		Spec: "0 */5 * * * *",
		// Timeout 只限制 Task 执行，不包含 overlap、全局 gate 或锁等待时间。
		Timeout: 30 * time.Second,
		// false 是默认值：上一次本地 invocation 未结束时，本轮直接跳过。
		AllowOverlap: false,
		Task: func(ctx context.Context) error {
			// 实际任务应把 ctx 继续传给数据库、Redis 或 HTTP 调用。
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	})
	if err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}

	// 示例在首次计划时间前删除任务，因此 Task 不会执行。
	fmt.Println(s.Remove("cleanup_expired_data"))

	stopCtx, cancelStop := context.WithTimeout(context.Background(), time.Second)
	defer cancelStop()
	fmt.Println(s.Stop(stopCtx) == nil)

	// Output:
	// true
	// true
}

// ExampleScheduler_Add_cronFormats 展示 scheduler 支持的 cron 表达式格式。
//
// 所有格式都在 Add 时由同一个 parser 校验。示例不启动 scheduler，只验证注册契约并立即 Stop；
// Stop 同样适用于尚未 Start 的 scheduler。
func ExampleScheduler_Add_cronFormats() {
	s, err := scheduler.New(scheduler.Config{TimeZone: "Asia/Shanghai"})
	if err != nil {
		panic(err)
	}

	formats := []struct {
		key  string
		spec string
	}{
		{key: "five_fields", spec: "0 0 * * *"},
		{key: "optional_seconds", spec: "0 0 0 * * *"},
		{key: "descriptor", spec: "@daily"},
		{key: "per_job_timezone", spec: "CRON_TZ=Asia/Shanghai 0 0 * * *"},
	}
	for _, item := range formats {
		if err := s.Add(scheduler.Job{
			Key:  item.key,
			Spec: item.spec,
			Task: func(context.Context) error { return nil },
		}); err != nil {
			panic(err)
		}
	}

	fmt.Println(len(formats))
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}

	// Output: 4
}

// ExampleNewRedisLocker 展示 Redis locker、scheduler 和 job 三个配置层级如何组合。
//
// 仅向 Config 提供 Locker 不会让全部任务自动加锁。local_report 的 Lock 为 nil，因此始终是
// 本地任务；sync_catalog 显式声明 Lock，并通过零 TTL 继承 Config.DefaultLockTTL。
func ExampleNewRedisLocker() {
	server, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer server.Close()
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()

	redisLocker, err := scheduler.NewRedisLocker(client, scheduler.RedisLockerOptions{
		Namespace: "aegiscore",
		Scope:     []string{"scheduler"},
		Retry: scheduler.RetryPolicy{
			// 固定两秒间隔；MaxAttempts=3 包含首次立即尝试，即大致在 0s、2s、4s 尝试。
			InitialInterval: 2 * time.Second,
			MaxInterval:     2 * time.Second,
			MaxAttempts:     3,
			Jitter:          false,
		},
	})
	if err != nil {
		panic(err)
	}

	s, err := scheduler.New(scheduler.Config{
		Locker: redisLocker,
		// 只有 Job.Lock 非 nil 且 TTL 为零时，任务才继承这个默认 lease 时长。
		DefaultLockTTL: time.Minute,
	})
	if err != nil {
		panic(err)
	}
	if err := s.Add(scheduler.Job{
		Key:  "local_report",
		Spec: "@hourly",
		Task: func(context.Context) error { return nil },
	}); err != nil {
		panic(err)
	}
	if err := s.Add(scheduler.Job{
		Key:  "sync_catalog",
		Spec: "@hourly",
		Lock: &scheduler.LockPolicy{}, // Key 默认使用 Job.Key，TTL 默认使用 Config.DefaultLockTTL。
		Task: func(context.Context) error { return nil },
	}); err != nil {
		panic(err)
	}

	// 两个 job 都能注册；是否使用 Redis 由各自 Lock 的 nil/non-nil 状态决定。
	fmt.Println(s.Remove("local_report"), s.Remove("sync_catalog"))
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}

	// Output: true true
}

// ExampleRedisLocker_Acquire 展示 owner-token lock 的获取、竞争、续租和释放。
//
// Acquire 返回 nil,false,nil 表示锁正被其他 owner 持有，不是 Redis 系统错误。Unlock 与 Renew
// 都会校验当前 owner token，因此旧 owner 不能删除或续租后来由其他实例取得的锁。
func ExampleRedisLocker_Acquire() {
	_, _, locker, closeRedis := newExampleRedisLocker(scheduler.RetryPolicy{MaxAttempts: 1})
	defer closeRedis()

	lock, acquired, err := locker.Acquire(context.Background(), "daily_report", time.Minute, 0)
	if err != nil {
		panic(err)
	}
	_, secondAcquired, err := locker.Acquire(context.Background(), "daily_report", time.Minute, 0)
	if err != nil {
		panic(err)
	}
	if err := lock.Renew(context.Background(), 2*time.Minute); err != nil {
		panic(err)
	}
	unlockErr := lock.Unlock(context.Background())
	fmt.Println(acquired, secondAcquired, unlockErr == nil)

	// Output: true false true
}

// ExampleScheduler_Add_lockOnce 展示 WaitTimeout=0 时只尝试一次分布式锁。
//
// 示例先由另一个 owner 占用 sync_catalog，再启动同 key 的 scheduler job。任务触发后只执行一次
// SET NX；锁忙会记录 skipped/lock_busy，Task 不会运行。
func ExampleScheduler_Add_lockOnce() {
	_, _, locker, closeRedis := newExampleRedisLocker(scheduler.RetryPolicy{MaxAttempts: 1})
	defer closeRedis()

	heldLock, acquired, err := locker.Acquire(context.Background(), "sync_catalog", time.Minute, 0)
	if err != nil || !acquired {
		panic("failed to prepare held lock")
	}
	defer func() { _ = heldLock.Unlock(context.Background()) }()

	metrics := newExampleMetrics()
	s, err := scheduler.New(scheduler.Config{Locker: locker, Metrics: metrics})
	if err != nil {
		panic(err)
	}
	var taskRuns atomic.Int64
	if err := s.Add(scheduler.Job{
		Key:  "sync_catalog",
		Spec: "* * * * * *",
		Lock: &scheduler.LockPolicy{
			TTL:         time.Minute,
			WaitTimeout: 0, // 零值明确表示单次尝试，不进入 retry loop。
		},
		Task: func(context.Context) error {
			taskRuns.Add(1)
			return nil
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}

	skipped := waitExampleValue(metrics.skipped)
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(skipped.reason, taskRuns.Load())

	// Output: lock_busy 0
}

// ExampleScheduler_Add_lockWait 展示正数 WaitTimeout 内的 Redis 重试与最终成功。
//
// RetryPolicy 属于 locker，WaitTimeout 属于 job：前者决定重试节奏，后者限制本次 invocation 的
// 总等待时间。示例通过 miniredis.CommandCount 观察至少两次 SET NX，再释放原 owner 的锁；这证明
// job 确实经历了竞争重试，而不是恰好在第一次尝试前取得锁。
func ExampleScheduler_Add_lockWait() {
	server, _, locker, closeRedis := newExampleRedisLocker(scheduler.RetryPolicy{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     10 * time.Millisecond,
		MaxAttempts:     0, // 零表示不限制次数，仍受下面 WaitTimeout 的总时限约束。
	})
	defer closeRedis()

	heldLock, acquired, err := locker.Acquire(context.Background(), "reconcile_inventory", time.Minute, 0)
	if err != nil || !acquired {
		panic("failed to prepare held lock")
	}
	commandsAfterOwnerAcquire := server.CommandCount()

	metrics := newExampleMetrics()
	s, err := scheduler.New(scheduler.Config{Locker: locker, Metrics: metrics})
	if err != nil {
		panic(err)
	}
	taskDone := make(chan struct{})
	if err := s.Add(scheduler.Job{
		Key:  "reconcile_inventory",
		Spec: "* * * * * *",
		Lock: &scheduler.LockPolicy{
			TTL:         time.Minute,
			WaitTimeout: 500 * time.Millisecond,
		},
		Task: func(context.Context) error {
			close(taskDone)
			return nil
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}

	// owner acquire 之后至少新增两条命令，表示 scheduler 已做首次尝试和至少一次重试。
	waitExampleCondition(func() bool { return server.CommandCount() >= commandsAfterOwnerAcquire+2 })
	if err := heldLock.Unlock(context.Background()); err != nil {
		panic(err)
	}
	waitExampleValue(taskDone)
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println("retried and completed")

	// Output: retried and completed
}

// ExampleScheduler_Add_lockRenew 展示长任务在持有 lock 时自动续租。
//
// 示例使用可观察的 Lock 实现验证 scheduler 在任务运行期间调用 Renew，并在任务结束后才调用 Unlock。
// 生产环境使用 RedisLocker 时，同一流程会通过带 owner token 的 Lua 脚本刷新 Redis TTL。
func ExampleScheduler_Add_lockRenew() {
	lock := &exampleLock{}
	metrics := newExampleMetrics()
	s, err := scheduler.New(scheduler.Config{
		Locker:  exampleLocker{lock: lock},
		Metrics: metrics,
	})
	if err != nil {
		panic(err)
	}
	if err := s.Add(scheduler.Job{
		Key:     "monthly_settlement",
		Spec:    "* * * * * *",
		Timeout: 500 * time.Millisecond,
		Lock: &scheduler.LockPolicy{
			TTL: 100 * time.Millisecond,
			Renew: &scheduler.RenewPolicy{
				Interval:          10 * time.Millisecond,
				Timeout:           50 * time.Millisecond,
				ContinueOnFailure: false,
			},
		},
		Task: func(ctx context.Context) error {
			// 长任务必须持续检查 ctx；这里模拟分批处理直到观察到首次续租。
			for lock.renewCount.Load() == 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					runtime.Gosched()
				}
			}
			return nil
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}
	waitExampleValue(metrics.completed)
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(lock.renewCount.Load() > 0, lock.unlocked.Load())

	// Output: true true
}

// ExampleScheduler_Add_lockRenewFailure 展示 ContinueOnFailure=false 的 fail-fast 行为。
//
// 续租失败会记录独立的 lock_renew_failed，并取消 Task context。Task 协作返回后，本次 invocation
// 还会记录 failed；无论任务返回什么错误，scheduler 都会等待 renew guard 退出后再释放 lock。
func ExampleScheduler_Add_lockRenewFailure() {
	lock := &exampleLock{renewErr: errors.New("redis unavailable")}
	metrics := newExampleMetrics()
	s, err := scheduler.New(scheduler.Config{
		Locker:  exampleLocker{lock: lock},
		Metrics: metrics,
	})
	if err != nil {
		panic(err)
	}
	taskResult := make(chan error, 1)
	if err := s.Add(scheduler.Job{
		Key:  "renew_fail_fast",
		Spec: "* * * * * *",
		Lock: &scheduler.LockPolicy{
			TTL: 100 * time.Millisecond,
			Renew: &scheduler.RenewPolicy{
				Interval:          10 * time.Millisecond,
				Timeout:           50 * time.Millisecond,
				ContinueOnFailure: false,
			},
		},
		Task: func(ctx context.Context) error {
			<-ctx.Done()
			taskResult <- ctx.Err()
			return ctx.Err()
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}
	waitExampleValue(metrics.failed)
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(errors.Is(<-taskResult, context.Canceled), metrics.renewFailedCount.Load() == 1)

	// Output: true true
}

// ExampleScheduler_Add_lockRenew_continueOnFailure 展示 ContinueOnFailure=true 的结果语义。
//
// 该策略不会取消 Task context，适合调用方希望完成当前幂等批次后再退出的场景；但续租失败仍会
// 合并进 invocation 最终结果，所以即使 Task 返回 nil，metrics 仍记录 failed 而不是 completed。
func ExampleScheduler_Add_lockRenew_continueOnFailure() {
	lock := &exampleLock{renewErr: errors.New("redis unavailable")}
	metrics := newExampleMetrics()
	s, err := scheduler.New(scheduler.Config{
		Locker:  exampleLocker{lock: lock},
		Metrics: metrics,
	})
	if err != nil {
		panic(err)
	}
	taskContextActive := make(chan bool, 1)
	if err := s.Add(scheduler.Job{
		Key:  "renew_continue",
		Spec: "* * * * * *",
		Lock: &scheduler.LockPolicy{
			TTL: 100 * time.Millisecond,
			Renew: &scheduler.RenewPolicy{
				Interval:          10 * time.Millisecond,
				Timeout:           50 * time.Millisecond,
				ContinueOnFailure: true,
			},
		},
		Task: func(ctx context.Context) error {
			// 等 metrics 确认续租失败已经被 scheduler 处理，再检查 context 未被取消。
			waitExampleCondition(func() bool { return metrics.renewFailedCount.Load() == 1 })
			taskContextActive <- ctx.Err() == nil
			return nil
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}
	waitExampleValue(metrics.failed)
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(<-taskContextActive, metrics.failedCount.Load() == 1)

	// Output: true true
}

// ExampleScheduler_globalConcurrency 展示 scheduler 级全局并发上限与 skip policy。
//
// 两个 job 在同一秒触发，但 MaxConcurrentJobs=1 只允许一个 Task 进入执行；另一个 invocation
// 立即以 global_concurrency_limit 跳过。若改为 GlobalConcurrencyWait，第二个 invocation 会等待
// 配额或 Stop 取消 root context，而不是进入持久队列。
func ExampleScheduler_globalConcurrency() {
	metrics := newExampleMetrics()
	s, err := scheduler.New(scheduler.Config{
		MaxConcurrentJobs:       1,
		GlobalConcurrencyPolicy: scheduler.GlobalConcurrencySkip,
		Metrics:                 metrics,
	})
	if err != nil {
		panic(err)
	}

	var taskRuns atomic.Int64
	releaseTask := make(chan struct{})
	task := func(ctx context.Context) error {
		taskRuns.Add(1)
		select {
		case <-releaseTask:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for _, key := range []string{"rebuild_search_index", "refresh_materialized_view"} {
		if err := s.Add(scheduler.Job{Key: key, Spec: "* * * * * *", Task: task}); err != nil {
			panic(err)
		}
	}
	if err := s.Start(); err != nil {
		panic(err)
	}

	waitExampleValue(metrics.started)
	skipped := waitExampleValue(metrics.skipped)
	close(releaseTask)
	waitExampleValue(metrics.completed)
	if err := s.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println(taskRuns.Load(), skipped.reason)

	// Output: 1 global_concurrency_limit
}

// ExampleScheduler_Stop 展示 Stop 通过任务 context 协作取消活动任务并等待 drain。
//
// 即使 Job.Timeout 为零，Task 仍收到 scheduler 派生的可取消 context。Stop 先停止新触发，再取消
// root context 并等待本次 Task 返回；它不会强杀忽略 context 的 goroutine。
func ExampleScheduler_Stop() {
	s, err := scheduler.New(scheduler.Config{})
	if err != nil {
		panic(err)
	}
	started := make(chan struct{})
	taskResult := make(chan error, 1)
	if err := s.Add(scheduler.Job{
		Key:  "reconcile_inventory",
		Spec: "* * * * * *",
		Task: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			taskResult <- ctx.Err()
			return ctx.Err()
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}
	waitExampleValue(started)

	stopCtx, cancelStop := context.WithTimeout(context.Background(), time.Second)
	defer cancelStop()
	if err := s.Stop(stopCtx); err != nil {
		panic(err)
	}
	fmt.Println(errors.Is(<-taskResult, context.Canceled))

	// Output: true
}

// ExampleScheduler_Stop_retryDrain 展示第一次 Stop 不再等待后，后台 drain 仍继续运行。
//
// 示例故意让 Task 在收到取消后暂时不返回。第一个 Stop 使用已取消的调用方 context，因此会返回
// 包装后的 context.Canceled；它不会丢弃底层 cron drain。释放任务后，第二个 Stop 会继续等待同一
// drain 并返回成功，而不是创建另一套关闭流程。
func ExampleScheduler_Stop_retryDrain() {
	s, err := scheduler.New(scheduler.Config{})
	if err != nil {
		panic(err)
	}
	started := make(chan struct{})
	releaseTask := make(chan struct{})
	if err := s.Add(scheduler.Job{
		Key:  "slow_shutdown",
		Spec: "* * * * * *",
		Task: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			// 生产任务不应无界忽略取消；这里仅用于证明 Stop 不会强杀 goroutine。
			<-releaseTask
			return ctx.Err()
		},
	}); err != nil {
		panic(err)
	}
	if err := s.Start(); err != nil {
		panic(err)
	}
	waitExampleValue(started)

	firstStopCtx, cancelFirstStop := context.WithCancel(context.Background())
	cancelFirstStop()
	firstErr := s.Stop(firstStopCtx)
	fmt.Println(errors.Is(firstErr, context.Canceled))

	close(releaseTask)
	secondErr := s.Stop(context.Background())
	fmt.Println(secondErr == nil)

	// Output:
	// true
	// true
}

// exampleSkip 保存 example metrics 观察到的稳定 job key 与跳过原因。
type exampleSkip struct {
	jobKey string
	reason string
}

// exampleMetrics 用带缓冲 channel 暴露 scheduler 事件，让示例按真实状态同步而不是依赖 Sleep。
type exampleMetrics struct {
	triggered chan string
	started   chan string
	completed chan string
	failed    chan string
	skipped   chan exampleSkip

	renewFailedCount atomic.Int64
	failedCount      atomic.Int64
}

func newExampleMetrics() *exampleMetrics {
	return &exampleMetrics{
		triggered: make(chan string, 32),
		started:   make(chan string, 32),
		completed: make(chan string, 32),
		failed:    make(chan string, 32),
		skipped:   make(chan exampleSkip, 32),
	}
}

func (m *exampleMetrics) JobTriggered(jobKey string) { m.triggered <- jobKey }
func (m *exampleMetrics) JobStarted(jobKey string)   { m.started <- jobKey }
func (m *exampleMetrics) JobCompleted(jobKey string, _ time.Duration) {
	m.completed <- jobKey
}
func (m *exampleMetrics) JobFailed(jobKey string, _ time.Duration) {
	m.failedCount.Add(1)
	m.failed <- jobKey
}
func (m *exampleMetrics) JobSkipped(jobKey, reason string) {
	m.skipped <- exampleSkip{jobKey: jobKey, reason: reason}
}
func (m *exampleMetrics) JobLockRenewFailed(string) {
	m.renewFailedCount.Add(1)
}

// exampleLocker 为续租示例返回同一个可观察 lock；它只模拟 Locker port，不实现 Redis retry。
type exampleLocker struct {
	lock scheduler.Lock
}

func (l exampleLocker) Acquire(context.Context, string, time.Duration, time.Duration) (scheduler.Lock, bool, error) {
	return l.lock, true, nil
}

// exampleLock 记录 renew/unlock，并可注入续租错误以展示两种 ContinueOnFailure 策略。
type exampleLock struct {
	renewErr   error
	renewCount atomic.Int64
	unlocked   atomic.Bool
}

func (l *exampleLock) Unlock(context.Context) error {
	l.unlocked.Store(true)
	return nil
}

func (l *exampleLock) Renew(context.Context, time.Duration) error {
	l.renewCount.Add(1)
	return l.renewErr
}

// newExampleRedisLocker 创建每个 Redis example 独占的内存 Redis、client 与 locker。
func newExampleRedisLocker(retry scheduler.RetryPolicy) (*miniredis.Miniredis, *redis.Client, *scheduler.RedisLocker, func()) {
	server, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	locker, err := scheduler.NewRedisLocker(client, scheduler.RedisLockerOptions{
		Namespace: "aegiscore",
		Scope:     []string{"scheduler"},
		Retry:     retry,
	})
	if err != nil {
		_ = client.Close()
		server.Close()
		panic(err)
	}
	return server, client, locker, func() {
		_ = client.Close()
		server.Close()
	}
}

// waitExampleValue 为所有异步示例提供统一 deadline，失败时给出明确的超时原因。
func waitExampleValue[T any](values <-chan T) T {
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		panic("timed out waiting for scheduler example event")
	}
}

// waitExampleCondition 使用可观察状态推进示例，不以固定睡眠假设 goroutine 已经执行。
func waitExampleCondition(condition func() bool) {
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			panic("timed out waiting for scheduler example condition")
		}
		runtime.Gosched()
	}
}
