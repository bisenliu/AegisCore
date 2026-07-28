## Context

`common/runtime/datastore` 当前拥有业务中立的 PostgreSQL/Redis 单资源构造、探测、关闭和 Fx adapter 能力。user-service 的部分 infrastructure 代码在需要事务时直接调用 Ent client 或 `database/sql` 的事务入口，并在 feature-local 代码中手写 rollback helper。

这些直接事务边界存在两个问题：事务 begin、commit、rollback 容易与 HTTP request context 生命周期耦合；不同 feature 对 rollback 错误、commit 前 request cancellation 和 defer 兜底的处理不一致。本变更以不兼容方式统一事务生命周期 primitive，避免继续扩散旧写法。

受影响路径：

- `common/runtime/datastore/`：新增通用事务 helper 和测试。
- `user-service/internal/features/role/infrastructure/`：迁移角色权限和用户角色事务边界。
- `user-service/internal/features/permission/infrastructure/` 或实际拥有 `bootstrap_store.go` 的 infrastructure 包：迁移 RBAC bootstrap 事务边界。
- `openspec/changes/add-datastore-transaction-helper/`：记录 proposal、design、spec delta 和 tasks。

不影响 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、部署清单、Prometheus/Grafana 资产或 RBAC policy 同步协议。

## Goals / Non-Goals

**Goals:**

- 在 `common/runtime/datastore` 提供只依赖标准库的泛型事务生命周期 helper。
- 让事务 begin 与 request cancellation 解耦，同时保留 context values 和 deadline。
- 保持事务内业务 SQL/Ent 操作继续使用原始 request context。
- commit 前显式检查原始 request context，避免客户端取消后继续提交。
- 统一失败分支 rollback、defer 兜底和 rollback error 保留语义。
- 迁移当前 user-service 中已识别的事务边界，删除 feature-local rollback helper。
- 通过测试和 architecture lint 验证 common 与 user-service 边界。

**Non-Goals:**

- 不新增兼容旧写法的 `LegacyTx`、`SafeRollback`、`Rollback(ctx)` 或双轨 API。
- 不修改 Ent 生成代码，不在 `common` 中导入 `github.com/aegiscore/user-service/ent`。
- 不改变业务 use case、HTTP API、RBAC 授权语义、数据库 schema 或 migration。
- 不新增通用 repository、unit-of-work、outbox、eventbus 或跨 feature transaction wrapper。

## Decisions

### Decision: common 只定义最小泛型事务接口

`common/runtime/datastore` 定义：

- `Transaction`：只包含 `Commit() error` 和 `Rollback() error`。
- `TransactionStarter[T Transaction]`：只包含 `BeginTransaction(context.Context) (T, error)`。
- `BeginTransaction[T Transaction](ctx, starter)`：返回 concrete transaction 和 `*Finish[T]`。
- `Finish[T]`：负责 `Commit(ctx)`、`Fail(err)` 和 `RollbackUnlessCommitted()`。

理由：该设计保持 common 业务中立，不绑定 Ent、SQL driver 或 user-service 类型；调用方用极薄 adapter 适配自己的事务入口。

备选方案：在 common 中提供 `BeginEntTx`。该方案调用方最少，但会让 common 依赖 user-service Ent 生成类型，违反 common 边界，因此拒绝。

### Decision: begin 使用 detached lifecycle context，业务操作使用原始 context

helper 在 begin 时基于 `context.WithoutCancel(ctx)` 创建 lifecycle context：保留 values，不继承 request cancellation；原始 context 有 deadline 时继承 deadline；无 deadline 时使用 `DefaultTransactionCleanupTimeout = 5 * time.Second`。

事务内 SQL/Ent 操作仍由调用方传入原始 `ctx`，不改业务查询取消语义。

备选方案：全部事务操作使用 detached context。该方案能最大化清理成功率，但会让客户端取消后业务查询继续执行，改变 request cancellation 语义，因此拒绝。

### Decision: commit 前检查原始 context

`Finish.Commit(ctx)` 先检查 `ctx.Err()`；若原始 request 已取消，直接返回 context error，不调用 `tx.Commit()`。未提交的事务由 `defer finish.RollbackUnlessCommitted()` 兜底 rollback。

理由：request 已取消时继续 commit 容易产生调用方不可见的副作用；显式拒绝提交能维持业务边界的可预测性。

备选方案：commit 也使用 detached context 并忽略 request cancellation。该方案可能减少提交失败，但违反取消后不继续提交的核心决策，因此拒绝。

### Decision: rollback error 不吞掉原始错误

`Finish.Fail(err)` 调用 rollback；如果 rollback 也失败，返回 `errors.Join(err, rollbackErr)`。`RollbackUnlessCommitted()` 返回 rollback 错误，但通常由 defer 兜底调用，显式错误分支必须使用 `Fail(err)` 保留错误。

理由：业务失败和清理失败都对排障有价值，旧 helper 常吞掉 rollback error 或覆盖主错误。

备选方案：记录 rollback error 后返回原始错误。该方案不需要调用方处理 joined error，但会丢失失败清理信号，因此拒绝。

### Decision: user-service 在 infrastructure 局部适配 Ent

Ent adapter 放在对应 infrastructure 包内，例如：

```go
type entTxStarter struct {
    client *ent.Client
}

func (s entTxStarter) BeginTransaction(ctx context.Context) (*ent.Tx, error) {
    return s.client.Tx(ctx)
}
```

理由：Ent 是服务生成类型，属于 user-service infrastructure 依赖；common 只暴露事务 primitive，不感知具体 ORM。

备选方案：在 user-service shared 或全局 infrastructure 包放统一 Ent adapter。该方案可能减少重复类型，但当前适配器极薄，放在消费侧更符合 feature-first 和最小接口原则。

## Risks / Trade-offs

- [Risk] 不兼容迁移可能遗漏直接事务调用。→ Mitigation：用代码搜索覆盖 `client.Tx(ctx)`、`BeginTx(ctx`、`.Commit()`、`.Rollback()` 和 `rollback(tx`，并在相关 infrastructure 包补充测试。
- [Risk] `RollbackUnlessCommitted()` 在 defer 中的错误可能被忽略。→ Mitigation：约定业务错误分支使用 `finish.Fail(err)`；defer 只作为兜底，测试覆盖 rollback failure 合并路径。
- [Risk] 无 deadline request 使用 5 秒 cleanup timeout，极端数据库阻塞时 rollback 仍可能超时或卡顿。→ Mitigation：timeout 有界且集中定义，后续可基于运行数据调整常量，不为首版引入配置复杂度。
- [Risk] generic helper 暴露的 `Finish` 状态如果被错误复用，可能重复 commit/rollback。→ Mitigation：首版保持最小实现和测试覆盖；调用方按单事务局部变量使用，不作为长期对象保存。
- [Risk] 仅依赖规范约束无法禁止未来直接事务调用。→ Mitigation：本次迁移现有代码，并在 tasks 中要求 architecture lint 与 grep 检查；后续可在架构 lint 中加入静态规则。

## Migration Plan

1. 在 `common/runtime/datastore/transaction.go` 新增 helper，添加 `transaction_test.go` 覆盖 lifecycle context、commit cancellation、rollback 和 joined error。
2. 在 user-service 相关 infrastructure 包新增局部 Ent starter adapter，迁移 `role_permission_store.go`、`user_role_store.go`、`bootstrap_store.go` 的事务边界。
3. 删除旧 feature-local `rollback(tx, err)` helper，并替换所有 `tx.Commit()`、`tx.Rollback()` 和 `return rollback(tx, err)` 分支。
4. 运行相关 package 测试、`make user-service-architecture-lint`，最后运行 `make test` 或 `make verify`。

回滚策略：本变更不改变数据库 schema 或外部 API；若实现出现问题，可在代码层 revert 本 change 的 helper 与迁移提交。由于方案明确不保留兼容双轨，回滚时必须整体恢复旧事务写法，不能部分混用。

## Open Questions

- 无。当前需求已明确不保留兼容 API、不修改 Ent 生成代码，并要求 common 只提供标准库泛型 primitive。
