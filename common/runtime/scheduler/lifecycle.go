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
// cron.Stop 不会强杀已运行任务；ctx 超时只表示停止等待，任务自身仍依赖 root context 和任务超时退出。
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

// isStopped 返回调度器是否已经停止接收或执行新触发任务。
func (s *Scheduler) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}
