## 1. 共享事务 helper

- [x] 1.1 在 `common/runtime/datastore/transaction.go` 新增 `DefaultTransactionCleanupTimeout`、`Transaction`、`TransactionStarter[T]`、`Finish[T]` 和 `BeginTransaction[T]`，只依赖 Go 标准库。
- [x] 1.2 实现 detached lifecycle context：保留原始 context values，不继承 request cancellation，继承原始 deadline；无 deadline 时使用 5 秒 cleanup timeout。
- [x] 1.3 实现 `Finish.Commit(ctx)`、`Finish.Fail(err)` 和 `Finish.RollbackUnlessCommitted()`，确保 commit 前检查原始 context，rollback error 通过 `errors.Join` 保留。
- [x] 1.4 添加 `common/runtime/datastore/transaction_test.go`，覆盖 begin 使用 detached context、request context canceled 后 rollback 仍被调用、commit 前 request canceled 拒绝提交、rollback error 与原始 error 合并。

## 2. user-service 事务迁移

- [x] 2.1 在 `user-service/internal/features/role/infrastructure/postgres` 添加 Ent transaction starter 适配器，适配 `*ent.Client` 到 `datastore.TransactionStarter[*ent.Tx]`。
- [x] 2.2 迁移 `user-service/internal/features/role/infrastructure/postgres/role_permission_store.go` 中全部 `s.client.Tx(ctx)` 事务边界到 `datastore.BeginTransaction(ctx, entTxStarter{...})`。
- [x] 2.3 迁移 `user-service/internal/features/role/infrastructure/postgres/user_role_store.go` 中的 Ent 事务边界到共享 helper。
- [x] 2.4 迁移 `user-service/internal/features/role/infrastructure/postgres/bootstrap_store.go` 中的 `db.BeginTx(ctx, nil)` 事务边界到共享 helper，并在该包内提供 `database/sql` transaction starter 适配器。
- [x] 2.5 删除 `user-service/internal/features/role/infrastructure/postgres/tx.go` 中旧的 feature-local `rollback(tx, err)` helper，并替换所有 `return rollback(tx, err)`、直接 `tx.Commit()` 和直接 `tx.Rollback()` 分支。

## 3. 测试与静态检查

- [x] 3.1 添加或调整 role infrastructure 测试，覆盖迁移后的角色权限、用户角色和 bootstrap 事务成功与错误路径。
- [x] 3.2 使用代码搜索确认 user-service infrastructure 中不再存在可由 helper 表达的 `client.Tx(ctx)`、`db.BeginTx(ctx, ...)`、手写 `rollback(tx, err)`、直接 `tx.Commit()` 或直接 `tx.Rollback()` 事务终结写法。
- [x] 3.3 运行 `go test ./common/runtime/datastore`。
- [x] 3.4 运行 user-service role infrastructure 相关 package 测试。
- [x] 3.5 运行 `make user-service-architecture-lint`。

## 4. 收尾验证

- [x] 4.1 确认本变更不修改 Ent 生成代码、OpenAPI 生成物、Atlas migration、部署资产或观测资产；如生成物出现 drift，说明原因并恢复非预期变更。
- [x] 4.2 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [x] 4.3 运行 `make lint`。
- [x] 4.4 运行 `make verify`。
- [x] 4.5 所有验证通过后，将已完成任务 checkbox 更新为 `- [x]`。
