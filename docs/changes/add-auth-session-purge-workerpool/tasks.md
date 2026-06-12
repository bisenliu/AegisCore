# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只引入受控后台任务池和 auth Redis purge 接入。
- [x] 使用 `go list -m -versions github.com/panjf2000/ants/v2` 或等价命令确认当前最新稳定版本仍为 `v2.12.1`；如果已有更新，更新本 change 文档并使用新的最新稳定版本。
- [x] 检查 `common/runtime` 现有 Fx provider、logger 和 datastore lifecycle 风格。
- [x] 检查 `user-service/internal/features/auth/infrastructure/redis/session_store.go` 当前 `DeleteAllUserSessions`、`purgeDetachedUserSessions` 和相关测试。
- [x] 确认本次不修改 HTTP API、application port 方法签名、Redis key schema、Lua detach 语义、Ent schema、migration、eventbus 或 outbox。

## Common Worker Pool

- [x] 在 `common` 模块执行 `go get github.com/panjf2000/ants/v2@v2.12.1`，更新 `common/go.mod`、`common/go.sum` 和必要的 workspace sum 文件。
- [x] 新增 `common/runtime/workerpool` 包。
- [x] 定义 `Task`、`Options`、`Stats`、`Pool` 和公共错误 `ErrClosed`、`ErrQueueFull`、`ErrInvalidTask`。
- [x] 基于 ants 实现固定 worker 并发。
- [x] 使用 ants 非阻塞提交实现池满返回 `ErrQueueFull`。
- [x] 实现 `Submit(ctx, task)`：校验 task、检查 pool 状态、尊重 caller context、提交成功后更新 submitted/queued stats。
- [x] 实现 task 执行 wrapper：设置 running stats、recover panic、记录 task error、更新 completed/failed/panicked stats。
- [x] 实现 `Stats()` 返回线程安全快照。
- [x] 实现 `Stop(ctx)`：停止接收新任务、等待 ants 已接收任务退出、取消 pool context 并返回关闭错误。
- [x] 实现 Fx lifecycle 注册，确保 `OnStop` 调用 `Stop(ctx)` 并记录关闭日志。
- [x] 保持 `common/runtime/workerpool` 不依赖 user-service、Gin、Ent、Redis/PostgreSQL client 或业务 DTO。

## Auth Redis Integration

- [x] 更新 `SessionStoreParams`，注入 named Redis client、config 和 `auth_session_purge_pool` 命名专用池。
- [x] 更新 `SessionStore`，持有 `PurgeTaskPool` 窄接口。
- [x] 在 auth Redis infrastructure 中提供 `NewSessionPurgePool`，配置固定 workers 和 stop timeout。
- [x] 在 auth feature module 中以 `auth_session_purge_pool` 命名注入 purge pool。
- [x] 为 auth purge pool 添加内部常量：workers、stop timeout；不新增配置字段。
- [x] 将 `DeleteAllUserSessions` 中的裸 `go func()` 替换为 `purgePool.Submit(...)`。
- [x] purge task 使用 pool lifecycle context 派生 `context.WithTimeout(..., deleteAllUserSessionsPurgeTTL)`。
- [x] purge task 记录 `task`、`user_id`、`purge_key`、`session_prefix`、`cut_time`、`batch_size` 等字段。
- [x] task 提交失败时从 `DeleteAllUserSessions` 返回 `submit delete user auth sessions purge` 包装错误。
- [x] 保持 `detachUserSessionsResultEmpty`、`Detached`、`Conflict` 和 unexpected result 的现有业务语义。
- [x] 确认 application command/use case 不感知 worker pool 或 ants。

## Tests

- [x] 新增 common worker pool 测试：并发上限。
- [x] 新增 common worker pool 测试：pool full 返回 `ErrQueueFull` 并更新 rejected stats。
- [x] 新增 common worker pool 测试：task error 更新 failed stats 并输出 error log。
- [x] 新增 common worker pool 测试：task panic 被 recover，更新 panicked stats 并输出 error log。
- [x] 新增 common worker pool 测试：`Stop` 后 `Submit` 返回 `ErrClosed`。
- [x] 新增 common worker pool 测试：`Stop` 等待已提交任务完成或按 context 超时返回。
- [x] 新增 common worker pool 测试：Fx lifecycle stop hook 注册并关闭 pool。
- [x] 更新 auth Redis session store 测试 helper，使其可创建带 lifecycle/logger/purge pool 的 store。
- [x] 保留并通过现有 `DeleteAllUserSessions`、批量 purge、新 session 不误删测试。
- [x] 新增 auth Redis 测试：purge pool 提交失败时 `DeleteAllUserSessions` 返回 error。
- [x] 新增 auth Redis 测试：purge task 执行失败时可通过 stats 或 observer logger 断言 error 可观测。
- [x] 如可行，补充 Fx lifecycle 顺序测试，确认 auth purge pool OnStop 早于 Redis client OnStop。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md` Common Organization，说明 `common/runtime/workerpool` 是跨服务后台任务池 runtime primitive。
- [x] 更新 `docs/ARCHITECTURE.md` Current Constraints 或 Infrastructure，说明该 worker pool 当前只用于 auth Redis purge，不是 MQ、eventbus、outbox 或可靠投递框架。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules，说明 feature application 不依赖 worker pool，后台清理属于 infrastructure adapter 实现细节。
- [x] 更新 `AGENTS.md` Repository Shape，补充 `common/runtime/workerpool`。
- [x] 更新 `AGENTS.md` Repository Rules，说明 ants 公共封装放 `common/runtime/workerpool`，长期后台清理不得散落裸 goroutine。
- [x] 确认文档没有重新引入 `openspec/` 或 `docs/opsx/`。

## Optimization Follow-up

- [x] 评估用户提出的“池中池”问题，确认常驻 `workerLoop` 会让 ants 退化为 goroutine 容器。
- [x] 移除 `common/runtime/workerpool` 中的自建 task channel、常驻 worker loop 和额外 WaitGroup 生命周期。
- [x] 改为 `Submit` 直接调用 ants pool，使用 `ants.WithNonblocking(true)` 将满载快速失败映射为 `ErrQueueFull`。
- [x] 改为 `Stop` 使用 ants 原生 `ReleaseContext`，并继续通过 pool context 取消运行中的任务。
- [x] 移除 auth purge pool 的 queue size 常量，保留 workers 和 stop timeout。
- [x] 将本轮新增或触碰的 workerpool 注释调整为中文。
- [x] 更新 change 文档、`AGENTS.md` 和 `docs/ARCHITECTURE.md`，避免继续描述旧的 bounded channel 模型。
- [x] 评估二次优化方案：保留 `Submit`/`Stop` 之间用于线性化关闭状态的锁，不采用成功提交后再增加 queued 的竞态计数方式。
- [x] 将 task 执行 context 调整为提交方 context 与 pool lifecycle context 的组合，兼顾调用链取消和服务关闭取消。
- [x] 新增测试确认 task 能感知提交方 context cancellation。
- [x] 按大型高并发系统标准调整为按用途命名的专用 worker pool provider，而不是在 `NewSessionStore` 内隐式创建。
- [x] 让 `NewSessionPurgePool` 显式依赖 named Redis client，保证 Fx stop order 中 purge pool 先于 Redis client 停止。
- [x] 新增 Fx 装配测试，确认 `SessionStore` 能消费 `auth_session_purge_pool` 命名专用池。

## Guardrails

- [x] 不新增 Prometheus、OpenTelemetry、metrics server 或 tracing exporter。
- [x] 不新增 Kafka、RabbitMQ、NATS、Redis Stream、eventbus、outbox、dispatcher 或通用 job framework。
- [x] 不修改 auth application port 方法签名、HTTP controller、request/response DTO 或 route graph。
- [x] 不修改 Redis key builder 的业务 key schema，除非仅补充日志字段。
- [x] 不修改 Ent schema、Ent generated code、Atlas migration 或 PostgreSQL adapter。
- [x] 不把 auth session purge 业务规则、Redis key 规则或 feature DTO 放入 `common/runtime/workerpool`。
- [x] 不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 common worker pool 单包测试：

```bash
cd common
go test ./runtime/workerpool
```

- [x] 运行 auth Redis adapter 单包测试：

```bash
cd user-service
go test ./internal/features/auth/infrastructure/redis
```

- [x] 运行变更范围测试：

```bash
make test-common
make test-user-service
```

- [x] 运行结构扫描，确认没有裸 goroutine 残留在目标方法：

```bash
rg -n "go func\\(\\)" user-service/internal/features/auth/infrastructure/redis/session_store.go
```

- [x] 运行依赖扫描，确认 ants 只进入 common workerpool 相关实现：

```bash
rg -n "panjf2000/ants|workerpool" common user-service AGENTS.md docs/ARCHITECTURE.md
```

- [x] 检查 git diff，确认只包含预期的 common workerpool、auth Redis adapter、测试、go.mod/go.sum 和文档变更。

## Review Notes

- [x] 确认 `DeleteAllUserSessions` 的同步失败仍同步返回，异步执行失败通过日志/stats 可观测。
- [x] 确认 task 提交失败不会被吞掉。
- [x] 确认 pool stop drain 不会在 Redis client 关闭之后才运行。
- [x] 确认 worker pool API 没有泄漏 auth/user-service 业务语义。
- [x] 确认 ants 使用版本为实现时可获得的最新稳定版本。
