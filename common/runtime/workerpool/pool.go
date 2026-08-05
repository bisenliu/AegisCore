package workerpool

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

// Pool 使用 ants 原生池执行后台任务，并通过固定容量限制并发。
// 创建方拥有 Pool 生命周期，必须在资源关闭边界显式调用 Stop。
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

// New 创建一个固定最大并发数的进程内后台任务池。
// ants.WithPreAlloc 只预分配内部 worker queue，不会预先启动 worker goroutine。
// 完整的配置、context、背压、错误、统计和关闭契约参见 package workerpool 文档与 ExamplePool。
func New(log *zap.Logger, opts Options) (*Pool, error) {
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

	// 将关闭检查和 in-flight 登记串行化，确保 Stop 不会漏等已经通过准入但尚未进入 ants 的任务。
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
			// 当前池使用阻塞提交；该分支保留给 ants 自身过载错误的稳定映射。
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
