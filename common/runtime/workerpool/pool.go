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
//
// 配置项含义如下：
//   - Name 是任务池的稳定名称，会裁剪首尾空白，不能为空。它会进入日志和 metrics，应该使用
//     auth.session_purge 等低基数名称，不能包含 user ID、请求 ID 等动态内容。
//   - Workers 是最多同时执行的任务数，必须大于零。当前实现预分配 ants worker，并在所有 worker
//     忙碌时让 Submit 阻塞等待；它不是可配置容量的消息队列。
//   - StopTimeout 是每次 Stop 调用等待 drain 的内部上限。大于零时，它与 Stop 调用方 context
//     中更早到期的 deadline 共同生效；小于等于零时只使用调用方 context。
//   - log 可以为 nil，此时使用 zap no-op logger；生产环境应传入真实 logger，以便观察任务失败、
//     panic 和停止异常。
//
// 用法一：创建任务池、提交任务并在资源关闭边界停止。
//
// Submit 返回 nil 只表示任务已被任务池接收，不表示 Task.Run 已经执行完成。任务的执行错误通过日志
// 和 Stats 暴露，不会异步返回给 Submit 调用方。
//
//	pool, err := workerpool.New(log, workerpool.Options{
//		Name:        "document.thumbnail",
//		Workers:     8,
//		StopTimeout: 10 * time.Second,
//	})
//	if err != nil {
//		return err
//	}
//
//	err = pool.Submit(ctx, workerpool.Task{
//		Name: "generate_thumbnail",
//		Fields: []zap.Field{
//			zap.String("document_id", documentID),
//		},
//		Run: func(taskCtx context.Context) error {
//			return thumbnailService.Generate(taskCtx, documentID)
//		},
//	})
//	if err != nil {
//		return err
//	}
//
//	// 在应用或拥有该 pool 的组件停止时调用；不要在每次 Submit 后调用。
//	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
//	defer cancel()
//	return pool.Stop(stopCtx)
//
// Pool 创建后立即可用，不需要 Start。创建方拥有 Pool 生命周期，即使尚未提交任务也必须在应用、
// Fx provider 或其他资源所有者的关闭边界显式调用 Stop；不要仅依赖进程退出回收资源。
//
// 用法二：让后台任务跟随提交方 context。
//
// Task.Run 收到的 context 同时关联 Submit 的 context 和 pool 生命周期。提交方在任务运行期间取消
// context 时，taskCtx.Done() 会关闭；取消只是协作通知，不会强杀 goroutine，因此 Task.Run 必须
// 检查 context，并把它传给数据库、Redis、HTTP 等下游调用。
//
//	err = pool.Submit(requestCtx, workerpool.Task{
//		Name: "rebuild_preview",
//		Run: func(taskCtx context.Context) error {
//			for hasNextBatch() {
//				if err := taskCtx.Err(); err != nil {
//					return err
//				}
//				if err := rebuildNextBatch(taskCtx); err != nil {
//					return err
//				}
//			}
//			return nil
//		},
//	})
//	if err != nil {
//		return err
//	}
//
// 如果任务本来就应该在 HTTP 请求结束后继续运行，不要直接传 requestCtx。可以使用
// context.WithoutCancel(requestCtx) 保留 trace 等 context value、移除请求取消，再在 Task.Run 内
// 设置任务自己的 deadline。不要在 Submit 返回后立即 cancel 作为 parent 传入的 context，否则刚被
// 接收的异步任务也会被取消。
//
//	backgroundCtx := context.WithoutCancel(requestCtx)
//	err = pool.Submit(backgroundCtx, workerpool.Task{
//		Name: "purge_expired_sessions",
//		Run: func(poolCtx context.Context) error {
//			taskCtx, cancel := context.WithTimeout(poolCtx, 30*time.Second)
//			defer cancel()
//			return purgeExpiredSessions(taskCtx)
//		},
//	})
//	if err != nil {
//		return err
//	}
//
// 用法三：理解满载时的背压。
//
// Workers=2 时最多同时运行两个任务。第三个及后续 Submit 会阻塞，直到有 worker 空闲或 Pool 被
// Stop；当前 Options 没有 queue capacity、非阻塞提交或 admission timeout 配置。Submit 的 context
// 会在进入等待前检查，但在等待空闲 worker 的过程中取消 context 不会中断该次阻塞；任务最终被接收
// 后会携带已经取消的 context 开始执行，或者在 Pool 停止竞态中返回 ErrClosed。
//
// 因此只能在允许同步背压的调用路径直接 Submit，不能把它当成“请求立即返回、后台无限排队”的队列。
// 如果入口不能阻塞，应在调用方设计明确的过载策略或选择具备有界队列和可取消 admission 的组件，
// 不要通过无界 goroutine 包裹 Submit。ErrQueueFull 保留用于映射 ants 过载错误，但当前阻塞配置不会
// 仅因为所有 worker 正忙就返回 ErrQueueFull。
//
// 用法四：处理提交错误、任务错误和 panic。
//
//   - Task.Name 裁剪后必须非空，Task.Run 不能为空，否则 Submit 返回 ErrInvalidTask。
//   - Submit 调用前 context 已取消时直接返回 ctx.Err()；Pool 已开始 Stop 后返回 ErrClosed。
//   - Task.Run 返回 error 时，Submit 的返回值不会改变；任务计入 Failed，并以 task.Fields 记录
//     worker pool task failed 日志。workerpool 不自动重试。
//   - Task.Run panic 会在 worker 边界恢复，计入 Panicked 并记录 panic 和 stacktrace；它不会让
//     进程崩溃，也不会自动重试。该任务不计入 Completed 或 Failed。
//   - Task.Run 返回 nil 才计入 Completed。
//
// Task.Name 应是稳定操作名，Task.Fields 用于提供本次任务的定位字段；不要把密码、token、完整 Redis
// key 或其他敏感值写入 Fields。需要让业务调用方获知执行结果时，应在业务层设计明确的结果通道或状态
// 存储，不能把 Submit 的 nil 当成业务成功。
//
// 用法五：优雅停止和超时后的继续 drain。
//
// 第一次 Stop 会原子地关闭准入，使后续 Submit 返回 ErrClosed，并在后台启动一份所有 Stop 调用共享
// 的 drain。正常停止会等待已经登记或接收的任务自然结束，不会立刻取消正在运行的 Task context。
//
// 如果 StopTimeout 或调用方 context 先到期，本次 Stop 返回包装后的 context.Canceled 或
// context.DeadlineExceeded，同时取消 pool 生命周期 context，通知仍在运行的任务尽快退出。它不会
// 强杀忽略 context 的任务，后台 drain 仍会继续；后续 Stop 可以重新等待同一份 drain 完成状态。
//
//	if err := pool.Stop(stopCtx); err != nil {
//		if errors.Is(err, context.DeadlineExceeded) {
//			log.Warn("worker pool is still draining", zap.Error(err))
//		} else {
//			return err
//		}
//	}
//	// 必要时可在更外层的关闭预算内再次等待；这不会启动第二次 drain。
//	return pool.Stop(finalStopCtx)
//
// Stop 可重复并发调用。不要从 Pool 自己的 Task.Run 内调用 Stop：Stop 要等待包括当前任务在内的
// in-flight 任务，可能形成自等待；应由任务池的资源所有者统一停止。StopTimeout<=0 且调用方使用
// context.Background() 时，如果某个任务永久忽略 context，Stop 也可能永久等待。
//
// 用法六：读取任务池统计。
//
//	stats := pool.Stats()
//	log.Info("worker pool snapshot",
//		zap.String("pool", stats.Name),
//		zap.Int("workers", stats.Workers),
//		zap.Int64("submitted", stats.Submitted),
//		zap.Int64("rejected", stats.Rejected),
//		zap.Int64("started", stats.Started),
//		zap.Int64("completed", stats.Completed),
//		zap.Int64("failed", stats.Failed),
//		zap.Int64("panicked", stats.Panicked),
//		zap.Int64("queued", stats.Queued),
//		zap.Int64("running", stats.Running),
//		zap.Int64("free", stats.Free),
//		zap.Int64("waiting", stats.Waiting),
//		zap.Bool("closed", stats.Closed),
//	)
//
// Submitted、Rejected、Started、Completed、Failed、Panicked 是累计计数；Queued、Running、Free、
// Waiting 和 Closed 是当前状态。Submit 阻塞等待 worker 时已经临时登记到 Submitted 和 Queued；
// 如果 ants 最终拒绝该任务，会回滚这两个计数并增加 Rejected。ErrInvalidTask 和进入准入前已经取消的
// context 不计入 Rejected。Waiting 是 ants 报告的等待提交方数量。各字段分别读取，在高并发变化期间
// 不保证对应同一个精确时间点。Pool 实现 StatsSource，可直接交给 metrics collector。
//
// workerpool 是单进程、内存内的并发限制 primitive，不持久化任务，不保证执行顺序、重试、exactly-once
// 或进程崩溃后的恢复。可靠投递应使用 outbox、MQ 或对应业务基础设施；周期任务应使用 scheduler。
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
// admissionMu 将关闭检查、计数登记和 inFlight 登记串行化，确保 Stop 不会漏等已经通过准入但尚未进入 ants 的任务。
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
	// 先释放 ants 阻止新任务进入，再等待 inFlight；stopErr 在 stopDone 关闭前写入，等待方读取时已有 happens-before 保证。
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
