## Why

当前 user-service infrastructure 中的事务边界直接使用 `client.Tx(ctx)`、`db.BeginTx(ctx, ...)` 和 feature-local rollback helper，事务 begin、commit、rollback 都容易绑定到 HTTP request context。request context 取消后，rollback 或 commit 清理路径可能失效，导致事务生命周期语义不一致。

本变更需要统一跨服务共享的事务 primitive，并以不兼容迁移方式移除直接管理事务终结的旧写法，降低后续 feature 新增事务边界时的行为差异。

## What Changes

- 在 `common/runtime/datastore` 新增泛型事务生命周期 helper，提供最小 `Transaction` 和 `TransactionStarter[T]` 接口以及 `BeginTransaction`/`Finish` 事务完成器。
- helper 使用 detached lifecycle context 创建事务：保留 context values，不继承 request cancellation，继承原始 deadline；无 deadline 时使用 5 秒 cleanup timeout。
- 事务内 SQL/Ent 业务操作继续使用原始 request context，保持业务查询可被客户端取消。
- `Finish.Commit(ctx)` 在提交前检查原始 context，若 request 已取消则拒绝提交。
- `Finish.Fail(err)` 和 `RollbackUnlessCommitted()` 统一 rollback 语义，rollback 失败时使用 `errors.Join` 保留原始错误和 rollback 错误。
- **BREAKING**：新增和改造后的 infrastructure 事务边界不再允许直接调用 `client.Tx(ctx)`、`db.BeginTx(ctx, ...)`、`tx.Commit()`、`tx.Rollback()` 或手写 `rollback(tx, err)` helper 管理事务生命周期。
- **BREAKING**：迁移 user-service 当前 Ent/database/sql 事务边界，删除 feature-local rollback helper，不保留 `LegacyTx`、`SafeRollback`、`Rollback(ctx)` 等兼容双轨 API。
- 不修改 Ent 生成代码；Ent 事务适配器放在 user-service 对应 infrastructure 包内，`common/runtime/datastore` 不导入 `github.com/aegiscore/user-service/ent`。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：扩展 `common/runtime/datastore` 的共享 runtime primitive 规格，新增通用事务生命周期 helper 及禁止直接事务边界的约束。

## Impact

- 影响 `common/runtime/datastore`：新增事务 helper 和单元测试。
- 影响 user-service infrastructure：迁移 `role_permission_store.go`、`user_role_store.go`、`bootstrap_store.go` 等现有事务边界，补充或调整相关测试。
- 不影响 HTTP API、OpenAPI schema、数据库 schema、Atlas migration、部署资产或 Ent 生成代码。
- 对新增/改造事务代码形成不兼容约束：事务 begin、commit、rollback 必须通过共享 helper 暴露的 `Finish` 完成器管理。
