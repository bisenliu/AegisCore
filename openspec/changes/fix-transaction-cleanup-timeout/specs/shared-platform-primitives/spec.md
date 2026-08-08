## ADDED Requirements

### Requirement: 共享事务 helper 不得注入隐藏事务生命周期上限

`common/runtime/datastore` 的共享事务 helper MUST 使用调用方拥有的 context 策略表达事务生命周期。`BeginTransaction` MUST 保留原始 context value，并 MUST 隔离原始取消信号以允许请求取消后执行 rollback cleanup；当原始 context 提供 deadline 时，事务 lifecycle context MUST 继承该 deadline；当原始 context 没有 deadline 时，helper MUST NOT 为传入 `Ent Tx`、`database/sql.BeginTx` 或其他 transaction starter 的 context 注入固定 timeout、cleanup budget 或隐藏 deadline。

#### Scenario: 无原始 deadline 的事务不会获得默认 5 秒 deadline
- **WHEN** 调用方以没有 deadline 的 context 调用 `datastore.BeginTransaction`
- **THEN** transaction starter 接收的 lifecycle context MUST 保留原始 context value 且不继承原始取消信号
- **AND** lifecycle context MUST NOT 包含由 datastore helper 注入的 5 秒 deadline 或其他固定 deadline

#### Scenario: 显式原始 deadline 继续约束事务生命周期
- **WHEN** 调用方以带 deadline 的 context 调用 `datastore.BeginTransaction`
- **THEN** transaction starter 接收的 lifecycle context MUST 继承同一个 deadline
- **AND** helper MUST NOT 放宽、覆盖或替换调用方显式 deadline

#### Scenario: 超过清理预算的无 deadline 事务仍可提交
- **WHEN** 原始 context 没有 deadline，且事务从 begin 到 commit 的耗时超过 `DefaultTransactionCleanupTimeout`
- **THEN** datastore helper MUST NOT 因该常量到期取消事务 lifecycle context
- **AND** 事务 MUST 能在 transaction implementation 和数据库策略允许时成功提交

#### Scenario: 提交前原始 context 取消时拒绝提交
- **WHEN** 事务已经开始，且原始 request context 在提交前被取消或超时
- **THEN** `Finish.Commit(ctx)` MUST 返回原始 context 错误并拒绝调用底层 commit
- **AND** 调用方 MUST 能通过 `Fail` 或 `RollbackUnlessCommitted` 回滚未提交事务

#### Scenario: rollback cleanup 不依赖 BeginTx 固定 timeout
- **WHEN** 事务需要在业务错误、提交拒绝或 defer 兜底中回滚
- **THEN** helper MUST NOT 通过传给 `BeginTx` 的固定 timeout 表达 rollback cleanup budget
- **AND** 长事务和泄漏保护 MUST 由调用方 context deadline、数据库 timeout 或拥有者显式生命周期策略表达
