package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// New 创建一个未启动的定时任务调度器。
//
// 配置分为三个层级，使用时不要混淆：
//
//   - Config 是 scheduler 级配置，控制时区、全局并发、logger、metrics，并提供可选 Locker。
//   - RedisLockerOptions.Retry 是 locker 级配置，所有通过同一 locker 等待锁的 job 共享它。
//   - Job 是单任务配置；只有 Job.Lock 非 nil 时该任务才使用分布式锁，Renew 也只作用于该 job。
//
// 用法一：普通无锁任务和完整生命周期。
//
//	s, err := scheduler.New(scheduler.Config{
//		TimeZone: "Asia/Shanghai",
//	})
//	if err != nil {
//		return err
//	}
//	err = s.Add(scheduler.Job{
//		Key:          "cleanup_expired_data",
//		Spec:         "0 */5 * * * *", // 每 5 分钟执行；第一列是可选的秒字段。
//		Timeout:      30 * time.Second,
//		AllowOverlap: false, // 默认值；同一 job 的上一次调用未结束时跳过本轮。
//		Task: func(ctx context.Context) error {
//			return cleanupExpiredData(ctx)
//		},
//	})
//	if err != nil {
//		return err
//	}
//	if err := s.Start(); err != nil {
//		return err
//	}
//	// Add 可以在 Start 前后调用。Remove 只阻止后续触发，不中断已经开始的 invocation。
//	// s.Remove("cleanup_expired_data")
//	// ...运行应用...
//	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	return s.Stop(stopCtx)
//
// Spec 支持标准五字段、可选 seconds、@hourly 等 descriptor，以及 CRON_TZ=Asia/Shanghai
// 前缀。Job.Key 会裁剪空白并且必须唯一；Timeout 为零表示不设置任务 deadline，但任务
// context 仍会在 Stop 时取消。
//
// 用法二：创建 Redis Locker。提供 Locker 只是启用锁能力，不会自动给所有 job 加锁。
//
// 下面的 RetryPolicy 表示固定每 2 秒重试，最多尝试 3 次。MaxAttempts 包含首次立即尝试，
// 所以锁持续被占用时，尝试时间大致是 0s、2s、4s。MaxAttempts=1 表示只尝试一次；
// MaxAttempts=0 表示不限制次数，但仍受每个 Job.Lock.WaitTimeout 的总时限约束。
//
//	redisLocker, err := scheduler.NewRedisLocker(redisClient, scheduler.RedisLockerOptions{
//		Namespace: "aegiscore",
//		Scope:     []string{"scheduler"},
//		Retry: scheduler.RetryPolicy{
//			InitialInterval: 2 * time.Second,
//			MaxInterval:     2 * time.Second, // 与 InitialInterval 相同，形成固定间隔。
//			MaxAttempts:     3,
//			Jitter:          false,
//		},
//	})
//	if err != nil {
//		return err
//	}
//	s, err := scheduler.New(scheduler.Config{
//		TimeZone:       "Asia/Shanghai",
//		Locker:         redisLocker,
//		DefaultLockTTL: time.Minute, // Job.Lock.TTL 为零时使用该默认值。
//	})
//	if err != nil {
//		return err
//	}
//
// RetryPolicy 留空时，InitialInterval 默认 50ms、MaxInterval 默认 1s、MaxAttempts 默认 0、
// Jitter 默认 false。Job.Lock.TTL=0 时使用 Config.DefaultLockTTL；两者都为零会导致 Add
// 返回 ErrInvalidLock。
//
// 如果把 InitialInterval 设为 1 秒、MaxInterval 设为 8 秒，退避上限依次为
// 1s、2s、4s、8s；达到 8 秒后保持不变。Jitter=true 时，每轮实际等待会在当前退避
// 上限的 1/2 到完整值之间随机，降低多个实例同时重试造成的竞争。
//
// 同一个 scheduler 可以混合注册无锁和有锁任务。下面的 Lock 为 nil，即使 Config 已经
// 提供 Locker，该任务仍然不会访问 Redis lock：
//
//	err = s.Add(scheduler.Job{
//		Key:  "local_report",
//		Spec: "@hourly",
//		Task: buildLocalReport,
//	})
//	if err != nil {
//		return err
//	}
//
// 用法三：分布式锁只尝试一次。
//
// WaitTimeout=0 表示只执行首次 SET NX，不进入 retry loop。锁被其他 owner 持有时，
// 当前触发记录 skipped/lock_busy，不调用 Task。LockPolicy.Key 为空时自动使用 Job.Key；
// 通常应保持为空，让稳定 job key 同时成为跨实例锁标识。
//
//	err = s.Add(scheduler.Job{
//		Key:  "sync_catalog",
//		Spec: "0 */5 * * * *",
//		Lock: &scheduler.LockPolicy{
//			TTL:         time.Minute,
//			WaitTimeout: 0,
//		},
//		Task: syncCatalog,
//	})
//	if err != nil {
//		return err
//	}
//
// 用法四：在总等待上限内重试获取锁。
//
// 下面的 job 使用前述 locker 级 RetryPolicy：最多尝试 3 次、固定间隔 2 秒；
// WaitTimeout=10s 是该 job 获取锁的总等待上限。最大尝试次数、总等待时间、scheduler
// Stop 导致的 root context 取消和 Redis error 中任一条件先发生，都会提前结束等待。
//
//	err = s.Add(scheduler.Job{
//		Key:  "reconcile_inventory",
//		Spec: "0 */10 * * * *",
//		Lock: &scheduler.LockPolicy{
//			TTL:         time.Minute,
//			WaitTimeout: 10 * time.Second,
//		},
//		Task: reconcileInventory,
//	})
//	if err != nil {
//		return err
//	}
//
// 如果 MaxAttempts=3、固定间隔 2 秒，则通常在 0s、2s、4s 尝试后结束，不会为了用满
// 10 秒而继续重试；如果 MaxAttempts=0，则可以继续重试到 WaitTimeout 到期。等待到期或
// 尝试耗尽都视为 lock_busy；Redis 命令错误视为 lock_error，Task 都不会执行。
//
// 用法五：长任务自动续租。
//
// 假设任务最长运行 30 分钟，但锁 TTL 只有 1 分钟：如果不续租，任务运行 1 分钟后锁会
// 自动过期，其他实例可能再次获取锁并重复执行。配置 Renew 后，scheduler 每隔 Interval
// 刷新当前 owner 的 TTL；每次续租成功都会把剩余有效期重新设置为 LockPolicy.TTL。
//
//	err = s.Add(scheduler.Job{
//		Key:     "monthly_settlement",
//		Spec:    "0 0 2 1 * *", // 每月 1 日 02:00:00 执行。
//		Timeout: 30 * time.Minute, // 整个 Task 的执行 deadline。
//		Lock: &scheduler.LockPolicy{
//			TTL:         time.Minute,      // 不续租时，锁在 1 分钟后自然过期。
//			WaitTimeout: 15 * time.Second, // 首次获取锁最多等待 15 秒。
//			Renew: &scheduler.RenewPolicy{
//				Interval:          20 * time.Second, // 每 20 秒发起一次续租。
//				Timeout:           5 * time.Second,  // 单次 Redis 续租最多等待 5 秒。
//				ContinueOnFailure: false,
//			},
//		},
//		Task: runMonthlySettlement,
//	})
//	if err != nil {
//		return err
//	}
//
// 上述续租时间线为：0s 获取锁并得到 60s TTL；20s 续租后重新得到 60s TTL；40s 再次
// 续租后重新得到 60s TTL；任务结束时先停止并等待续租 goroutine，再释放锁。若进程崩溃，
// 续租随之停止，Redis key 最迟在当前 TTL 到期后释放，不会形成永久锁。
//
// Renew.Interval=0 默认使用 TTL/3，Renew.Timeout=0 默认使用 5 秒；Interval 和 Timeout
// 都必须小于 TTL。ContinueOnFailure=false 时，续租失败会取消 Task context；true 时
// Task 可以继续运行，但最终结果仍合并 renew error，并上报 lock_renew_failed。取消只是
// 协作通知，不会强杀 goroutine，因此长任务必须定期检查 ctx.Done() 并尽快返回。
//
//	func runMonthlySettlement(ctx context.Context) error {
//		for hasMoreWork() {
//			select {
//			case <-ctx.Done():
//				return ctx.Err()
//			default:
//				if err := settleNextBatch(ctx); err != nil {
//					return err
//				}
//			}
//		}
//		return nil
//	}
//
// 未配置 Renew 时，如果 Job.Timeout 为正数，Lock.TTL 必须大于 Job.Timeout，避免已知的
// 最长执行时间超过 lease。Redis owner-token lock 仍不是 exactly-once 或 fencing；所有
// 可能产生外部副作用的任务都应保持幂等。
//
// 用法六：限制 scheduler 全局并发。
//
//	s, err := scheduler.New(scheduler.Config{
//		MaxConcurrentJobs:       8,
//		GlobalConcurrencyPolicy: scheduler.GlobalConcurrencySkip,
//	})
//	if err != nil {
//		return err
//	}
//
// MaxConcurrentJobs=0 表示不限制全局并发；GlobalConcurrencyPolicy 留空默认使用
// GlobalConcurrencySkip。GlobalConcurrencySkip 在满载时立即以
// global_concurrency_limit 跳过；GlobalConcurrencyWait 会等待配额或 Stop 取消 root
// context。AllowOverlap=false 默认阻止同一 job 在本实例内重叠；AllowOverlap=true 允许
// 重叠，但与 global/lock wait 组合时，高频触发可能产生等待 goroutine，不是持久任务队列。
//
// 生命周期说明：Start 可重复调用且幂等；首次 Stop 会停止新触发、取消活动任务和等待者，
// 然后等待已触发 invocation drain。Stop 的调用方 context 超时只表示本次不再等待，后台
// drain 仍继续，后续 Stop 会等待同一个完成状态。进入 stopping 后不能再 Add 或重新 Start。
func New(cfg Config) (*Scheduler, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	metrics := cfg.Metrics
	if metrics == nil {
		metrics = NopMetrics{}
	}

	location := time.Local
	if strings.TrimSpace(cfg.TimeZone) != "" {
		loaded, err := time.LoadLocation(strings.TrimSpace(cfg.TimeZone))
		if err != nil {
			return nil, fmt.Errorf("load scheduler timezone: %w", err)
		}
		location = loaded
	}

	globalPolicy := cfg.GlobalConcurrencyPolicy
	if globalPolicy == "" {
		globalPolicy = GlobalConcurrencySkip
	}
	if globalPolicy != GlobalConcurrencySkip && globalPolicy != GlobalConcurrencyWait {
		return nil, fmt.Errorf("%w: unsupported global concurrency policy %q", ErrInvalidJob, globalPolicy)
	}

	var globalGate chan struct{}
	if cfg.MaxConcurrentJobs < 0 {
		return nil, fmt.Errorf("%w: max concurrent jobs must not be negative", ErrInvalidJob)
	}
	if cfg.MaxConcurrentJobs > 0 {
		globalGate = make(chan struct{}, cfg.MaxConcurrentJobs)
	}

	root, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cron: cron.New(
			cron.WithLocation(location),
			cron.WithLogger(cronZapLogger{logger: logger.Named("cron")}),
			cron.WithChain(cron.Recover(cronZapLogger{logger: logger.Named("cron")})),
			cron.WithParser(cron.NewParser(
				cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
			)),
		),
		logger:                  logger,
		locker:                  cfg.Locker,
		metrics:                 metrics,
		defaultLockTTL:          cfg.DefaultLockTTL,
		globalGate:              globalGate,
		globalConcurrencyPolicy: globalPolicy,
		root:                    root,
		cancel:                  cancel,
		jobs:                    make(map[string]cron.EntryID),
	}, nil
}

// Start 启动调度器；重复调用不会重复启动底层 cron。
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == schedulerStopping || s.state == schedulerStopped || s.cron == nil {
		return ErrSchedulerStopped
	}
	if s.state == schedulerRunning {
		return nil
	}
	s.cron.Start()
	s.state = schedulerRunning
	s.logger.Info("scheduler started")
	return nil
}

// Add 注册一个定时任务。
func (s *Scheduler) Add(cfg Job) error {
	if err := s.validateJob(&cfg); err != nil {
		return err
	}

	localGate := make(chan struct{}, 1)
	localGate <- struct{}{}
	pipeline := s.buildPipeline(localGate)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == schedulerStopping || s.state == schedulerStopped || s.cron == nil {
		return ErrSchedulerStopped
	}
	if _, exists := s.jobs[cfg.Key]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateJobKey, cfg.Key)
	}

	id, err := s.cron.AddJob(cfg.Spec, cron.FuncJob(func() {
		_ = pipeline(&invocation{ctx: s.root, job: cfg})
	}))
	if err != nil {
		return err
	}
	s.jobs[cfg.Key] = id
	s.logger.Info("scheduler job registered", zap.String("job", cfg.Key), zap.String("spec", cfg.Spec))
	return nil
}

// Remove 按固定 key 移除已注册任务；已开始的 invocation 不会被中断。
func (s *Scheduler) Remove(key string) bool {
	key = strings.TrimSpace(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron == nil {
		return false
	}
	id, exists := s.jobs[key]
	if !exists {
		return false
	}

	s.cron.Remove(id)
	delete(s.jobs, key)
	s.logger.Info("scheduler job removed", zap.String("job", key))
	return true
}

// Stop 停止调度新任务，并等待已触发任务结束或外部 context 到期。
// cron.Stop 不会强杀已运行任务；ctx 超时只表示停止等待，任务自身仍依赖 root context 和任务超时退出。
func (s *Scheduler) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.state == schedulerCreated || s.state == schedulerRunning {
		s.state = schedulerStopping
		s.drainDone = make(chan struct{})
		stopCtx := s.cron.Stop()
		s.cancel()
		go s.finishStop(stopCtx)
	}
	done := s.drainDone
	s.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.logger.Warn("scheduler stop timeout", zap.Error(ctx.Err()))
		return fmt.Errorf("stop scheduler: %w", ctx.Err())
	}
}

// finishStop 在后台完成唯一一次 drain，并向所有 Stop 调用者广播完成状态。
func (s *Scheduler) finishStop(stopCtx context.Context) {
	<-stopCtx.Done()

	s.mu.Lock()
	s.state = schedulerStopped
	s.cron = nil
	s.jobs = nil
	done := s.drainDone
	s.mu.Unlock()

	close(done)
	s.logger.Info("scheduler stopped")
}
