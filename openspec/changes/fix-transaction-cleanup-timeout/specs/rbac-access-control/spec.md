## ADDED Requirements

### Requirement: RBAC policy 写事务不得受共享 helper 隐式 5 秒断点限制

RBAC 角色、角色权限、用户角色、系统绑定、超级管理员 bootstrap 和 policy outbox claim 的 PostgreSQL 事务 MUST 依赖 `common/runtime/datastore` 修正后的事务 lifecycle 语义。无原始 deadline 时，这些事务 MUST NOT 因 datastore helper 的固定 5 秒 timeout 被自动回滚；事务耗时、锁等待和慢查询边界 MUST 由调用方显式 deadline 或数据库 timeout 策略控制。

#### Scenario: policy revision counter 锁等待超过 5 秒仍按策略完成
- **WHEN** RBAC policy 写事务在分配单调 revision 时等待固定 `rbac_policy_revision_counters` 行锁超过 `DefaultTransactionCleanupTimeout`
- **THEN** datastore helper MUST NOT 因隐藏 5 秒 deadline 取消该事务
- **AND** 锁释放后事务 MUST 能继续追加 policy revision 和 outbox event，并在原始 context 和数据库策略允许时提交成功

#### Scenario: 提交前请求取消仍不得提交 RBAC 写结果
- **WHEN** RBAC policy 写事务已完成业务 mutation、revision 分配和 outbox event 构造，但原始 request context 在 commit 前取消或超时
- **THEN** store MUST 拒绝提交该事务
- **AND** 系统 MUST 回滚角色、角色权限、用户角色、policy revision counter、policy revision 和 outbox event 变更
- **AND** application MUST NOT 发送 policy change 通知或触发本实例 reload

#### Scenario: RBAC 高并发写入不存在 helper 层固定 5 秒失败点
- **WHEN** 多个 RBAC 写请求并发竞争 revision counter、角色行锁或系统绑定事务资源
- **THEN** 请求成功、失败或超时 MUST 由原始 context deadline、数据库错误或显式数据库 timeout 决定
- **AND** 系统 MUST NOT 因 `DefaultTransactionCleanupTimeout` 被传入 `BeginTx` 而在约 5 秒处形成固定自动回滚断点

#### Scenario: RBAC 本地代码不保留兼容绕过分支
- **WHEN** RBAC PostgreSQL store 使用事务 helper 执行在线写入、seed/system binding、bootstrap 或 outbox claim
- **THEN** store MUST 直接消费修正后的 `datastore.BeginTransaction` 行为
- **AND** store MUST NOT 增加旧 5 秒行为兼容开关、绕过参数或重复事务 lifecycle helper
