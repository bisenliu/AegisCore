package workerpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"go.uber.org/zap"
)

// run 调度工作协程真正执行用户提交的任务，处理超时控制并统计任务执行耗时与完成情况。
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

// linkedTaskContext 将任务自身的生命周期与整个任务池的关闭流程绑定，当任务池停机或任务被调用方主动取消时，能及时终止执行。
func linkedTaskContext(parent context.Context, poolCtx context.Context) (context.Context, context.CancelFunc) {
	taskCtx, cancel := context.WithCancel(parent)
	stopPoolCancel := context.AfterFunc(poolCtx, cancel)
	return taskCtx, func() {
		// 解除 AfterFunc 回调，避免任务正常结束后 poolCtx 取消再次触发无意义 cancel。
		stopPoolCancel()
		cancel()
	}
}

// fields 为任务日志附加任务池名称和具体任务名称，方便排查报错时确认是哪个异步任务出错。
func (p *Pool) fields(task Task, fields ...zap.Field) []zap.Field {
	all := make([]zap.Field, 0, len(task.Fields)+len(fields)+2)
	all = append(all, zap.String("pool", p.name), zap.String("task", task.Name))
	all = append(all, task.Fields...)
	all = append(all, fields...)
	return all
}

// validateTask 检查提交的任务是否合规，必须提供任务名称和具体的执行函数。
func validateTask(task Task) error {
	if strings.TrimSpace(task.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidTask)
	}
	if task.Run == nil {
		return fmt.Errorf("%w: run function is required", ErrInvalidTask)
	}
	return nil
}
