## MODIFIED Requirements

### Requirement: RBAC policy outbox 可靠投递

系统 MUST 以 PostgreSQL 中已提交的 RBAC policy outbox event 作为跨副本 revision 通知的可靠恢复事实，并由显式 dispatcher 对到期 event 执行 claim、Redis publish、成功 ack 和失败退避。user-service MUST 私有拥有轮询、批量、claim lease 与退避配置，并通过 permission lifecycle 启停同一 dispatcher 实例。dispatcher MUST 提供至少一次投递并在进程崩溃或 Redis 故障后自动恢复；Redis MUST 只作为可重放加速层。dispatcher 后台 goroutine 发生 panic 时 MUST fail-closed 停止运行，并记录包含 recovered value、稳定错误分类和 stack trace 的结构化日志。

#### Scenario: dispatcher 后台 panic recovery 可观测性

- **WHEN** dispatcher 后台 `run` 循环发生 panic
- **THEN** recovery 日志 MUST 记录 `error_category=unexpected_exit`
- **AND** recovery 日志 MUST 记录 `recovered` 字段，值来自 `recover()` 捕获结果
- **AND** recovery 日志 MUST 记录 stack trace 字段
- **AND** dispatcher MUST 将 `LastErrorCategory` 更新为 `unexpected_exit`
- **AND** dispatcher MUST 停止当前 ticker、标记 `Running=false`、上报 `DispatcherRunningObserved(false)` 并关闭当前 `done`
- **AND** 后续调用 `Stop(ctx)` MUST 幂等稳定返回
- **AND** dispatcher MUST NOT 因本场景自动重启后台 loop
