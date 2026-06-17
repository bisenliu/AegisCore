# Export runtime dependency metrics

## What

将用户服务已有运行时依赖状态接入 Prometheus metrics endpoint，覆盖 PostgreSQL、Redis、auth session purge workerpool、scheduler、RBAC policy watcher 和 Casbin policy reload 状态。

本变更在既有 `common/runtime/observability/metrics` provider 与用户服务 `/metrics` endpoint 基础上补齐 runtime dependency collector：

- 使用 `db.Stats()` 导出 PostgreSQL 连接池状态，资源 label 固定为 `user_db`。
- 使用 Redis `PING` 导出 `cache_redis` 可用性和 ping 延迟，不记录 key、command 参数或业务 payload。
- 使用 `workerpool.Stats()` 导出 auth session purge pool 的 submitted、rejected、started、completed、failed、panicked、queued、running、free、waiting 和 closed 状态。
- 实现 `common/runtime/scheduler.Metrics` 的 Prometheus adapter，导出 job registered、triggered、started、completed、failed、skipped、lock renew failed 和 duration。
- 导出 RBAC policy watcher 的 running 状态和 last error 状态。
- 为 Casbin policy reload 增加成功/失败计数或最近状态指标。

## Why

用户服务已经能够暴露 Prometheus `/metrics`，HTTP server RED 指标和 Go runtime/process 指标也已有基础，但关键依赖的运行状态仍主要依赖 health probe、日志或内存状态：

- PostgreSQL 连接池等待、占用和空闲状态只能在本地通过 `db.Stats()` 查看。
- Redis 可用性只在启动或 health check 里体现，缺少可 scrape 的延迟和状态。
- auth session purge workerpool 已有稳定 `Stats()`，但没有进入 `/metrics`。
- scheduler 已有 `Metrics` 接口，但当前默认 `NopMetrics`，Prometheus adapter 尚未落地。
- RBAC policy watcher 与 Casbin reload 失败会影响授权策略同步，但目前只通过 health/status 和日志观察。

这些状态都属于服务运行时依赖和基础设施健康信号，不是 auth/user/role/permission 业务指标。将它们以低基数 Prometheus 指标导出，可以让部署侧持续观察容量、积压、失败和策略同步状态，同时保持现有 health probe 语义不变。

## Scope

包括：

- 在 `common/runtime/observability/metrics` 增加低基数 runtime dependency label 约定，例如 `resource`、`pool`、`job`、`event`、`status`、`reason`。
- 为 PostgreSQL `*sql.DB` 连接池实现 Prometheus collector 或注册 helper。
- 为 Redis `*redis.Client` 实现可配置 timeout 的 availability/ping latency collector 或 probe helper。
- 为 `common/runtime/workerpool` 实现只读 Prometheus collector，基于 `Stats()` 快照导出计数和 gauge。
- 为 `common/runtime/scheduler.Metrics` 实现 Prometheus adapter；adapter 位于 metrics package 或服务侧 wiring，不放入 scheduler 执行核心。
- 在 user-service provider 层注册 `user_db`、`cache_redis` 和 `auth_session_purge_pool` 的 metrics collector。
- 在 permission feature 或 provider 层接入 RBAC watcher 和 Casbin policy reload 状态 metrics。
- 为 Casbin policy reload 增加最小 metrics hook，记录 reload success/failure 计数或最近一次成功状态；不记录错误消息全文。
- 所有 label 只使用固定枚举式资源名、任务名、事件名或状态值。
- 更新 `common/runtime/observability/metrics/README.md`、`docs/ARCHITECTURE.md` 和必要开发文档，说明指标与 label 基数边界。
- 增加单元测试或 provider wiring 测试，覆盖 collector 注册、disabled provider 零副作用、低基数 label、workerpool/scheduler 适配和 RBAC 状态指标。

不包括：

- 不新增通用 eventbus、outbox、worker system 或可靠投递框架。
- 不改变 workerpool、scheduler、RBAC watcher 或 Casbin enforcer 的核心执行语义。
- 不改变 `/livez`、`/readyz`、`/startupz` 的响应契约或检查逻辑。
- 不新增 dashboard、alert rules、ServiceMonitor、PodMonitor、Helm chart 或 Kubernetes 部署资产。
- 不新增 auth/user/role/permission 业务指标。
- 不在 label 中放用户 ID、角色 ID、权限 ID、session ID、token ID、trace ID、Redis key、SQL、raw path、错误消息全文或其他高基数/敏感值。
- 不修改数据库 schema、Ent generated code、Atlas migration、Redis key schema 或业务 API response。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `observability.metrics.enabled: true` 时，`GET /metrics` 能看到 PostgreSQL、Redis、workerpool、scheduler、RBAC watcher 和 Casbin policy reload 相关指标。
- PostgreSQL 指标至少覆盖 open、in-use、idle、wait count、wait duration、max open 等 `db.Stats()` 稳定字段，`resource` label 固定为 `user_db`。
- Redis 指标至少覆盖 `cache_redis` 可用性或 ping 延迟，且不暴露 key、command 参数、payload、DSN 或错误消息全文。
- Auth session purge workerpool 指标覆盖 submitted、rejected、started、completed、failed、panicked、queued、running、free、waiting 和 closed。
- Scheduler Prometheus adapter 覆盖 registered、triggered、started、completed、failed、skipped、lock renew failed 和 job duration。
- RBAC watcher 指标表达 running 状态和 last error 状态；Casbin policy reload 指标表达 reload 成功/失败计数或最近状态。
- 指标 label 基数受控，并在文档或测试中明确禁止高基数/敏感 label。
- Metrics disabled 时 collector 注册保持零副作用，不启动 Redis probe 后台任务。
- workerpool 和 scheduler 现有测试不受破坏。
- 实现后 `make test-common` 和 `make test-user-service` 通过，或明确说明外部依赖导致未能运行。
