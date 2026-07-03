## Context

`user-service/internal/features/role/infrastructure/postgres` 已实现 `RoleStore`、`UserRoleStore`、`RolePermissionStore` 和 `PermissionLookup`，application port 以外部 UUID 作为业务标识，Ent 内部自增 ID 仅用于 join 表外键。当前同包测试主要覆盖 seed store 和角色权限绑定 active permission 复核，Role CRUD、用户角色绑定和角色权限绑定的常规路径、异常路径、重复输入和事务回滚覆盖不足。

本次 change 只补齐当前实现的回归测试，并用 OpenSpec 明确 Role PostgreSQL store 的持久化契约；不改变 HTTP API、application port、Ent schema、Atlas migration 或部署资产。

## Goals / Non-Goals

**Goals:**

- 覆盖 `RoleStore` 的 `Create`、`GetByRoleID`、`GetByRoleIDs`、`List`、`Update`、`SetActive`。
- 覆盖 `UserRoleStore` 的 `ListByUserID`、`Add`、`Replace`、`Remove`。
- 覆盖 `RolePermissionStore` 的 `ListByRoleID`、`Add`、`Replace`、`Remove`。
- 覆盖 not found、唯一约束冲突、空列表、重复 ID、事务回滚和领域错误映射。
- 新增测试使用当前 Ent schema、外部 UUID 字段和当前领域错误，不保留旧兼容路径。
- 测试断言遵循 `docs/TESTING.md`，优先使用语义化 `require`。

**Non-Goals:**

- 不修改 Ent schema、Atlas migration 或生成代码。
- 不修改 role、permission、user application port 定义。
- 不新增旧 internal ID 查询入口、旧 role code 字段、旧 binding 行为或兼容 helper。
- 不修改 HTTP controller、OpenAPI、Casbin policy sync、RBAC seed 或部署资产。

## Decisions

- `RoleStore` 和 `UserRoleStore` 的 CRUD 测试使用同包 sqlite `enttest` helper 覆盖默认 `go test` 路径，`RolePermissionStore` 的事务和 `FOR UPDATE` 相关测试继续使用现有 PostgreSQL container harness。原因是普通 Ent CRUD 与唯一约束可在默认测试中快速回归，而角色权限绑定需要真实 PostgreSQL 行为；备选方案是所有测试都走 container，但会让未启用 `AEGISCORE_TEST_CONTAINERS` 的默认覆盖仍停留在旧水平。
- 测试数据直接通过当前 store 或 Ent client 构造。原因是本次目标是 infrastructure store 边界，允许在同包测试中读取 Ent 记录验证绑定状态；备选方案是只走 application use case，但会把 command/query 行为混入 store 测试。
- 绑定替换的失败路径通过缺失或 inactive 引用触发，并断言旧绑定仍保留。原因是当前实现先校验引用再删除重建，或在事务内回滚；备选方案是 mock Ent 错误，但会削弱对当前 schema 和真实约束的覆盖。
- 重复 ID 场景按当前实现验证去重或错误语义，不新增兼容逻辑。`RolePermissionStore.Replace` 对重复 permission UUID 去重；`UserRoleStore.Replace` 当前通过查询数量与输入数量一致性发现重复或缺失并返回 `ErrRoleNotFound`。

## Risks / Trade-offs

- PostgreSQL container 测试耗时高于 sqlite 测试 -> 默认 CRUD 覆盖使用 sqlite `enttest`，仅 `RolePermissionStore` 的事务锁语义使用 container，并限制测试数据规模。
- 直接验证 Ent join 表会让测试绑定当前 schema -> 这是本次目标的一部分，schema 变化需要显式更新测试。
- `UserRoleStore.Replace` 对重复角色 ID 的当前错误语义不够理想 -> 本次只固化当前无兼容行为，不新增生产逻辑；若未来要改变语义，应单独创建 change。
- 覆盖率目标依赖包内所有测试一起运行 -> 验证使用 `go test -cover ./user-service/internal/features/role/infrastructure/postgres` 和 `go tool cover -func`。

## Migration Plan

无需数据迁移或发布编排。该 change 只新增或调整测试与 OpenSpec delta；回滚时删除新增测试和 change artifacts 即可。

验证命令：

- `go test -cover ./user-service/internal/features/role/infrastructure/postgres`
- `go test -coverprofile /tmp/role-postgres.cover ./user-service/internal/features/role/infrastructure/postgres`
- `go tool cover -func /tmp/role-postgres.cover`
- `openspec validate cover-role-postgres-stores-no-compat`

## Open Questions

无。
