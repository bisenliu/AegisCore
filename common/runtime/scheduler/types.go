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

// Job 描述一个定时任务。
type Job struct {
	Key          string
	Spec         string
	Timeout      time.Duration
	AllowOverlap bool
	// Lock 为 nil 时不使用分布式锁；非 nil 时由 scheduler 负责获取、续租和释放。
	Lock *LockPolicy
	Task func(context.Context) error
}

// LockPolicy 描述单个任务的分布式锁策略。
type LockPolicy struct {
	Key string
	TTL time.Duration
	// WaitTimeout 为零时只尝试一次，正数时在该总时限内按 locker retry policy 等待。
	WaitTimeout time.Duration
	// Renew 为 nil 时不续租；非 nil 时在任务运行期间维护当前 owner 的 lease。
	Renew *RenewPolicy
}

// RenewPolicy 描述单个任务的分布式锁续租策略。
type RenewPolicy struct {
	Interval          time.Duration
	Timeout           time.Duration
	ContinueOnFailure bool
}

// schedulerState 只描述 cron 生命周期，不参与单次 invocation 的资源清理。
type schedulerState uint8

const (
	schedulerCreated schedulerState = iota
	schedulerRunning
	schedulerStopping
	schedulerStopped
)

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
	// jobs 仅在包内保存 cron EntryID，对外注册和删除始终使用固定 job key。
	jobs map[string]cron.EntryID
	// state 只允许从 created/running 单向进入 stopping/stopped，scheduler 不支持重启。
	state schedulerState
	// drainDone 由首次 Stop 创建，后续 Stop 共享它等待同一批活动任务结束。
	drainDone chan struct{}
}
