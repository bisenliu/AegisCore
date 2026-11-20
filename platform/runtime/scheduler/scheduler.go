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
// 完整的配置、并发、锁、续租、观测和关闭契约参见 package scheduler 文档与 ExampleScheduler。
func New(cfg Config) (*Scheduler, error) {
	if cfg.DefaultLockTTL < 0 {
		return nil, fmt.Errorf("%w: default lock ttl must not be negative", ErrInvalidLock)
	}

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
