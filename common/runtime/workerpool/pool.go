package workerpool

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Task 描述一个后台任务单元。
type Task struct {
	Name   string
	Fields []zap.Field
	Run    func(context.Context) error
}

// Options 配置后台任务池。
type Options struct {
	Name        string
	Workers     int
	StopTimeout time.Duration
}

// Pool 使用 ants 原生池执行后台任务，并通过固定容量限制并发。
type Pool struct {
	name        string
	workers     int
	stopTimeout time.Duration
	log         *zap.Logger

	ctx         context.Context
	cancel      context.CancelFunc
	workersPool *ants.Pool

	admissionMu sync.Mutex
	inFlight    sync.WaitGroup
	stopOnce    sync.Once
	stopDone    chan struct{}
	stopErr     error
	closed      atomic.Bool

	counters counters
}

// New 创建后台任务池，并向 Fx 注册停止钩子。
func New(lc fx.Lifecycle, log *zap.Logger, opts Options) (*Pool, error) {
	pool, err := NewUnmanaged(log, opts)
	if err != nil {
		return nil, err
	}
	if lc != nil {
		lc.Append(fx.Hook{OnStop: pool.Stop})
	}
	return pool, nil
}

// NewUnmanaged 创建不注册 Fx 生命周期钩子的后台任务池，供测试或手动管理场景使用。
func NewUnmanaged(log *zap.Logger, opts Options) (*Pool, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidTask)
	}
	if opts.Workers <= 0 {
		return nil, fmt.Errorf("%w: workers must be positive", ErrInvalidTask)
	}
	if log == nil {
		log = zap.NewNop()
	}

	antsPool, err := ants.NewPool(opts.Workers, ants.WithPreAlloc(true))
	if err != nil {
		return nil, fmt.Errorf("create worker pool %s: %w", name, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool := &Pool{
		name:        name,
		workers:     opts.Workers,
		stopTimeout: opts.StopTimeout,
		log:         log,
		ctx:         ctx,
		cancel:      cancel,
		workersPool: antsPool,
		stopDone:    make(chan struct{}),
	}
	return pool, nil
}

// Submit 将任务提交给 ants 执行；当池已满时阻塞等待空闲 worker。
// 任务执行时同时受提交方 context 和 pool 生命周期 context 控制。
func (p *Pool) Submit(ctx context.Context, task Task) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateTask(task); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if p.closed.Load() {
		p.counters.rejected.Add(1)
		return ErrClosed
	}

	p.admissionMu.Lock()
	if p.closed.Load() {
		p.admissionMu.Unlock()
		p.counters.rejected.Add(1)
		return ErrClosed
	}
	p.counters.submitted.Add(1)
	p.counters.queued.Add(1)
	p.inFlight.Add(1)
	p.admissionMu.Unlock()

	if err := p.workersPool.Submit(func() {
		defer p.inFlight.Done()
		p.run(ctx, task)
	}); err != nil {
		p.counters.submitted.Add(-1)
		p.counters.queued.Add(-1)
		p.counters.rejected.Add(1)
		p.inFlight.Done()
		if errors.Is(err, ants.ErrPoolOverload) {
			// 当前池使用阻塞提交，普通池满会等待；该分支保留给未来启用 ants 过载保护时统一映射错误。
			p.log.Warn("worker pool overloaded", p.fields(task, zap.Int("workers", p.workers))...)
			return ErrQueueFull
		}
		if errors.Is(err, ants.ErrPoolClosed) {
			p.closed.Store(true)
			return ErrClosed
		}
		return fmt.Errorf("submit worker pool task %s: %w", p.name, err)
	}
	return nil
}

// Stats 返回任务池计数器的瞬时快照。
func (p *Pool) Stats() Stats {
	return Stats{
		Name:      p.name,
		Workers:   p.workers,
		Submitted: p.counters.submitted.Load(),
		Rejected:  p.counters.rejected.Load(),
		Started:   p.counters.started.Load(),
		Completed: p.counters.completed.Load(),
		Failed:    p.counters.failed.Load(),
		Panicked:  p.counters.panicked.Load(),
		Queued:    p.counters.queued.Load(),
		Running:   p.counters.running.Load(),
		Free:      p.freeWorkers(),
		Waiting:   p.waitingSubmitters(),
		Closed:    p.closed.Load(),
	}
}

// Stop 停止接收新任务，并等待已登记或已接收任务完成。
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

func (p *Pool) freeWorkers() int64 {
	return int64(p.workersPool.Free())
}

func (p *Pool) waitingSubmitters() int64 {
	return int64(p.workersPool.Waiting())
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
