package workerpool

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Stop 停止接收新任务，并等待已登记或已接收任务完成。
// StopTimeout <= 0 时只使用调用方 ctx；超时会取消 pool context，通知仍在运行的任务尽快退出。
// Stop 可重复调用，所有调用共享同一次 drain 状态。
func (p *Pool) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.stopOnce.Do(func() {
		p.admissionMu.Lock()
		p.closed.Store(true)
		p.admissionMu.Unlock()
		go p.drain()
	})

	select {
	case <-p.stopDone:
		return p.stopErr
	default:
	}

	stopCtx, cancel := withStopTimeout(ctx, p.stopTimeout)
	defer cancel()
	select {
	case <-p.stopDone:
		return p.stopErr
	case <-stopCtx.Done():
		select {
		case <-p.stopDone:
			return p.stopErr
		default:
		}
		p.cancel()
		err := fmt.Errorf("stop worker pool %s: %w", p.name, stopCtx.Err())
		p.log.Error("worker pool stop failed", zap.String("pool", p.name), zap.Any("stats", p.Stats()), zap.Error(err))
		return err
	}
}

func (p *Pool) drain() {
	// 先释放 ants 阻止新任务进入，再等待 inFlight；stopDone 为 stopErr 建立 happens-before 保证。
	p.stopErr = p.workersPool.ReleaseContext(context.Background())
	p.inFlight.Wait()
	p.cancel()
	if p.stopErr != nil {
		p.stopErr = fmt.Errorf("stop worker pool %s: %w", p.name, p.stopErr)
		p.log.Error("worker pool stop failed", zap.String("pool", p.name), zap.Any("stats", p.Stats()), zap.Error(p.stopErr))
	} else {
		p.log.Info("worker pool stopped", zap.String("pool", p.name), zap.Any("stats", p.Stats()))
	}
	close(p.stopDone)
}

func withStopTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
