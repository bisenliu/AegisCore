package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// invocation 只传递本次触发所需的数据，不保存跨 stage 的资源持有布尔状态。
type invocation struct {
	ctx      context.Context
	job      Job
	lock     Lock
	duration time.Duration
}

// handler 是内部带 context/error 语义的执行单元，最终节点调用业务 Task。
type handler func(*invocation) error

// middleware 是不可导出的固定 stage 构造器，调用方不能插入或重排安全关键步骤。
type middleware func(handler) handler

// buildPipeline 在 Add 时构造不可变执行链。
// resultStage 在调用栈上包住 renewStage，以便续租错误合并后再记录结果；实际前向动作仍按
// triggered、local、global、lock、task context、renew、started、task 的顺序发生。
func (s *Scheduler) buildPipeline(localGate chan struct{}) handler {
	return chain(
		func(inv *invocation) error {
			return inv.job.Task(inv.ctx)
		},
		s.triggeredStage(),
		s.localOverlapStage(localGate),
		s.globalConcurrencyStage(),
		s.lockStage(),
		s.taskContextStage(),
		s.resultStage(),
		s.renewStage(),
		s.startedStage(),
	)
}

// chain 从后向前包装 stage，使参数声明顺序等于执行链的外到内顺序。
func chain(final handler, stages ...middleware) handler {
	for index := len(stages) - 1; index >= 0; index-- {
		final = stages[index](final)
	}
	return final
}

// triggeredStage 在任何准入判断前记录一次固定 job key 的触发事件。
func (s *Scheduler) triggeredStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) error {
			s.metrics.JobTriggered(inv.job.Key)
			return next(inv)
		}
	}
}

// localOverlapStage 为默认不允许重叠的 job 提供进程内单任务互斥。
func (s *Scheduler) localOverlapStage(gate chan struct{}) middleware {
	return func(next handler) handler {
		return func(inv *invocation) error {
			if inv.job.AllowOverlap {
				return next(inv)
			}
			select {
			case <-gate:
				// 成功取得本地 token 后立即登记释放，task error 或 panic 都会归还。
				defer func() { gate <- struct{}{} }()
				return next(inv)
			default:
				s.metrics.JobSkipped(inv.job.Key, "local_overlap")
				s.logger.Info("scheduler job skipped because previous run is still active", zap.String("job", inv.job.Key))
				return nil
			}
		}
	}
}

// globalConcurrencyStage 按 scheduler 级 skip/wait policy 获取共享并发配额。
func (s *Scheduler) globalConcurrencyStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) error {
			if s.globalGate == nil {
				return next(inv)
			}
			if !s.acquireGlobalGate() {
				s.metrics.JobSkipped(inv.job.Key, "global_concurrency_limit")
				s.logger.Info("scheduler job skipped because global concurrency limit is reached", zap.String("job", inv.job.Key))
				return nil
			}
			// 全局 token 的所有权止于本 stage，内层任何退出路径都会释放。
			defer func() { <-s.globalGate }()
			return next(inv)
		}
	}
}

// lockStage 获取可选分布式锁，并把 busy 与系统错误映射为稳定 skip reason。
func (s *Scheduler) lockStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) error {
			lock, ok, err := s.acquireLock(inv.ctx, inv.job)
			if err != nil {
				s.metrics.JobSkipped(inv.job.Key, "lock_error")
				s.logger.Error("scheduler job lock failed", zap.String("job", inv.job.Key), zap.Error(err))
				return nil
			}
			if !ok {
				s.metrics.JobSkipped(inv.job.Key, "lock_busy")
				s.logger.Info("scheduler job skipped because distributed lock is held", zap.String("job", inv.job.Key))
				return nil
			}
			if lock == nil {
				return next(inv)
			}

			inv.lock = lock
			// unlock 使用独立 context，并在 context/renew stage 完成清理后执行。
			defer s.unlock(inv.job.Key, lock)
			return next(inv)
		}
	}
}

// taskContextStage 从 scheduler root 派生任务 context，并应用可选任务 timeout。
func (s *Scheduler) taskContextStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) error {
			var cancel context.CancelFunc
			if inv.job.Timeout > 0 {
				inv.ctx, cancel = context.WithTimeout(inv.ctx, inv.job.Timeout)
			} else {
				inv.ctx, cancel = context.WithCancel(inv.ctx)
			}
			// 即使未配置 timeout，也创建可由 scheduler Stop 取消的任务 context。
			defer cancel()
			return next(inv)
		}
	}
}

// resultStage 包住续租与任务执行，使续租错误合并后再记录最终结果。
func (s *Scheduler) resultStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					s.metrics.JobFailed(inv.job.Key, inv.duration)
					s.logger.Error("scheduler job panicked", zap.String("job", inv.job.Key), zap.Any("panic", recovered), zap.Duration("duration", inv.duration))
					panic(recovered)
				}
			}()

			err = next(inv)
			if err != nil {
				s.metrics.JobFailed(inv.job.Key, inv.duration)
				s.logger.Error("scheduler job failed", zap.String("job", inv.job.Key), zap.Error(err), zap.Duration("duration", inv.duration))
				return err
			}
			s.metrics.JobCompleted(inv.job.Key, inv.duration)
			s.logger.Info("scheduler job completed", zap.String("job", inv.job.Key), zap.Duration("duration", inv.duration))
			return nil
		}
	}
}

// renewStage 为已持有的 lock 启动局部续租 guard，并合并 task 与 renew error。
func (s *Scheduler) renewStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) (err error) {
			if inv.lock == nil || inv.job.Lock == nil || inv.job.Lock.Renew == nil {
				return next(inv)
			}

			jobCtx, cancelJob := context.WithCancel(inv.ctx)
			inv.ctx = jobCtx
			// guard.stop 先停止并等待续租 goroutine，再把续租错误合并进任务结果。
			defer cancelJob()
			guard := s.startRenewGuard(jobCtx, cancelJob, inv.job, inv.lock)
			defer func() {
				if renewErr := guard.stop(); renewErr != nil {
					err = errors.Join(err, fmt.Errorf("lock renew failed: %w", renewErr))
				}
			}()
			return next(inv)
		}
	}
}

// startedStage 在调用 Task 前记录 started，并只测量实际 Task 执行耗时。
func (s *Scheduler) startedStage() middleware {
	return func(next handler) handler {
		return func(inv *invocation) error {
			s.metrics.JobStarted(inv.job.Key)
			s.logger.Info("scheduler job started", zap.String("job", inv.job.Key))
			// duration 只包围 Task，不包含任何 gate、lock wait 或 renew guard 初始化。
			startedAt := time.Now()
			defer func() { inv.duration = time.Since(startedAt) }()
			return next(inv)
		}
	}
}

// acquireGlobalGate 根据全局并发策略获取执行配额；wait 会同时监听 scheduler root 取消。
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

// acquireLock 根据任务锁策略尝试获取分布式锁。
func (s *Scheduler) acquireLock(ctx context.Context, job Job) (Lock, bool, error) {
	if job.Lock == nil {
		return nil, true, nil
	}

	key := strings.TrimSpace(job.Lock.Key)
	if key == "" {
		key = job.Key
	}
	return s.locker.Acquire(ctx, key, job.Lock.TTL, job.Lock.WaitTimeout)
}

// unlock 使用独立超时上下文释放任务锁，避免任务上下文取消后阻塞锁释放。
func (s *Scheduler) unlock(jobKey string, lock Lock) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultLockUnlockTimeout)
	defer cancel()
	if err := lock.Unlock(ctx); err != nil {
		s.logger.Warn("scheduler job lock unlock failed", zap.String("job", jobKey), zap.Error(err))
	}
}
