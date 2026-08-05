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
// 用法一：注册不使用分布式锁的普通任务。
//
//	s, err := scheduler.New(scheduler.Config{TimeZone: "Asia/Shanghai"})
//	if err != nil {
//		return err
//	}
//	err = s.Add(scheduler.Job{
//		Key:     "cleanup_expired_data",
//		Spec:    "0 */5 * * * *",
//		Timeout: 30 * time.Second,
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
//	// ...运行应用；期间可按固定 key 动态 Remove 任务...
//	// s.Remove("cleanup_expired_data")
//	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	return s.Stop(stopCtx)
//
// 用法二：提供 Redis Locker。RetryPolicy 属于 locker，所有使用该 locker 的锁任务共享
// 相同的重试退避参数；仅仅设置 Config.Locker 不会让所有任务自动加锁。
//
//	redisLocker, err := scheduler.NewRedisLocker(redisClient, scheduler.RedisLockerOptions{
//		Namespace: "aegiscore",
//		Scope:     []string{"scheduler"},
//		Retry: scheduler.RetryPolicy{
//			InitialInterval: 2 * time.Second,
//			MaxInterval:     2 * time.Second,
//			MaxAttempts:     3,
//			Jitter:          false,
//		},
//	})
//	if err != nil {
//		return err
//	}
//	s, err := scheduler.New(scheduler.Config{
//		TimeZone:                "Asia/Shanghai",
//		Locker:                  redisLocker,
//		DefaultLockTTL:          time.Minute,
//		MaxConcurrentJobs:       8,
//		GlobalConcurrencyPolicy: scheduler.GlobalConcurrencySkip,
//	})
//	if err != nil {
//		return err
//	}
//
// 即使 scheduler 已提供 Locker，下面的 Job.Lock 仍为 nil，因此该任务不会获取分布式锁：
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
// 用法三：锁只尝试一次。WaitTimeout 为零时不会进入 retry loop；锁被其他 owner 持有时，
// 本轮任务以 lock_busy 跳过。LockPolicy.Key 为空时自动使用 Job.Key。
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
// 用法四：在总等待上限内重试获取锁。下面最多尝试 3 次，其中包含首次立即尝试；
// 固定间隔为 2 秒，但如果先达到 WaitTimeout、调用方 context 取消或 Redis 返回错误，
// 会提前结束。将 InitialInterval 设为 1 秒、MaxInterval 设为 8 秒时，则按
// 1s、2s、4s、8s 的上限进行指数退避；Jitter=true 会进一步随机化每次等待。
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
// 用法五：长任务自动续租。Renew 只对当前 Job 生效；Interval 和 Timeout 为零时分别
// 默认使用 TTL/3 和 5 秒。ContinueOnFailure=false 会在续租失败时取消任务 context；
// true 会允许任务继续，但最终结果仍保留 renew failure，并上报 lock_renew_failed。
//
//	err = s.Add(scheduler.Job{
//		Key:     "monthly_settlement",
//		Spec:    "0 0 2 1 * *",
//		Timeout: 30 * time.Minute,
//		Lock: &scheduler.LockPolicy{
//			TTL:         time.Minute,
//			WaitTimeout: 15 * time.Second,
//			Renew: &scheduler.RenewPolicy{
//				Interval:          20 * time.Second,
//				Timeout:           5 * time.Second,
//				ContinueOnFailure: false,
//			},
//		},
//		Task: runMonthlySettlement,
//	})
//	if err != nil {
//		return err
//	}
//
// GlobalConcurrencySkip 在满载时跳过当前触发；GlobalConcurrencyWait 等待配额或 Stop
// 取消 root context。Stop 只协作取消活动任务和等待者，不会强杀 goroutine。Redis lock
// 是 owner-token lease，不提供 exactly-once 或 fencing，任务仍须保证幂等并响应 context。
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
