package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// renewGuard 归属于单次 invocation，stop 必须在 unlock 前完成。
type renewGuard struct {
	cancel context.CancelFunc
	done   chan struct{}
	errCh  chan error
}

// stop 由 renewStage 的单个 defer 调用，停止 goroutine 后返回本轮唯一的续租错误。
func (g *renewGuard) stop() error {
	g.cancel()
	<-g.done
	return <-g.errCh
}

// startRenewGuard 启动 invocation 局部续租循环，guard.stop 会停止并等待 goroutine。
func (s *Scheduler) startRenewGuard(jobCtx context.Context, cancelJob context.CancelFunc, job Job, lock Lock) *renewGuard {
	policy := job.Lock.Renew
	renewCtx, cancelRenew := context.WithCancel(jobCtx)
	guard := &renewGuard{
		cancel: cancelRenew,
		done:   make(chan struct{}),
		errCh:  make(chan error, 1),
	}

	go func() {
		defer close(guard.done)
		defer close(guard.errCh)

		ticker := time.NewTicker(policy.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				// 每次 Redis 操作另有短 timeout，避免一次续租阻塞整个 guard drain。
				opCtx, cancel := context.WithTimeout(renewCtx, policy.Timeout)
				err := lock.Renew(opCtx, job.Lock.TTL)
				cancel()
				if err == nil {
					continue
				}
				if renewCtx.Err() != nil {
					return
				}

				s.metrics.JobLockRenewFailed(job.Key)
				s.logger.Error("scheduler job lock renew failed", zap.String("job", job.Key), zap.Error(err))
				guard.errCh <- err
				if !policy.ContinueOnFailure {
					cancelJob()
				}
				return
			}
		}
	}()

	return guard
}
