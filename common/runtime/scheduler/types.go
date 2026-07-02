package scheduler

import (
	"context"
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
