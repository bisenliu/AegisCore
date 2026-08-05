package scheduler

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Start 启动调度器；重复调用不会重复启动底层 cron。
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == schedulerStopping || s.state == schedulerStopped || s.cron == nil {
		return ErrSchedulerStopped
	}
	if s.state == schedulerRunning {
		return nil
	}
	s.cron.Start()
	s.state = schedulerRunning
	s.logger.Info("scheduler started")
	return nil
}

// Stop 停止调度新任务，并等待已触发任务结束或外部 context 到期。
// cron.Stop 不会强杀已运行任务；ctx 超时只表示停止等待，任务自身仍依赖 root context 和任务超时退出。
func (s *Scheduler) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.state == schedulerCreated || s.state == schedulerRunning {
		s.state = schedulerStopping
		s.drainDone = make(chan struct{})
		stopCtx := s.cron.Stop()
		s.cancel()
		go s.finishStop(stopCtx)
	}
	done := s.drainDone
	s.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.logger.Warn("scheduler stop timeout", zap.Error(ctx.Err()))
		return fmt.Errorf("stop scheduler: %w", ctx.Err())
	}
}

// finishStop 在后台完成唯一一次 drain，并向所有 Stop 调用者广播完成状态。
func (s *Scheduler) finishStop(stopCtx context.Context) {
	<-stopCtx.Done()

	s.mu.Lock()
	s.state = schedulerStopped
	s.cron = nil
	s.jobs = nil
	done := s.drainDone
	s.mu.Unlock()

	close(done)
	s.logger.Info("scheduler stopped")
}
