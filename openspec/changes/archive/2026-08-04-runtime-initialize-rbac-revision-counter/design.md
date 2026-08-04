## Context

在线 RBAC mutation 通过固定单行 `rbac_policy_revision_counters` 的原子递增和 PostgreSQL 行锁串行化 revision 分配。当前实现直接更新 `id=1`，而 counter 行依赖 migration 中的手写 DML；基线 migration 由 Ent/Atlas schema 重新生成后不保留数据语句，使新数据库首次角色、角色权限或用户角色写入失败。

本次变更归属 user-service role PostgreSQL infrastructure。counter 初始化包含 RBAC revision 语义，不进入 `common/`、`internal/shared/` 或 `internal/integration/`。表结构、HTTP API、OpenAPI、Redis/Casbin 同步协议、部署清单和观测资产均不变。

## Goals / Non-Goals

**Goals:**

- 让缺少固定 counter 行的新数据库在首次在线 RBAC mutation 中自动恢复。
- 已有 revision 时从数据库最大已提交 revision 继续，避免 revision 冲突或倒退。
- 保持 counter 行锁、transactional outbox 和 commit-ordered revision 的既有正确性。
- 让 Atlas migration 保持纯 schema，不依赖手写 seed `INSERT` SQL。

**Non-Goals:**

- 不移除 counter 表，不改用 sequence、Redis counter 或 advisory lock。
- 不支持旧二进制与新二进制长期混合写入造成的 counter 人为落后修复。
- 不改变 RBAC seed、HTTP DTO、错误码或 projection 恢复协议。

## Decisions

### Decision: 在 revision 分配函数中提供缺失恢复

正常路径继续对固定 counter 行执行 `UPDATE last_revision = last_revision + 1`，保持单次数据库 mutation 和现有行锁成本。仅当 Ent 返回 not found 时，事务读取 `rbac_policy_revisions` 当前最大值，以该值通过 `OnConflict(...).Ignore()` 幂等创建固定行，然后重试原子递增。`Ignore()` 在冲突时仅将现有列设为自身，不会使用过期初始值覆盖 `last_revision`，且避免单行 Ent upsert 在 `DO NOTHING` 无返回行时产生 not found。

并发首次写入可能同时观察到 counter 缺失，但固定主键冲突会使竞争事务等待创建者提交或回滚；创建者后续仍持有 counter 行锁直到整个 policy transaction 结束，因此后续事务不能以更大 revision 先提交。

备选方案是在 migration 中保留 seed DML。该方案运行时最简单，但数据语句不属于 Ent/Atlas schema，重新生成基线时容易再次丢失，因此不采用。备选方案 transaction advisory lock 无需 counter 行，但需要绕过当前 Ent transaction 边界，并改变已验收的并发机制，因此不采用。

### Decision: 初始化与业务 mutation 使用同一 transaction

counter 的查询、幂等创建、递增、revision 和 outbox 写入均复用现有 Ent transaction。初始化后任一 revision/outbox/commit 失败都回滚 counter 创建或递增，调用方不会观察到无对应业务事实的 revision。

### Decision: 测试夹具不预建 counter

直接使用 `Schema.Create` 或 Atlas migration 的 PostgreSQL 测试不再额外 seed counter，使 adapter 并发测试和 HTTP E2E 验证生产初始化路径。已有 SQLite 测试可保留显式 counter 以覆盖正常快速路径，另以 PostgreSQL 测试覆盖缺失恢复和真实锁行为。

## Risks / Trade-offs

- [首次在线 mutation 增加一次查询、一次 upsert 和一次重试更新] -> 仅 counter 缺失时发生；成功初始化后恢复为单次原子更新。
- [并发首次初始化处理错误可能破坏提交顺序] -> 使用真实 PostgreSQL barrier 与并发写测试验证等待、连续 revision 和 outbox 唯一性。
- [已有 counter 行人为落后于最大 revision] -> 本变更只处理缺失状态；禁止旧新二进制长期混写，并保留 revision 唯一约束使异常 fail-closed。
- [运行时首次写入承担初始化职责] -> 初始化位于同一 transaction，失败可重试且不产生部分提交；E2E 覆盖空 counter 数据库。

## Migration Plan

1. 移除基线 migration 中手写 counter seed DML并刷新 `atlas.sum`。
2. 发布包含运行时幂等初始化的新二进制；首次在线 RBAC mutation 自动创建固定 counter 行。
3. 观察首次 RBAC 写入、revision/outbox 唯一性和 policy reload lag。
4. 回滚应用代码时，已创建的 counter 行继续兼容旧实现；无需删除数据或回滚 schema。

## Open Questions

无。
