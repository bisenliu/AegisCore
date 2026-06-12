# Design

## Overview

本变更把 auth Redis detached session 清理拆成两个层次：

```text
common/runtime/workerpool
  -> ants-backed bounded-concurrency background task runtime primitive

user-service/internal/features/auth/infrastructure/redis
  -> auth session purge task producer and Redis cleanup implementation
```

`common/runtime/workerpool` 只提供“提交任务、限制并发、满载拒绝、记录 task 生命周期、优雅停止”的 runtime 能力。它不认识 Redis session、user_id、purge key、auth token 或任何 feature 业务语义。

Auth Redis adapter 负责业务相关输入：detach 原用户 session index，生成 purge key，构造 task name 和日志字段，并在 task body 中调用已有 `purgeDetachedUserSessions`。

ants 版本固定为当前最新稳定版本 `github.com/panjf2000/ants/v2 v2.12.1`。实现时应通过 `go get github.com/panjf2000/ants/v2@v2.12.1` 更新 `common/go.mod` 与 `common/go.sum`。

## Common Worker Pool API

推荐公共 API：

```go
package workerpool

type Task struct {
    Name   string
    Fields []zap.Field
    Run    func(context.Context) error
}

type Options struct {
    Name        string
    Workers     int
    StopTimeout time.Duration
}

type Pool struct {
    // unexported fields
}

func New(lc fx.Lifecycle, log *zap.Logger, opts Options) (*Pool, error)
func (p *Pool) Submit(ctx context.Context, task Task) error
func (p *Pool) Stats() Stats
func (p *Pool) Stop(ctx context.Context) error
```

`New` 可以接收 `fx.Lifecycle`，也可以拆成 `New` + `RegisterLifecycle`。实现时优先保持调用方简单：

```go
pool, err := workerpool.New(params.Lifecycle, params.Log, workerpool.Options{...})
```

公共错误：

```go
var (
    ErrClosed       = errors.New("worker pool closed")
    ErrQueueFull    = errors.New("worker pool queue full")
    ErrInvalidTask  = errors.New("worker pool task is invalid")
)
```

`Task.Run` 为空、`Task.Name` 为空或 workers 非正数时应返回明确错误，避免生产环境 silently fallback 到不可预期容量。

## Execution Model

推荐模型：

```text
Submit(ctx, task)
  -> check pool open
  -> submit task wrapper directly into ants pool
  -> ants worker executes task
```

原因：

- ants 负责 worker 并发上限和协程复用，不再用常驻 `workerLoop` 把 ants 退化成 goroutine 容器。
- `ants.WithNonblocking(true)` 负责满载快速失败，避免 auth HTTP 调用方在后台清理高峰期被阻塞。
- `Submit` 可结合 caller context 实现“请求取消则不再提交后台任务”。
- pool 自己统计 submitted、rejected、running、completed、failed 和 panicked。

`Submit` 行为：

- pool 已关闭：返回 `ErrClosed`。
- task 无效：返回 `ErrInvalidTask`。
- ants 有可用 worker：提交并返回 nil。
- ants 已满：返回 `ErrQueueFull`。为了让问题尽快暴露，auth purge pool 使用非阻塞提交；后续如需要排队语义，需要重新设计 SubmitTimeout 或独立 admission control，而不是恢复池中池。
- caller context 已取消：返回 `ctx.Err()`。

执行行为：

- 每个 task 使用 pool 生命周期 context 派生执行 context。
- worker 调用 `task.Run(taskCtx)`。
- task 返回 error：记录 error 日志，增加 failed 计数。
- task panic：recover，记录 error 日志和 stack，增加 panicked 计数。
- task 正常完成：增加 completed 计数。

## Lifecycle

Pool 生命周期分为三段：

1. Running：接收新任务，ants worker 直接执行 task wrapper。
2. Closing：Fx `OnStop` 或 `Stop` 被调用后，停止接收新任务，并等待 ants 中运行中的任务退出。
3. Stopped：ants pool 释放完成，或 shutdown context 取消。

关闭步骤：

```text
OnStop(ctx)
  -> mark closed
  -> ants.ReleaseContext(ctx)
  -> cancel pool context
```

如果 `Options.StopTimeout` 大于 0，可以在 Fx shutdown ctx 外再收窄一个内部 timeout，但不能忽略 Fx 传入的 ctx。关闭超时后返回 error，并记录 pool stats，便于定位还有多少 task 未完成。

Auth session purge worker pool 应作为 auth feature 内的按用途命名专用 provider 暴露，例如 `auth_session_purge_pool`。`NewSessionStore` 只消费这个命名池，不在 adapter 构造函数内部创建池。这样 Fx graph 可以显式呈现后台资源，后续大型系统接入配置、metrics、health 或替换测试池时不需要改 Redis adapter 业务代码。

该 pool provider 需要依赖 named Redis client，即使构造任务池本身不直接访问 Redis，也要通过 Fx 依赖图表达“purge pool drain 时 Redis client 仍必须存活”。Fx 停止时按依赖关系反向执行，从而保证 purge pool OnStop 早于 Redis client OnStop。

## Auth Redis Integration

`SessionStoreParams` 增加：

```go
type SessionStoreParams struct {
    fx.In

    Redis     *rediscache.Client `name:"cache_redis"`
    Cfg       *config.Config
    PurgePool PurgeTaskPool `name:"auth_session_purge_pool"`
}

type SessionPurgePoolParams struct {
    fx.In

    Lifecycle fx.Lifecycle
    Redis     *rediscache.Client `name:"cache_redis"`
    Log       *zap.Logger
}
```

`SessionStore` 增加：

```go
type SessionStore struct {
    redis                *rediscache.Client
    keys                 authdomain.RedisKeyBuilder
    tokenVersionCacheTTL time.Duration
    purgePool            PurgeTaskPool
}
```

`PurgeTaskPool` 是 auth Redis adapter 消费的窄接口，只包含 `Submit` 和 `Stats`，避免把完整 worker pool 生命周期能力泄漏给业务 adapter。

推荐 auth purge pool 默认值：

```go
const (
    deleteAllUserSessionsPurgeWorkers = 4
    deleteAllUserSessionsPurgeStopTimeout = 30 * time.Second
)
```

这些值先作为 auth Redis adapter 内部常量，不新增配置字段。原因是当前只有 auth session purge 一个真实消费者，把配置形态过早放进 `common/runtime/config` 会扩大公共契约。后续如果其他服务也需要 worker pool 运维配置，再单独设计 `runtime.worker_pools` 或服务侧配置映射。

`DeleteAllUserSessions` 调整：

```go
case detachUserSessionsResultDetached:
    task := workerpool.Task{
        Name: "auth.redis.purge_detached_user_sessions",
        Fields: []zap.Field{
            zap.String("user_id", userID),
            zap.String("purge_key", purgeKey),
        },
        Run: func(taskCtx context.Context) error {
            purgeCtx, cancel := context.WithTimeout(taskCtx, deleteAllUserSessionsPurgeTTL)
            defer cancel()
            return r.purgeDetachedUserSessions(purgeCtx, purgeKey, r.keys.AuthSessionPrefix(userID), cutTime)
        },
    }
    if err := r.purgePool.Submit(context.WithoutCancel(ctx), task); err != nil {
        return fmt.Errorf("submit delete user auth sessions purge: %w", err)
    }
    return nil
```

`context.WithoutCancel(ctx)` 可避免 HTTP request cancel 让已 detach 的 purge 无法提交，但不应绕过 pool lifecycle context；task 执行仍受 pool 停止控制。

## Observability

当前仓库没有 metrics 基础设施，所以第一阶段使用两类可观测能力：

- structured logs：task submit rejected、task failed、task panicked、pool stop timeout。
- in-memory stats：`Stats()` 返回 submitted、rejected、started、completed、failed、panicked、queued、running、closed；其中 queued 表示已提交给 ants 但尚未开始执行的任务数，非独立 channel 队列长度。

Auth purge task 日志字段至少包含：

- `task`
- `pool`
- `user_id`
- `purge_key`
- `session_prefix`
- `cut_time`
- `batch_size`
- `error`

日志等级建议：

- 提交失败：由 caller 返回 error，application 现有链路记录 error；pool 也可记录 warn。
- task error：`Error`。
- task panic：`Error` + stacktrace。
- pool stop timeout：`Error`。
- pool stopped cleanly：`Info` 或 `Debug`。

未来接入 Prometheus/OTel 时，可以在 `common/runtime/workerpool` 增加 observer interface 或 exporter，不需要修改 auth Redis adapter 的业务逻辑。

## Failure Semantics

Redis detach 脚本失败：同步返回 error，行为不变。

Detach result empty：返回 nil，行为不变。

Detach result conflict：同步返回 error，行为不变。

Task submit failure：返回 error。此时 purge key 已创建且有 TTL，系统有兜底清理窗口，但请求链路必须知道后台清理没有成功接手。

Task execution failure：请求已经返回，不能再改变 HTTP 响应。通过日志和 stats 暴露，由 purge key TTL 兜底。可以在 task 内添加有限重试，但第一版建议保持简单，先把失败可观测化；如压测或故障演练证明 Redis 瞬时错误常见，再补退避重试。

Pool stop timeout：服务关闭返回 error，Fx 会暴露 shutdown 问题。未完成 purge key 仍由 Redis TTL 兜底。

## Dependency Rules

`common/runtime/workerpool` 可以依赖：

- 标准库
- `github.com/panjf2000/ants/v2`
- `go.uber.org/fx`
- `go.uber.org/zap`
- `common/runtime/logger`，如果需要 stacktrace helper

`common/runtime/workerpool` 禁止依赖：

- `user-service`
- Gin
- Ent
- Redis/PostgreSQL client
- auth/user feature packages
- HTTP response envelope
- business DTO

Auth Redis adapter 可以依赖 `common/runtime/workerpool`，因为它属于 infrastructure/runtime 边界。Application port 不应暴露 worker pool，也不应让 use case 感知 ants。

## Documentation Updates

更新 `docs/ARCHITECTURE.md`：

- Common Organization 增加 `common/runtime/workerpool`，说明它是跨服务稳定后台任务池 runtime primitive。
- Infrastructure 或 Current Constraints 中说明当前 worker pool 只用于 Redis session purge，不是 MQ、eventbus、outbox、通用 job system 或可靠投递框架。
- Dependency Rules 中强调 feature application 不依赖 worker pool，后台清理属于 infrastructure adapter 内部实现细节。

更新 `AGENTS.md`：

- Repository Shape 的 `common/` 分类补充 `runtime/workerpool`。
- Repository Rules 中说明 ants 公共封装放 `common/runtime/workerpool`，服务或 feature 不直接散落裸 goroutine 做长期后台清理。
- 明确不要把 workerpool 用作事件总线、outbox、跨 feature orchestration 或业务 job framework。

## Verification Strategy

Common worker pool 测试：

```bash
cd common
go test ./runtime/workerpool
```

覆盖：

- worker 并发不超过配置。
- pool full 返回 `ErrQueueFull` 并增加 rejected。
- task error 增加 failed 并记录日志。
- task panic 被 recover 并增加 panicked。
- `Stop` 后 `Submit` 返回 `ErrClosed`。
- `Stop` 能等待已提交任务完成。

Auth Redis adapter 测试：

```bash
cd user-service
go test ./internal/features/auth/infrastructure/redis
```

覆盖：

- `DeleteAllUserSessions` 后旧 session 最终被 purge。
- 多批 session 能被后台任务清理。
- detach 后新创建的 session 不被误删。
- purge pool 提交失败时 `DeleteAllUserSessions` 返回 error。
- purge task 执行失败时有 error 统计或日志。

全量范围：

```bash
make test-common
make test-user-service
```

如 `go get` 更新依赖后，应检查 `common/go.mod`、`common/go.sum`、`go.work.sum` 是否有预期变更。

## Risks And Mitigations

### Stop Order Risk

Risk: Redis client 先关闭，purge worker drain 时访问已关闭 client。

Mitigation: 使用 auth feature 内的命名专用 worker pool provider，并让该 provider 显式依赖 named Redis client；通过 Fx stop order 测试确认 worker pool OnStop 早于 Redis client OnStop。

### Pool Saturation

Risk: 高并发登出导致 purge pool 满，后续 request 返回错误。

Mitigation: pool full 必须可观测；purge key TTL 兜底避免永久残留。默认 workers 先设置为 4，后续按压测结果决定是否引入配置或显式 admission control。

### Common Abstraction Drift

Risk: `common/runtime/workerpool` 被扩展成带业务语义的 job framework。

Mitigation: API 只接受 `Task` 和 runtime options，不引入业务 payload、retry policy DSL、event topic 或 persistence。

### Error Noise

Risk: Redis 瞬时失败导致大量 task error 日志。

Mitigation: 第一版先保证失败可见；如果噪声真实发生，再追加有限重试和日志采样，避免一开始把可靠投递语义做重。

### ants API Mismatch

Risk: ants 最新版本 API 与预期不同。

Mitigation: 实现前以 `go doc` 或本地 module cache 校验 `ants.NewPool`、`Submit`、`ReleaseContext` 和 options 名称；以 `go test ./runtime/workerpool` 作为 API 集成检查。
