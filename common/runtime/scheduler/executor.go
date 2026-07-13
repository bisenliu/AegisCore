package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// runJob 串联一次任务触发的本地互斥、全局并发、分布式锁、超时、续租、执行和收尾流程。
// 所有资源释放、panic 捕获和结果记录都收口到 defer，避免任一准入阶段提前返回时泄漏 gate 或锁。
func (s *Scheduler) runJob(cfg JobConfig, localGate chan struct{}) {
	enteredAt := time.Now()
	state := newJobRunState()

	defer func() {
		s.finishJobRun(cfg, localGate, state, enteredAt, recover())
	}()

	if s.isStopped() {
		return
	}
	s.metrics.JobTriggered(cfg.Key)

	if !s.acquireExecutionGates(cfg, localGate, state) {
		return
	}
	if !s.acquireJobLock(cfg, state) {
		return
	}

	jobCtx := s.prepareJobRun(cfg, state)

	state.started = true
	s.metrics.JobStarted(cfg.Key)
	s.logger.Info("scheduler job started", zap.String("job", cfg.Key))

	state.jobErr = cfg.Task(jobCtx)
}

type jobRunState struct {
	// 这些字段记录本次 run 已持有的资源，cleanupJobRun 据此幂等释放未完整启动的任务。
	gateAcquired       bool
	globalGateAcquired bool
	lock               Lock
	jobCancel          context.CancelFunc
	stopRenew          func()
	renewErrCh         <-chan error
	started            bool
	jobErr             error
}

func newJobRunState() *jobRunState {
	return &jobRunState{jobCancel: func() {}}
}

// acquireExecutionGates 获取本地互斥和全局并发配额。
func (s *Scheduler) acquireExecutionGates(cfg JobConfig, localGate chan struct{}, state *jobRunState) bool {
	if !cfg.AllowOverlap {
		select {
		case <-localGate:
			state.gateAcquired = true
		default:
			s.metrics.JobSkipped(cfg.Key, "local_overlap")
			s.logger.Info("scheduler job skipped because previous run is still active", zap.String("job", cfg.Key))
			return false
		}
	}

	if s.globalGate == nil {
		return true
	}
	if ok := s.acquireGlobalGate(); !ok {
		s.metrics.JobSkipped(cfg.Key, "global_concurrency_limit")
		s.logger.Info("scheduler job skipped because global concurrency limit is reached", zap.String("job", cfg.Key))
		return false
	}
	state.globalGateAcquired = true
	return true
}

// acquireJobLock 获取任务分布式锁，并记录锁竞争或锁错误路径。
func (s *Scheduler) acquireJobLock(cfg JobConfig, state *jobRunState) bool {
	acquiredLock, ok, err := s.acquireLock(cfg)
	if err != nil {
		s.metrics.JobSkipped(cfg.Key, "lock_error")
		s.logger.Error("scheduler job lock failed", zap.String("job", cfg.Key), zap.Error(err))
		return false
	}
	if !ok {
		s.metrics.JobSkipped(cfg.Key, "lock_busy")
		s.logger.Info("scheduler job skipped because distributed lock is held", zap.String("job", cfg.Key))
		return false
	}
	state.lock = acquiredLock
	return true
}

// prepareJobRun 创建任务上下文并按锁策略启动续租。
func (s *Scheduler) prepareJobRun(cfg JobConfig, state *jobRunState) context.Context {
	var (
		jobCtx context.Context
		cancel context.CancelFunc
	)
	if cfg.Timeout > 0 {
		jobCtx, cancel = context.WithTimeout(s.root, cfg.Timeout)
	} else {
		jobCtx, cancel = context.WithCancel(s.root)
	}
	state.jobCancel = cancel
	if state.lock != nil && cfg.Lock.AutoRenew {
		state.stopRenew, state.renewErrCh = s.startRenew(jobCtx, state.jobCancel, cfg, state.lock)
	}
	return jobCtx
}

// finishJobRun 保持原有收尾顺序并记录最终结果。
func (s *Scheduler) finishJobRun(cfg JobConfig, localGate chan struct{}, state *jobRunState, enteredAt time.Time, recovered any) {
	s.cleanupJobRun(cfg, localGate, state)
	s.recordJobResult(cfg, state, time.Since(enteredAt), recovered)
}

func (s *Scheduler) cleanupJobRun(cfg JobConfig, localGate chan struct{}, state *jobRunState) {
	// 收尾顺序先停止续租并吸收续租错误，再取消任务上下文和释放锁，避免续租 goroutine 在锁释放后继续写同一 owner。
	if state.stopRenew != nil {
		state.stopRenew()
	}
	if state.jobErr == nil && state.renewErrCh != nil {
		if renewErr, ok := <-state.renewErrCh; ok && renewErr != nil {
			state.jobErr = fmt.Errorf("lock renew failed: %w", renewErr)
		}
	}
	state.jobCancel()
	if state.lock != nil {
		s.unlock(cfg.Key, state.lock)
	}
	if state.globalGateAcquired {
		<-s.globalGate
	}
	if state.gateAcquired {
		localGate <- struct{}{}
	}
}

func (s *Scheduler) recordJobResult(cfg JobConfig, state *jobRunState, duration time.Duration, recovered any) {
	if recovered != nil {
		s.metrics.JobFailed(cfg.Key, duration)
		s.logger.Error("scheduler job panicked", zap.String("job", cfg.Key), zap.Any("panic", recovered), zap.Duration("duration", duration))
		return
	}
	if !state.started {
		return
	}
	if state.jobErr != nil {
		s.metrics.JobFailed(cfg.Key, duration)
		s.logger.Error("scheduler job failed", zap.String("job", cfg.Key), zap.Error(state.jobErr), zap.Duration("duration", duration))
		return
	}

	s.metrics.JobCompleted(cfg.Key, duration)
	s.logger.Info("scheduler job completed", zap.String("job", cfg.Key), zap.Duration("duration", duration))
}

// acquireGlobalGate 根据全局并发策略获取执行配额。
// wait 策略会等待直到 root context 取消；默认 skip 策略只做一次非阻塞尝试。
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

// acquireLock 根据任务锁策略尝试获取分布式锁；未启用锁时视为已获得执行权。
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

// unlock 使用独立超时上下文释放任务锁，避免任务上下文取消后阻塞锁释放。
func (s *Scheduler) unlock(jobKey string, lock Lock) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultLockUnlockTimeout)
	defer cancel()
	if err := lock.Unlock(ctx); err != nil {
		s.logger.Warn("scheduler job lock unlock failed", zap.String("job", jobKey), zap.Error(err))
	}
}
