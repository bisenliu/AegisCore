package scheduler

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

// startRenew 启动锁续租循环，并在不可恢复的续租失败时按任务策略取消任务。
func (s *Scheduler) startRenew(jobCtx context.Context, cancelJob context.CancelFunc, cfg JobConfig, lock Lock) (func(), <-chan error) {
	interval := cfg.Lock.RenewInterval
	if interval <= 0 {
		interval = cfg.Lock.TTL / 3
	}
	renewTimeout := cfg.Lock.RenewTimeout
	if renewTimeout <= 0 {
		renewTimeout = defaultLockRenewTimeout
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
