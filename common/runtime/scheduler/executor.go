package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// runJob 串联一次任务触发的本地互斥、全局并发、分布式锁、超时、续租、执行和收尾流程。
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

// acquireGlobalGate 根据全局并发策略获取执行配额。
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := lock.Unlock(ctx); err != nil {
		s.logger.Warn("scheduler job lock unlock failed", zap.String("job", jobKey), zap.Error(err))
	}
}
