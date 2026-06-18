package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// GlobalConcurrencyPolicy 描述调度器全局并发满载时的处理方式。
type GlobalConcurrencyPolicy string

const (
	// GlobalConcurrencySkip 表示全局并发满载时跳过本轮执行。
	GlobalConcurrencySkip GlobalConcurrencyPolicy = "skip"
	// GlobalConcurrencyWait 表示全局并发满载时等待空位。
	GlobalConcurrencyWait GlobalConcurrencyPolicy = "wait"
)

// LockMode 描述分布式锁未立即获取时的处理方式。
type LockMode string

const (
	// LockModeSkipIfLocked 表示锁被占用时跳过本轮执行。
	LockModeSkipIfLocked LockMode = "skip_if_locked"
	// LockModeWait 表示锁被占用时在 WaitTimeout 内等待。
	LockModeWait LockMode = "wait"
)

// Config 配置定时任务调度器。
type Config struct {
	TimeZone                string
	Logger                  *zap.Logger
	Locker                  Locker
	Metrics                 Metrics
	DefaultLockTTL          time.Duration
	MaxConcurrentJobs       int
	GlobalConcurrencyPolicy GlobalConcurrencyPolicy
}

// JobConfig 描述一个定时任务。
type JobConfig struct {
	Key          string
	Spec         string
	Timeout      time.Duration
	AllowOverlap bool
	Lock         LockPolicy
	Task         func(context.Context) error
}

// LockPolicy 描述单个任务的分布式锁策略。
type LockPolicy struct {
	Enabled                bool
	Key                    string
	TTL                    time.Duration
	Mode                   LockMode
	WaitTimeout            time.Duration
	AutoRenew              bool
	RenewInterval          time.Duration
	RenewTimeout           time.Duration
	ContinueOnRenewFailure bool
}

// Scheduler 管理定时任务注册、执行、可观测性、可选分布式锁和优雅关闭。
type Scheduler struct {
	mu                      sync.Mutex
	cron                    *cron.Cron
	logger                  *zap.Logger
	locker                  Locker
	metrics                 Metrics
	defaultLockTTL          time.Duration
	globalGate              chan struct{}
	globalConcurrencyPolicy GlobalConcurrencyPolicy
	root                    context.Context
	cancel                  context.CancelFunc
	jobs                    map[string]cron.EntryID
	stopped                 bool
}

// New 创建一个未启动的定时任务调度器。
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

	if s.stopped || s.cron == nil {
		return ErrSchedulerStopped
	}
	s.cron.Start()
	s.logger.Info("scheduler started")
	return nil
}

// AddJob 注册一个定时任务。
func (s *Scheduler) AddJob(cfg JobConfig) (cron.EntryID, error) {
	if err := s.validateJob(&cfg); err != nil {
		return 0, err
	}

	localGate := make(chan struct{}, 1)
	localGate <- struct{}{}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped || s.cron == nil {
		return 0, ErrSchedulerStopped
	}
	if _, exists := s.jobs[cfg.Key]; exists {
		return 0, fmt.Errorf("%w: %s", ErrDuplicateJobKey, cfg.Key)
	}

	id, err := s.cron.AddJob(cfg.Spec, cron.FuncJob(func() {
		s.runJob(cfg, localGate)
	}))
	if err != nil {
		return 0, err
	}
	s.jobs[cfg.Key] = id
	s.logger.Info("scheduler job registered", zap.String("job", cfg.Key), zap.String("spec", cfg.Spec), zap.Int("entry_id", int(id)))
	return id, nil
}

// RemoveJob 移除已注册任务。
func (s *Scheduler) RemoveJob(key string) bool {
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
	s.logger.Info("scheduler job removed", zap.String("job", key), zap.Int("entry_id", int(id)))
	return true
}

// Shutdown 停止调度新任务，并等待已触发任务结束或外部 context 到期。
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.cancel()
	instance := s.cron
	s.mu.Unlock()

	if instance == nil {
		return nil
	}
	stopCtx := instance.Stop()

	select {
	case <-stopCtx.Done():
		s.mu.Lock()
		s.cron = nil
		s.jobs = nil
		s.mu.Unlock()
		s.logger.Info("scheduler stopped")
		return nil
	case <-ctx.Done():
		s.logger.Warn("scheduler shutdown timeout", zap.Error(ctx.Err()))
		return ctx.Err()
	}
}

func (s *Scheduler) runJob(cfg JobConfig, localGate chan struct{}) {
	enteredAt := time.Now()
	var (
		gateAcquired       bool
		globalGateAcquired bool
		lock               Lock
		jobCancel          context.CancelFunc = func() {}
		stopRenew          func()
		renewErrCh         <-chan error
		started            bool
		jobErr             error
	)

	defer func() {
		recovered := recover()

		if stopRenew != nil {
			stopRenew()
		}
		if jobErr == nil && renewErrCh != nil {
			if renewErr, ok := <-renewErrCh; ok && renewErr != nil {
				jobErr = fmt.Errorf("lock renew failed: %w", renewErr)
			}
		}
		jobCancel()
		if lock != nil {
			s.unlock(cfg.Key, lock)
		}
		if globalGateAcquired {
			<-s.globalGate
		}
		if gateAcquired {
			localGate <- struct{}{}
		}

		duration := time.Since(enteredAt)
		if recovered != nil {
			s.metrics.JobFailed(cfg.Key, duration)
			s.logger.Error("scheduler job panicked", zap.String("job", cfg.Key), zap.Any("panic", recovered), zap.Duration("duration", duration))
			return
		}
		if !started {
			return
		}
		if jobErr != nil {
			s.metrics.JobFailed(cfg.Key, duration)
			s.logger.Error("scheduler job failed", zap.String("job", cfg.Key), zap.Error(jobErr), zap.Duration("duration", duration))
			return
		}

		s.metrics.JobCompleted(cfg.Key, duration)
		s.logger.Info("scheduler job completed", zap.String("job", cfg.Key), zap.Duration("duration", duration))
	}()

	if s.isStopped() {
		return
	}
	s.metrics.JobTriggered(cfg.Key)

	if !cfg.AllowOverlap {
		select {
		case <-localGate:
			gateAcquired = true
		default:
			s.metrics.JobSkipped(cfg.Key, "local_overlap")
			s.logger.Info("scheduler job skipped because previous run is still active", zap.String("job", cfg.Key))
			return
		}
	}

	if s.globalGate != nil {
		if ok := s.acquireGlobalGate(); !ok {
			s.metrics.JobSkipped(cfg.Key, "global_concurrency_limit")
			s.logger.Info("scheduler job skipped because global concurrency limit is reached", zap.String("job", cfg.Key))
			return
		}
		globalGateAcquired = true
	}

	acquiredLock, ok, err := s.acquireLock(cfg)
	if err != nil {
		s.metrics.JobSkipped(cfg.Key, "lock_error")
		s.logger.Error("scheduler job lock failed", zap.String("job", cfg.Key), zap.Error(err))
		return
	}
	if !ok {
		s.metrics.JobSkipped(cfg.Key, "lock_busy")
		s.logger.Info("scheduler job skipped because distributed lock is held", zap.String("job", cfg.Key))
		return
	}
	lock = acquiredLock

	jobCtx := s.root
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(s.root, cfg.Timeout)
		jobCancel = cancel
	}
	if lock != nil && cfg.Lock.AutoRenew {
		stopRenew, renewErrCh = s.startRenew(jobCtx, jobCancel, cfg, lock)
	}

	started = true
	s.metrics.JobStarted(cfg.Key)
	s.logger.Info("scheduler job started", zap.String("job", cfg.Key))

	jobErr = cfg.Task(jobCtx)
}

func (s *Scheduler) acquireGlobalGate() bool {
	switch s.globalConcurrencyPolicy {
	case GlobalConcurrencyWait:
		select {
		case s.globalGate <- struct{}{}:
			return true
		case <-s.root.Done():
			return false
		}
	default:
		select {
		case s.globalGate <- struct{}{}:
			return true
		default:
			return false
		}
	}
}

func (s *Scheduler) acquireLock(cfg JobConfig) (Lock, bool, error) {
	if !cfg.Lock.Enabled {
		return nil, true, nil
	}

	key := strings.TrimSpace(cfg.Lock.Key)
	if key == "" {
		key = cfg.Key
	}

	waitTimeout := cfg.Lock.WaitTimeout
	if cfg.Lock.Mode == LockModeSkipIfLocked {
		waitTimeout = 0
	}
	return s.locker.Acquire(s.root, key, cfg.Lock.TTL, waitTimeout)
}

func (s *Scheduler) startRenew(jobCtx context.Context, cancelJob context.CancelFunc, cfg JobConfig, lock Lock) (func(), <-chan error) {
	interval := cfg.Lock.RenewInterval
	if interval <= 0 {
		interval = cfg.Lock.TTL / 3
	}
	renewTimeout := cfg.Lock.RenewTimeout
	if renewTimeout <= 0 {
		renewTimeout = 5 * time.Second
	}

	renewCtx, cancelRenew := context.WithCancel(s.root)
	done := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(done)
		defer close(errCh)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				return
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				opCtx, cancel := context.WithTimeout(renewCtx, renewTimeout)
				err := lock.Renew(opCtx, cfg.Lock.TTL)
				cancel()
				if err != nil {
					if errors.Is(err, context.Canceled) && renewCtx.Err() != nil {
						return
					}
					s.metrics.JobLockRenewFailed(cfg.Key)
					s.logger.Error("scheduler job lock renew failed", zap.String("job", cfg.Key), zap.Error(err))
					errCh <- err
					if !cfg.Lock.ContinueOnRenewFailure {
						cancelJob()
					}
					return
				}
			}
		}
	}()

	return func() {
		cancelRenew()
		<-done
	}, errCh
}

func (s *Scheduler) unlock(jobKey string, lock Lock) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := lock.Unlock(ctx); err != nil {
		s.logger.Warn("scheduler job lock unlock failed", zap.String("job", jobKey), zap.Error(err))
	}
}

func (s *Scheduler) validateJob(cfg *JobConfig) error {
	cfg.Key = strings.TrimSpace(cfg.Key)
	cfg.Spec = strings.TrimSpace(cfg.Spec)

	if cfg.Key == "" {
		return fmt.Errorf("%w: key is required", ErrInvalidJob)
	}
	if cfg.Spec == "" {
		return fmt.Errorf("%w: spec is required", ErrInvalidJob)
	}
	if cfg.Task == nil {
		return fmt.Errorf("%w: task is required", ErrInvalidJob)
	}
	if !cfg.Lock.Enabled {
		return nil
	}
	if s.locker == nil {
		return fmt.Errorf("%w: locker is required", ErrInvalidLock)
	}
	if cfg.Lock.Mode == "" {
		cfg.Lock.Mode = LockModeSkipIfLocked
	}
	if cfg.Lock.Mode != LockModeSkipIfLocked && cfg.Lock.Mode != LockModeWait {
		return fmt.Errorf("%w: unsupported mode %q", ErrInvalidLock, cfg.Lock.Mode)
	}
	if cfg.Lock.Mode == LockModeWait && cfg.Lock.WaitTimeout <= 0 {
		return fmt.Errorf("%w: wait timeout is required when mode is wait", ErrInvalidLock)
	}
	if cfg.Lock.TTL <= 0 {
		cfg.Lock.TTL = s.defaultLockTTL
	}
	if cfg.Lock.TTL <= 0 {
		return fmt.Errorf("%w: ttl is required", ErrInvalidLock)
	}
	if cfg.Timeout > 0 && !cfg.Lock.AutoRenew && cfg.Lock.TTL <= cfg.Timeout {
		return fmt.Errorf("%w: ttl must be greater than job timeout or auto renew must be enabled", ErrInvalidLock)
	}
	if cfg.Lock.AutoRenew {
		if cfg.Lock.RenewInterval <= 0 {
			cfg.Lock.RenewInterval = cfg.Lock.TTL / 3
		}
		if cfg.Lock.RenewInterval <= 0 || cfg.Lock.RenewInterval >= cfg.Lock.TTL {
			return fmt.Errorf("%w: renew interval must be positive and less than ttl", ErrInvalidLock)
		}
		if cfg.Lock.RenewTimeout <= 0 {
			cfg.Lock.RenewTimeout = 5 * time.Second
		}
		if cfg.Lock.RenewTimeout >= cfg.Lock.TTL {
			return fmt.Errorf("%w: renew timeout must be less than ttl", ErrInvalidLock)
		}
	}
	return nil
}

func (s *Scheduler) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}
