## 1. Datastore 事务语义

- [x] 1.1 修改 `common/runtime/datastore/transaction.go`，使无原始 deadline 时的 lifecycle context 只使用 `context.WithCancel(context.WithoutCancel(ctx))`，不再注入 `DefaultTransactionCleanupTimeout` 或任何固定 deadline。
- [x] 1.2 更新 `DefaultTransactionCleanupTimeout` 相关注释，移除其作为 `BeginTx` lifecycle timeout 的含义；若实现后不再需要该常量，则删除常量和相关引用。
- [x] 1.3 保持 `Finish.Commit(ctx)` 提交前检查原始 context 的行为，确认原始 context 取消时不会调用底层 `Commit`。
- [x] 1.4 确认 `BeginTransaction` 失败、`Commit` 成功、`Fail` 和 `RollbackUnlessCommitted` 后均会取消 helper 创建的 lifecycle context，避免 context 泄漏。

## 2. Datastore 测试

- [x] 2.1 更新 `common/runtime/datastore/transaction_test.go`，将无原始 deadline 的期望改为 starter context 没有 datastore 注入 deadline。
- [x] 2.2 保留并校验原始 context value 继承、原始取消信号隔离和显式 deadline 继承场景。
- [x] 2.3 新增无原始 deadline 的事务在超过旧 `DefaultTransactionCleanupTimeout` 后仍可提交的回归测试。
- [x] 2.4 保留并强化提交前原始 context 取消时 `Finish.Commit(ctx)` 拒绝提交且后续 rollback 生效的测试。

## 3. RBAC/PostgreSQL 回归覆盖

- [x] 3.1 检查 `user-service/internal/features/role/infrastructure/postgres` 和 `user-service/internal/features/permission/infrastructure/postgres` 现有事务测试，定位可覆盖 RBAC policy 写入、system binding、bootstrap 或 outbox claim 的最小测试入口。
- [x] 3.2 增加 deterministic 锁等待或等价慢事务测试，验证 RBAC policy revision counter 或相关事务在等待超过旧 5 秒断点后不因 datastore helper 自动取消。
- [x] 3.3 增加或更新提交前原始 request context 取消的 RBAC 写事务测试，验证角色、绑定、revision counter、policy revision 和 outbox event 均不提交，且不触发 policy change 通知或 reload。
- [x] 3.4 确认 RBAC PostgreSQL store 没有新增兼容开关、绕过参数、本地事务 helper 或旧 5 秒行为分支。

## 4. OpenSpec 与文档同步

- [x] 4.1 根据最终实现核对本 change 的 `proposal.md`、`design.md` 和 `specs/**/*.md`，确保描述与代码一致。
- [x] 4.2 如实现改变长期稳定行为描述，更新对应主规格或保持本 change delta 可归档为主规格。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认 OpenSpec、架构边界和文档规则通过。

## 5. 验证与交付

- [x] 5.1 运行 `go test ./common/runtime/datastore`，确认共享事务 helper 测试通过。
- [x] 5.2 运行相关 user-service PostgreSQL/RBAC package 测试，至少覆盖 `user-service/internal/features/role/infrastructure/postgres` 和新增/更新测试所在 package。
- [x] 5.3 检查本次变更不包含 Ent 生成物、OpenAPI 生成物、Atlas migration 或部署清单 drift；如触发生成物变化，运行对应生成命令并检查 `git diff`。
- [x] 5.4 将本次预期代码、测试、OpenSpec 和文档变更加到暂存区。
- [x] 5.5 运行 `make lint`，失败时修复后重跑并保持暂存区包含预期变更。
- [x] 5.6 运行 `make verify`，失败时修复后重跑并保持最终验证通过。
