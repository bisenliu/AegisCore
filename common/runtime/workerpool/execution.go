package workerpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"go.uber.org/zap"
)

func (p *Pool) run(ctx context.Context, task Task) {
	taskCtx, cancel := linkedTaskContext(ctx, p.ctx)
	defer cancel()

	p.counters.queued.Add(-1)
	p.counters.started.Add(1)
	p.counters.running.Add(1)
	defer p.counters.running.Add(-1)
	defer func() {
		if recovered := recover(); recovered != nil {
			p.counters.panicked.Add(1)
			p.log.Error("worker pool task panicked", p.fields(task, zap.Any("panic", recovered), zap.ByteString("stacktrace", debug.Stack()))...)
		}
	}()
	if err := task.Run(taskCtx); err != nil {
		p.counters.failed.Add(1)
		p.log.Error("worker pool task failed", p.fields(task, zap.Error(err))...)
		return
	}
	p.counters.completed.Add(1)
}

func linkedTaskContext(parent context.Context, poolCtx context.Context) (context.Context, context.CancelFunc) {
	taskCtx, cancel := context.WithCancel(parent)
	stopPoolCancel := context.AfterFunc(poolCtx, cancel)
	return taskCtx, func() {
		// 解除 AfterFunc 回调，避免任务正常结束后 poolCtx 取消再次触发无意义 cancel。
		stopPoolCancel()
		cancel()
	}
}

func (p *Pool) fields(task Task, fields ...zap.Field) []zap.Field {
	all := make([]zap.Field, 0, len(task.Fields)+len(fields)+2)
	all = append(all, zap.String("pool", p.name), zap.String("task", task.Name))
	all = append(all, task.Fields...)
	all = append(all, fields...)
	return all
}

func validateTask(task Task) error {
	if strings.TrimSpace(task.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidTask)
	}
	if task.Run == nil {
		return fmt.Errorf("%w: run function is required", ErrInvalidTask)
	}
	return nil
}
