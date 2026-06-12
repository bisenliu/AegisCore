# Add auth session purge workerpool

## What

用受控 worker pool 替换认证 Redis session 全量删除里的裸后台 goroutine。`DeleteAllUserSessions` 仍同步执行 Redis detach，把用户 session sorted-set 移动到临时 purge key 并设置 TTL；detach 成功后，将 detached session 清理提交到可观测、可限流、可优雅退出的后台任务池。

本变更引入一个跨服务无业务语义的公共 runtime primitive：

```text
common/runtime/workerpool/
  pool.go       # ants-backed worker pool facade
  errors.go     # closed / pool full / invalid task errors
  stats.go      # in-memory counters and snapshots
```

底层使用 `github.com/panjf2000/ants/v2`，版本使用当前 `go list -m -versions github.com/panjf2000/ants/v2` 查询到的最新稳定版本 `v2.12.1`。

Auth Redis adapter 消费该公共 worker pool API，提交 `auth.redis.purge_detached_user_sessions` 任务，任务失败时记录结构化 error 并增加失败计数；服务停止时先停止接收新任务，等待已接收任务完成或被 shutdown context 取消。

## Why

当前 [session_store.go](/Users/liubisen/Desktop/sander/Project/my/AegisCore/user-service/internal/features/auth/infrastructure/redis/session_store.go:278) 的 `DeleteAllUserSessions` 在 detach 成功后直接启动 goroutine 清理 detached sessions：

- goroutine 生命周期不受 Fx 管理，服务关闭时无法优雅等待。
- 清理失败被丢弃，无法通过日志或指标发现。
- 高并发登出或强制改密时 goroutine 数量没有统一上限。
- Redis 临时 purge key 虽有 TTL 兜底，但失败或积压不可观测，旧 session key 可能在 TTL 窗口内堆积。

受控 worker pool 可以把这类后台清理从“请求路径里顺手起 goroutine”收敛成 runtime 能力：限制并发、满载拒绝、记录失败、支持 shutdown drain，并且不把 auth 业务语义下沉到 `common`。

## Scope

包括：

- 在 `common/go.mod` 引入 `github.com/panjf2000/ants/v2 v2.12.1`。
- 新增 `common/runtime/workerpool`，封装 ants pool、pool full 背压、task error/panic 记录、内存统计和 Fx lifecycle 关闭。
- 在 auth feature module 中提供 `auth_session_purge_pool` 命名专用 worker pool，并注入 auth Redis session store，移除 `DeleteAllUserSessions` 内部裸 goroutine。
- 为 `DeleteAllUserSessions` purge 任务设置有限并发、满载拒绝和关闭超时。
- purge task 执行失败时记录 error、user_id、purge_key、session_prefix、cut_time 和 task name。
- task 提交失败时从 `DeleteAllUserSessions` 返回 error，让 application 现有错误日志链路感知“已 detach 但后台清理没有接手”的风险。
- 更新 `docs/ARCHITECTURE.md` 和 `AGENTS.md`，说明 `common/runtime/workerpool` 的职责和 ants 使用边界。
- 补充 common worker pool 单元测试和 auth Redis adapter 行为测试。

不包括：

- 不引入 Prometheus、OpenTelemetry 或新的 metrics server。
- 不新增 MQ、eventbus、outbox、dispatcher 或通用 job framework。
- 不改变 auth application port 方法签名或 HTTP API 响应契约。
- 不修改 Redis key schema、Lua detach 脚本语义、Ent schema、Atlas migration 或 token version 逻辑。
- 不把 auth session purge 业务规则放进 `common`。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `DeleteAllUserSessions` 不再直接调用 `go func()`。
- detached session purge 由受控 ants-backed worker pool 执行，并有明确并发上限和满载拒绝语义。
- worker pool 关闭时停止接收新任务，并在 Fx `OnStop` 中等待已接收任务完成或按 shutdown context 退出。
- auth session purge 使用按用途命名的专用池，不与其他 feature 或后台任务共享全局池。
- purge task 内部 error 和 panic 不被吞掉，必须记录结构化日志并反映在 stats 中。
- task 提交失败会从 `DeleteAllUserSessions` 返回包装后的 error。
- Redis client 的 Fx 关闭顺序不会早于 auth session purge worker pool drain。
- common worker pool 不依赖 user-service、auth feature、Gin、Ent、Redis client 或业务 DTO。
- auth Redis adapter 测试覆盖后台 purge 成功、批量 purge、新 session 不被误删、提交失败返回 error、执行失败可观测。
- `make test-common` 和 `make test-user-service` 通过，或明确说明因外部依赖导致未能运行。
