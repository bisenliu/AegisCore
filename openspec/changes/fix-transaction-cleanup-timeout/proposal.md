## Why

`common/runtime/datastore` 当前在原始 context 无 deadline 时把 5 秒“事务清理超时”传给 `Ent Tx` 和 `database/sql.BeginTx`，而 `BeginTx` context 会覆盖事务从 begin 到 commit/rollback 的完整生命周期。这会把清理预算错误地变成所有无 deadline 事务的隐藏 5 秒硬上限，在 RBAC 写入、policy revision counter 行锁竞争、数据库慢查询或高并发排队时触发自动回滚和间歇 500，阻断上线可用性。

## What Changes

- **BREAKING**：移除无原始 deadline 时 `datastore.BeginTransaction` 为事务 lifecycle context 自动附加 5 秒 timeout 的行为。
- `datastore.BeginTransaction` 仍使用 detached lifecycle context 保留调用方 value，并继承调用方显式 deadline；无 deadline 时只创建可由 helper 终结的 cancelable context。
- `Finish.Commit(ctx)` 在提交前继续检查原始 request context；原始 context 已取消或超时时必须拒绝提交，并由既有 fail/defer 路径回滚。
- rollback 防泄漏不再通过 `BeginTx` context 的固定 timeout 表达；事务耗时策略由调用方 deadline 或数据库 `statement_timeout`、`lock_timeout`、`idle_in_transaction_session_timeout` 等显式配置承担。
- 更新 datastore 单元测试和 RBAC/PostgreSQL 相关回归测试，验证无原始 deadline 的事务超过 5 秒仍可提交，提交前取消仍拒绝提交，高并发锁竞争不再存在 helper 层隐式 5 秒断点。
- 不保留兼容开关、兼容分支或旧默认行为。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `shared-platform-primitives`：调整 `common/runtime/datastore` 共享事务 helper 的 lifecycle context、deadline 和提交取消语义。
- `rbac-access-control`：明确 RBAC policy 写入和 revision 分配事务不得受 datastore helper 隐式 5 秒事务生命周期限制。

## Impact

- 影响代码：`common/runtime/datastore/transaction.go`、`common/runtime/datastore/transaction_test.go`、`user-service/internal/features/role/infrastructure/postgres/tx.go` 相关事务调用测试、`user-service/internal/features/role/infrastructure/postgres/bootstrap_store.go`、`user-service/internal/features/permission/infrastructure/postgres/outbox_store.go`。
- 影响行为：无原始 deadline 的共享事务不再被 helper 默认 5 秒自动回滚；显式调用方 deadline 和提交前取消检查仍保持生效。
- 影响 RBAC：角色、角色权限、用户角色、bootstrap super admin、RBAC seed/system binding 和 outbox claim 的事务可在数据库策略允许范围内等待锁或慢查询完成，不再受隐藏 5 秒断点影响。
- 不影响公开 HTTP API、OpenAPI、数据库 schema、Atlas migration 或部署 manifest。
- 运维依赖：长事务、慢语句和锁等待的保护需要由调用方 context 或 PostgreSQL timeout 配置显式表达，而不是依赖 datastore helper 的隐藏默认值。
