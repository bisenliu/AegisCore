## ADDED Requirements

### Requirement: 通用事务生命周期 helper

`common/runtime/datastore` MUST 提供业务中立的泛型事务生命周期 helper，用于 infrastructure 代码创建、提交和回滚显式事务边界。helper MUST 只依赖标准库和最小事务接口，MUST NOT 导入 user-service、Ent 生成代码、feature 包或服务私有类型。

#### Scenario: 使用 detached lifecycle context 创建事务

- **WHEN** infrastructure 代码通过共享 helper 使用 request context 开始事务
- **THEN** helper MUST 使用保留原始 context values 的事务 lifecycle context 调用事务 starter
- **AND** 事务 lifecycle context MUST NOT 继承 request cancellation
- **AND** 原始 context 存在 deadline 时，事务 lifecycle context MUST 继承该 deadline
- **AND** 原始 context 不存在 deadline 时，事务 lifecycle context MUST 使用有界 cleanup timeout

#### Scenario: 事务内业务操作使用原始 request context

- **WHEN** application 或 infrastructure 代码在事务内执行 SQL 或 Ent 业务操作
- **THEN** 这些业务操作 MUST 继续使用原始 request context
- **AND** request cancellation MUST 仍能中断事务内业务查询

#### Scenario: request 取消后拒绝提交

- **WHEN** 原始 request context 在 commit 前已经取消
- **THEN** 事务完成器 MUST 拒绝提交并返回原始 context error
- **AND** 调用方 MUST 通过事务完成器的 rollback 兜底路径回滚事务

#### Scenario: 失败分支保留 rollback 错误

- **WHEN** 事务内业务操作失败且 rollback 也失败
- **THEN** 事务完成器 MUST 返回同时保留原始业务错误和 rollback 错误的 error
- **AND** rollback 错误 MUST NOT 被吞掉或覆盖原始业务错误

### Requirement: 禁止直接事务边界

新增或改造后的 infrastructure 事务边界 MUST 通过 `common/runtime/datastore` 的共享事务 helper 创建、提交和回滚。业务 infrastructure MUST NOT 直接调用 `client.Tx(ctx)`、`db.BeginTx(ctx, ...)`、`tx.Commit()`、`tx.Rollback()` 或手写 `rollback(tx, err)` helper 管理可由共享 helper 表达的事务生命周期。

#### Scenario: Ent 事务边界适配共享 helper

- **WHEN** user-service infrastructure 需要创建 Ent 事务
- **THEN** infrastructure 包 MUST 在消费侧适配 Ent client 到共享 transaction starter 接口
- **AND** Ent 适配器 MUST 留在 user-service infrastructure 边界内
- **AND** `common/runtime/datastore` MUST NOT 依赖 user-service Ent 生成类型

#### Scenario: 使用事务完成器终结事务

- **WHEN** infrastructure 代码已经通过共享 helper 开始事务
- **THEN** 成功路径 MUST 使用事务完成器执行 commit
- **AND** 错误路径 MUST 使用事务完成器执行 rollback 并保留原始错误
- **AND** defer 兜底路径 MUST 使用事务完成器在未提交时回滚事务
- **AND** 调用方 MUST NOT 绕过事务完成器直接提交或回滚事务
