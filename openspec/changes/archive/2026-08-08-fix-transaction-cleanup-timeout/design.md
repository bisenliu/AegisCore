## Context

`common/runtime/datastore.BeginTransaction` 当前用 `context.WithoutCancel(ctx)` 创建 detached context，并在原始 context 没有 deadline 时额外套上 `DefaultTransactionCleanupTimeout`。该 context 被传入 `Ent Client.Tx(ctx)` 和 `sql.DB.BeginTx(ctx, nil)`，而 `database/sql` 明确规定 `BeginTx` context 会一直作用到事务 commit 或 rollback；context 取消时标准库会回滚事务，`Commit` 会返回错误。

这个实现与注释中的“事务清理超时”不一致：5 秒预算覆盖了完整业务事务生命周期，而不是只覆盖 rollback cleanup。RBAC 写路径会在事务内串行推进 `rbac_policy_revision_counters` 固定行，并追加 policy revision 与 outbox event；锁等待、慢 SQL 或高并发排队都可能让事务生命周期超过 5 秒，进而被 helper 隐式取消。

受影响路径包括 `common/runtime/datastore/transaction.go`、`common/runtime/datastore/transaction_test.go`、`user-service/internal/features/role/infrastructure/postgres/tx.go`、`user-service/internal/features/role/infrastructure/postgres/bootstrap_store.go` 和 `user-service/internal/features/permission/infrastructure/postgres/outbox_store.go`。本变更不改变 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 移除无原始 deadline 时共享事务 helper 的隐藏 5 秒事务生命周期上限。
- 保留 detached lifecycle context 的 value 继承和 request cancel 隔离，使请求取消后仍可执行 rollback cleanup。
- 保留调用方显式 deadline：原始 context 有 deadline 时，事务 lifecycle context 必须继承该 deadline。
- 保留提交前原始 context 检查：原始 request context 已取消或超时时不得提交业务结果。
- 为 datastore 和 RBAC/PostgreSQL 事务路径补充回归验证，覆盖超过 5 秒事务、提交前取消和锁竞争行为。
- 不保留兼容开关、旧默认值兼容分支或双路径实现。

**Non-Goals:**

- 不新增 HTTP API、配置字段、环境变量、OpenAPI 注解或部署资源。
- 不修改 Ent schema、Atlas migration 或数据库表结构。
- 不在 `common` 中内置 user-service 私有 RBAC 策略、锁等待策略或 PostgreSQL 参数。
- 不为测试便利新增生产接口、mock-only adapter 或业务无关分支。
- 不用 helper 内固定超时替代数据库 `statement_timeout`、`lock_timeout` 或 `idle_in_transaction_session_timeout`。

## Decisions

### Decision: 无 deadline 时 lifecycle context 只可取消，不设置固定 timeout

`transactionLifecycleContext` 在原始 context 无 deadline 时改为基于 `context.WithoutCancel(ctx)` 创建 `context.WithCancel`。这样事务仍能在 `Finish.Commit`、`Finish.Fail` 或 `RollbackUnlessCommitted` 后释放 lifecycle context，但不会把 cleanup budget 传入 `BeginTx`。

理由：`BeginTx` context 是事务生命周期 context，不是 cleanup context。把 5 秒 timeout 放在这里会导致数据库自动回滚未完成事务。直接移除该 timeout 是最小且正确的行为修正。

备选方案：将默认 timeout 扩大到 30 秒或可配置。该方案被拒绝，因为它仍然保留隐藏事务上限，只是推迟触发，并继续混淆业务 deadline 与 cleanup budget。

### Decision: 显式 deadline 继续由调用方拥有

原始 context 有 deadline 时，detached lifecycle context 继续继承该 deadline。调用方显式设置的业务超时、命令超时或启动超时仍可约束事务生命周期。

理由：调用方 deadline 是外部可见策略，语义明确；本变更只移除 helper 私自引入的默认 deadline。

备选方案：无论原始 context 是否有 deadline 都移除事务 deadline。该方案被拒绝，因为会破坏调用方显式超时控制。

### Decision: `Finish.Commit(ctx)` 保持提交前检查原始 context

提交前继续执行 `ctx.Err()` 检查。如果 request context 已取消或超时，`Commit` 直接返回该错误，并由调用方既有 defer/fail 路径回滚事务。

理由：detached lifecycle context 是为了允许 rollback cleanup，不是为了允许请求取消后继续提交业务结果。

备选方案：提交时只依赖事务 lifecycle context。该方案被拒绝，因为 request cancel 后可能提交调用方已经放弃的业务结果。

### Decision: rollback 防泄漏不通过 `BeginTx` context 实现

本变更不引入新的 rollback context adapter。对于 `database/sql.Tx` 和 Ent generated `Tx`，公开 rollback API 均为 `Rollback() error`，无法接收 context。rollback 防泄漏应依赖事务 owner 的 defer、连接池/driver 行为和数据库 `idle_in_transaction_session_timeout` 等显式数据库策略。

理由：为 rollback 强行抽象 context 会要求新增 wrapper 或 adapter，但现有 concrete transaction 不能真正消费该 context，容易制造虚假的安全感和冗余生产代码。

备选方案：新增 `RollbackContext(context.Context)` optional interface 并包装所有事务。该方案被拒绝，因为当前底层事务 API 不支持 context rollback，新增接口无法提供真实跨实现保障。

### Decision: RBAC 路径只消费修正后的 datastore 语义

RBAC role、role permission、user role、bootstrap、seed/system binding 和 outbox claim 路径不新增本地兼容逻辑。它们继续通过 `datastore.BeginTransaction` 获得事务，只通过测试验证不再受 helper 隐式 5 秒限制。

理由：根因位于共享 datastore helper，RBAC 本地绕过或分支会重复事务策略并扩大维护面。

备选方案：仅在 RBAC transaction starter 中绕过 `datastore.BeginTransaction`。该方案被拒绝，因为会留下 common helper 缺陷并使其他消费者继续暴露风险。

## Risks / Trade-offs

- [Risk] 无原始 deadline 的长事务可持续超过 5 秒，占用连接或锁更久。→ Mitigation：长事务策略必须通过调用方 context deadline 或 PostgreSQL `statement_timeout`、`lock_timeout`、`idle_in_transaction_session_timeout` 显式配置；helper 不再提供隐藏策略。
- [Risk] 依赖旧单测期望的代码会失败。→ Mitigation：同步更新 `common/runtime/datastore/transaction_test.go`，把测试期望改为无原始 deadline 时不存在 helper 注入 deadline。
- [Risk] RBAC 高并发测试可能受本地数据库环境波动影响。→ Mitigation：优先用 deterministic lock holder 复现固定行锁等待，并只断言不存在 helper 层 5 秒断点；数据库环境不可用时保留单元级 coverage。
- [Risk] 回滚路径仍无 context 参数。→ Mitigation：保持 defer rollback 兜底，并把强制清理预算放在数据库 idle transaction timeout 等真实可执行的数据库策略上。

## Migration Plan

- 修改 `common/runtime/datastore` 的 lifecycle context 行为并更新注释，删除“无 deadline 时 5 秒 cleanup timeout 用于 BeginTx”的语义。
- 更新 datastore 单元测试，新增超过 `DefaultTransactionCleanupTimeout` 后仍可提交的无 deadline 场景。
- 增加或更新 user-service RBAC/PostgreSQL 事务回归测试，覆盖 policy revision counter 锁等待或等价事务生命周期超过 5 秒后仍可按策略提交。
- 运行 `make user-service-architecture-lint` 验证 OpenSpec 与架构规则。
- 运行相关 Go 测试，优先覆盖 `common/runtime/datastore` 和 `user-service/internal/features/role/infrastructure/postgres`。
- 回滚策略：恢复上一版本代码即可回到旧行为；不需要数据库回滚、数据迁移回滚或 OpenAPI 回滚。

## Open Questions

- 无。当前要求已明确为直接移除隐式 5 秒事务生命周期上限，不保留兼容方案。
