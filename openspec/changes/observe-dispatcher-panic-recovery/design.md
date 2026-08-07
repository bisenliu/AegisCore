## Context

RBAC outbox dispatcher 是 permission feature 内的后台主动资源。`run` 循环通过 defer recover 捕获 panic，并将 dispatcher 标记为 `unexpected_exit` 后停止运行。现有实现能保持 fail-closed，但日志只包含错误分类，不包含 recovered value 或 stack trace，导致 panic 根因定位需要依赖外部崩溃信息。

## Goals / Non-Goals

**Goals:**

- panic recovery 日志必须包含 `error_category=unexpected_exit`。
- panic recovery 日志必须包含 `recovered` 字段，记录 recover 捕获的值。
- panic recovery 日志必须包含 stack trace 字段。
- panic 后继续停止 dispatcher，设置 `running=false`，关闭当前 `done`，停止 ticker，并记录 `DispatcherRunningObserved(false)`。
- 单元测试覆盖状态、Stop 幂等和日志排障字段。

**Non-Goals:**

- 不引入 panic 后自动重启。
- 不改变 outbox event retry、claim lease、Ack/Fail 或投递状态机。
- 不改变 `Start(ctx)`、`Stop(ctx)` 或 `DispatchOnce(ctx)` 的结构化接口语义。
- 不改变 Redis watcher、Casbin projection 或 user-role cache。

## Decisions

1. 使用现有 `logger.Error` 与 `logger.StackTrace` 记录 recovery 日志。

   理由：permission application 已使用 `logger.StackTrace` 统一追加 `stacktrace` 字段，复用该约定可避免新增日志字段规范。`logger.Error` 还会复用 dispatcher 运行 context 中的 logger-aware values。

2. 使用 `zap.Any("recovered", recovered)` 记录 recover value。

   理由：panic value 可以是 string、error 或任意类型，`zap.Any` 能保留测试和线上排障需要的实际值。

3. 不改变 defer 清理顺序。

   理由：现有 defer 已在 panic 后停止 ticker、清理当前 run 句柄、标记 `running=false`、上报 running=false 并关闭 done。该 change 只补充日志字段，避免改变生命周期行为。

## Risks / Trade-offs

- [Risk] 日志字段名称不一致会削弱告警和查询稳定性。→ Mitigation：沿用 `error_category` 与 `stacktrace`，新增明确的 `recovered` 字段，并用 observer 测试断言。
- [Risk] recovery 日志改造时误改 panic 后清理流程。→ Mitigation：测试继续断言 `Running=false`、`LastErrorCategory=unexpected_exit`、`Stop(ctx)` 幂等和 running=false metric。

## Migration Plan

该 change 只修改内部日志和单元测试，不涉及数据库 migration、OpenAPI、部署资产或运行配置。回滚方式是恢复 dispatcher recovery 日志字段变更与对应测试。

验证方式：运行 `go test ./user-service/internal/features/permission/application -run 'TestDispatcherUnexpectedExit|TestDispatcherStartStop'`。

## Open Questions

无。
