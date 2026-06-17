# Design

## Overview

本变更在已有 Prometheus provider 和 `/metrics` endpoint 上增加 runtime dependency metrics：

```text
common/runtime/observability/metrics
  -> SQL DB stats collector
  -> Redis ping collector
  -> workerpool stats collector
  -> scheduler Metrics adapter
  -> shared low-cardinality labels

user-service/internal/providers
  -> register user_db/cache_redis/auth_session_purge_pool collectors
  -> register scheduler adapter when scheduler resources exist

permission feature / provider wiring
  -> RBAC watcher status collector
  -> Casbin policy reload metrics hook
```

核心原则：

- metrics adapter 只读取已有稳定状态或已有接口，不改变 runtime primitive 行为。
- common package 承载无业务语义 adapter；user-service provider 决定具名资源和具体注册。
- label 必须低基数、固定枚举式、可聚合。
- health probe 继续表达 readiness/startup 语义；metrics 只表达可观测信号。
- metrics disabled 时零副作用，不创建后台 ping loop。

## Current State

已有基础：

- `common/runtime/observability/metrics.Provider` 使用独立 Prometheus registry，disabled 模式零副作用。
- 用户服务已通过 router 暴露配置化 metrics endpoint，默认 `/metrics`。
- `common/http/middleware.HTTPServerMetrics` 已接入 HTTP server RED 指标。
- PostgreSQL provider 暴露具名 `*sql.DB`：`user_db`。
- Redis provider 暴露具名 `*redis.Client`：`cache_redis`。
- `common/runtime/workerpool.Stats` 已提供 submitted、rejected、started、completed、failed、panicked、queued、running、free、waiting 和 closed。
- `common/runtime/scheduler.Metrics` 已定义 registered、triggered、started、completed、failed、skipped、lock renew failed 和 duration 上报点。
- RBAC policy watcher 已暴露 `Running()` 和 `LastError()`。
- Casbin policy engine 已暴露 `Reload(ctx)` 和 `LastError()`。

约束：

- `common/runtime/observability/metrics` 不导入 user-service、Gin、Ent 或 feature 包。
- datastore collector 可以依赖标准库 `database/sql` 和 `github.com/redis/go-redis/v9`，但只读取基础状态或执行 `PING`。
- scheduler 和 workerpool 不反向依赖 Prometheus。
- 代码注释使用中文；日志消息如新增必须使用英文。
- 不新增 OpenSpec/OPSX 工件。

## Label Contract

扩展 common metrics label 常量：

```go
const (
    LabelResource = "resource"
    LabelPool     = "pool"
    LabelJob      = "job"
    LabelEvent    = "event"
    LabelStatus   = "status"
    LabelReason   = "reason"
)
```

允许值：

| Label | 示例 | 约束 |
|---|---|---|
| `resource` | `user_db`, `cache_redis` | 只能是配置/代码固定资源名 |
| `pool` | `auth_session_purge_pool` | 只能是按用途命名的固定 pool |
| `job` | `rbac_policy_version_check` | 只能是 scheduler 注册时的固定 job key |
| `event` | `submitted`, `failed`, `lock_renew_failed` | 固定枚举 |
| `status` | `success`, `failure`, `running`, `stopped`, `closed`, `open` | 固定枚举 |
| `reason` | `local_overlap`, `global_concurrency_limit`, `lock_error`, `lock_busy` | 固定枚举，不得使用错误文本 |

继续禁止：

- 用户 ID、角色 ID、权限 ID、session ID、token ID。
- trace ID、span ID、request ID、IP、User-Agent。
- Redis key、SQL、DSN、raw path、URL query、email、username。
- JWT、Authorization header、Cookie、原始错误消息全文。

测试应覆盖 label key 常量，并通过指标样本断言资源名为固定值。

## PostgreSQL Collector

在 common metrics package 增加 `SQLDBCollector` 或 `NewSQLDBCollector`：

```go
type SQLDBCollectorOptions struct {
    Resource string
    DB       *sql.DB
}
```

Collector 每次 scrape 调用 `DB.Stats()`，导出：

| Metric | Type | Labels | Source |
|---|---|---|---|
| `aegiscore_postgres_pool_open_connections` | gauge | `resource` | `OpenConnections` |
| `aegiscore_postgres_pool_in_use_connections` | gauge | `resource` | `InUse` |
| `aegiscore_postgres_pool_idle_connections` | gauge | `resource` | `Idle` |
| `aegiscore_postgres_pool_wait_count_total` | counter | `resource` | `WaitCount` |
| `aegiscore_postgres_pool_wait_duration_seconds_total` | counter | `resource` | `WaitDuration.Seconds()` |
| `aegiscore_postgres_pool_max_open_connections` | gauge | `resource` | `MaxOpenConnections` |
| `aegiscore_postgres_pool_max_idle_closed_total` | counter | `resource` | `MaxIdleClosed` |
| `aegiscore_postgres_pool_max_idle_time_closed_total` | counter | `resource` | `MaxIdleTimeClosed` |
| `aegiscore_postgres_pool_max_lifetime_closed_total` | counter | `resource` | `MaxLifetimeClosed` |

说明：

- `resource` 在 user-service 中固定传 `resources.NameUserDB`。
- Collector 不执行 SQL，不读取 DSN，不改变 pool 配置。
- `WaitCount` 和 closed counters 来自单调累积字段，可作为 Prometheus counter 导出。

## Redis Collector

Redis 基础可观测性推荐使用 scrape-time ping collector，避免新增后台 goroutine：

```go
type RedisPingCollectorOptions struct {
    Resource string
    Client   *redis.Client
    Timeout  time.Duration
}
```

每次 scrape：

1. 创建带 timeout 的 context。
2. 调用 `Client.Ping(ctx).Err()`。
3. 记录 availability 和 duration。

指标：

| Metric | Type | Labels | Semantics |
|---|---|---|---|
| `aegiscore_redis_up` | gauge | `resource` | 成功为 1，失败为 0 |
| `aegiscore_redis_ping_duration_seconds` | gauge 或 histogram | `resource` | 最近一次 scrape 的 ping 耗时，或 ping latency histogram |
| `aegiscore_redis_ping_failures_total` | counter | `resource` | ping 失败累计次数 |

设计取舍：

- scrape-time ping 简单且无后台生命周期；如果未来 scrape 量大或 Redis 延迟要求更严格，可另开变更改为后台缓存状态。
- timeout 使用 Redis 配置 `PingTimeout` 或保守默认值，必须为正值。
- 不将 Redis command、key、error message、addr 或 DB number 放入 label。
- 失败原因可以体现在日志或 health check，metrics 只导出固定 `up=0` 和失败计数。

## Workerpool Collector

在 common metrics package 增加只读 workerpool collector：

```go
type WorkerpoolStatsSource interface {
    Stats() workerpool.Stats
}

type WorkerpoolCollectorOptions struct {
    Pool string
    Source WorkerpoolStatsSource
}
```

指标：

| Metric | Type | Labels | Source |
|---|---|---|---|
| `aegiscore_workerpool_tasks_total` | counter | `pool`,`event` | submitted/rejected/started/completed/failed/panicked |
| `aegiscore_workerpool_queued` | gauge | `pool` | queued |
| `aegiscore_workerpool_running` | gauge | `pool` | running |
| `aegiscore_workerpool_free` | gauge | `pool` | free |
| `aegiscore_workerpool_waiting` | gauge | `pool` | waiting |
| `aegiscore_workerpool_closed` | gauge | `pool` | closed bool |
| `aegiscore_workerpool_workers` | gauge | `pool` | workers |

`pool` 使用 `Stats().Name` 或显式固定 options 值。user-service 当前只注册 `auth_session_purge_pool`。

## Scheduler Metrics Adapter

实现 `scheduler.Metrics` 的 Prometheus adapter，建议放在 common metrics package：

```go
func NewSchedulerMetrics(provider *Provider, opts SchedulerMetricsOptions) scheduler.Metrics
```

当 provider nil 或 disabled 时返回 `scheduler.NopMetrics{}`。

指标：

| Metric | Type | Labels |
|---|---|---|
| `aegiscore_scheduler_jobs_total` | counter | `job`,`event`,`status`,`reason` |
| `aegiscore_scheduler_job_duration_seconds` | histogram | `job`,`status` |

映射：

- `JobRegistered(job)` -> event `registered`, status `success`。
- `JobTriggered(job)` -> event `triggered`, status `success`。
- `JobStarted(job)` -> event `started`, status `success`。
- `JobCompleted(job,duration)` -> event `completed`, status `success` + duration status `success`。
- `JobFailed(job,duration)` -> event `failed`, status `failure` + duration status `failure`。
- `JobSkipped(job,reason)` -> event `skipped`, status `skipped`, reason 固定为 scheduler 现有枚举。
- `JobLockRenewFailed(job)` -> event `lock_renew_failed`, status `failure`。

为避免可变 label 组合过多，`reason` 对非 skipped/lock failure 事件可固定为 `none`。Adapter 应记录固定枚举，不接收原始 error。

## RBAC Watcher And Casbin Reload Metrics

RBAC watcher 状态可以作为 user-service 或 permission infrastructure collector 注册：

```go
type WatcherStatus interface {
    Running() bool
    LastError() error
}
```

指标：

| Metric | Type | Labels | Semantics |
|---|---|---|---|
| `aegiscore_rbac_policy_watcher_running` | gauge | none 或 `resource=rbac_policy_watcher` | running 为 1，stopped 为 0 |
| `aegiscore_rbac_policy_watcher_last_error` | gauge | none 或 `resource=rbac_policy_watcher` | LastError 非 nil 为 1，否则 0 |

Casbin policy reload 有两种实现路径：

1. 在 `permission/infrastructure/casbin.Engine` 增加可选 metrics recorder，`Reload` 成功/失败时记录。
2. 在 permission application `PolicyRefreshCoordinator` 和 watcher remote reload wrapper 处记录 reload 结果。

推荐路径 1：engine 是所有 reload 的汇聚点，包括初始化、在线管理接口本实例 reload 和 watcher 远端 reload。增加一个窄接口，避免 Casbin package 直接依赖 Prometheus：

```go
type ReloadMetrics interface {
    ReloadSucceeded()
    ReloadFailed()
    SetLastStatus(success bool)
}
```

user-service 或 permission fx provider 提供 Prometheus 实现；未提供时使用 nop。

指标：

| Metric | Type | Labels |
|---|---|---|
| `aegiscore_casbin_policy_reloads_total` | counter | `status` |
| `aegiscore_casbin_policy_reload_last_status` | gauge | `status` |

`status` 只能为 `success` 或 `failure`。不记录 error message、role ID、permission ID 或 route template。

## User-Service Wiring

建议在 `user-service/internal/providers` 新增 metrics collector provider 文件，例如 `metrics_collectors.go`：

- 接收 `*commonmetrics.Provider`。
- 接收 `*sql.DB name:"user_db"`。
- 接收 `*redis.Client name:"cache_redis"`。
- 接收 `*config.Config`，读取 Redis ping timeout。
- 接收 auth session purge pool 的窄接口或 `workerpool.Stats()` source。
- 接收 RBAC watcher status。
- 注册 PostgreSQL、Redis、workerpool、RBAC watcher collectors。

如果 auth purge pool 当前只在 auth Redis infrastructure 内部以命名 provider 暴露，需要保持 provider 输出可注入到服务级 metrics wiring；不要让 common collector 导入 auth feature 业务类型。

Scheduler adapter 接入点取决于用户服务是否已经创建 scheduler instance：

- 如果当前没有真实 scheduler job，只实现 common adapter 和测试，不注册用户服务 scheduler 指标。
- 如果已有 scheduler provider，构造 scheduler 时注入 `NewSchedulerMetrics(provider, ...)`，替换 `NopMetrics`。

Casbin reload metrics 在 permission feature Fx 中注入更自然，因为 `Engine` 属于 permission infrastructure。实现时保持指标 recorder 接口最小化。

## Metric Registration

所有 collector 通过 `Provider.Register` 注册：

- provider disabled 时返回 nil，零副作用。
- duplicate collector registration 由 provider 现有保护处理。
- 注册失败应让 Fx 启动失败，避免服务在指标半注册状态运行。

Redis ping collector 不应在构造时 ping Redis；只在 scrape 时执行，避免改变启动依赖语义。当前启动和 health check 已经负责 Redis 可用性判定。

## Tests

Common package tests：

- SQL collector 使用 mock 或真实 `sql.DB` 测试 `Describe/Collect` 基础行为；可通过设置 pool 参数和 `Stats()` 可观察字段断言。
- Redis collector 使用 fake Redis client 不容易时，可抽象最小 ping interface 并用 fake 覆盖成功、失败和 timeout；生产 adapter 再包一层 go-redis client。
- Workerpool collector 使用 fake stats source 覆盖 counters/gauges。
- Scheduler adapter 调用各 `Metrics` 方法后 gather，断言 counter/histogram label 与数值。
- Disabled provider 返回 `NopMetrics` 或 collector 注册零副作用。
- Label constants 和禁止高基数文档测试保持同步。

User-service tests：

- Provider wiring test：metrics enabled 时 `GET /metrics` 包含 `user_db`、`cache_redis`、`auth_session_purge_pool` 或 RBAC watcher 指标。
- Metrics disabled 时不注册 collector 或不会额外暴露指标。
- RBAC watcher status collector：running/last error 映射为 1/0。
- Casbin engine reload success/failure 更新 metrics recorder，reload failure 不泄漏错误消息 label。
- Existing workerpool 和 scheduler tests 继续通过。

## Risks / Trade-offs

- Scrape-time Redis ping 会让每次 Prometheus scrape 产生一次 Redis round trip。当前指标范围小、scrape 频率可控，优先选择无后台生命周期的简单实现。
- PostgreSQL collector 只反映连接池状态，不代表 SQL 查询成功率；readiness 仍由 health check 负责。
- `scheduler.JobSkipped` 的 `reason` 来自现有固定字符串，后续新增 reason 必须保持枚举式。
- Casbin reload metrics 若接在 engine 内，会覆盖所有 reload 来源，但需要给 engine 构造函数新增可选依赖和测试。
- Metrics endpoint 不经过 RBAC，部署侧仍需负责网络侧保护。
