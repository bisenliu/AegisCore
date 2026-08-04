## Why

当前 RBAC 在线 mutation 假定 migration 已写入固定 revision counter 行，但 Atlas schema migration 只稳定描述表结构，重新生成基线 SQL 时会丢失手写 seed DML，导致新数据库首次在线 RBAC 写入返回 counter not found。需要把 counter 缺失恢复放进权威 mutation transaction，避免部署正确性依赖生成物之外的数据语句。

## What Changes

- 在线 RBAC mutation 优先原子递增已有 counter；counter 缺失时读取已提交最大 revision，通过 Ent 幂等 upsert 初始化固定行，再执行原子递增。
- counter 初始化、业务 mutation、revision 和 outbox 继续位于同一 PostgreSQL transaction，任一步失败均完整回滚。
- Atlas migration 保持纯 schema，不新增或保留 counter seed `INSERT` SQL。
- 测试直接覆盖空 counter、已有 revision 对齐、多事务并发初始化和 HTTP E2E 首次写入。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 将固定 revision counter 的初始化从 migration DML 调整为在线 mutation transaction 内的 Ent 幂等初始化，同时保持 commit-ordered revision 语义。

## Impact

- 影响 `user-service/internal/features/role/infrastructure/postgres/tx.go` 的 revision 分配实现及相关 PostgreSQL/E2E 测试。
- `rbac_policy_revision_counters` 表结构不变，不新增 Ent schema 或 Atlas migration；现有 migration 移除手写 seed DML。
- 不改变 HTTP API、OpenAPI、Redis 协议、Casbin projection、部署清单、`common/`、`internal/shared/` 或 `internal/integration/`。
