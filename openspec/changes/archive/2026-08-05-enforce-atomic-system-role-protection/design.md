## Context

当前 role domain 通过 `RoleMutation{Name, Active}` 和 `ProtectSystemMutation` 保护系统角色，`UpdateRole`、`SetRoleActive` 在 application 层先读取角色再决定是否调用 store。该检查遗漏 `Description`，且读取发生在 store 事务之外。`RoleStore.Update` 与 `SetActive` 最终仅按 `role_id` 更新，没有锁定最新角色状态或限定 `is_system=false`。

角色权限 add、replace、remove 虽使用 `transactPolicyChange`，但 application 和 PostgreSQL store 都未拒绝 `IsSystem=true`。`getLockedRoleByExternalID` 实际只执行普通 Ent 查询，没有调用 `ForUpdate()`。成功 mutation 会在同一事务内追加 policy revision 和 outbox event，因此当前公开 API 能够原子提交不应被允许的系统角色漂移。

现有消费侧端口已区分普通 `RoleStore`、`RolePermissionStore` 与受信 `SeedRoleStore`、`SeedRolePermissionStore`，可以在 role feature 内完成修复。Ent 已为 PostgreSQL 查询生成 `ForUpdate()`，不需要修改 Ent schema、Atlas migration 或生成代码。

受影响路径包括：

- `user-service/internal/features/role/domain/role.go`
- `user-service/internal/features/role/application/command/role.go`、`binding.go`、生成 mocks 及测试
- `user-service/internal/features/role/infrastructure/postgres/role_store.go`、`role_permission_store.go`、事务 helper 及测试
- `user-service/internal/features/role/transport/http/` 和 `user-service/docs/openapi.*`
- `openspec/changes/enforce-atomic-system-role-protection/`

`common/`、`internal/shared/rbacbaseline`、`internal/integration`、permission policy loader、Redis policy sync、deployments 和观测资产不参与本次实现。

## Goals / Non-Goals

**Goals:**

- 所有普通 metadata、状态和角色权限写端口在同一 PostgreSQL transaction 内基于锁定后的最新角色状态强制系统角色不变量。
- 任何指向系统角色的普通写请求，包括 description-only 和目标值相同的请求，统一返回 `ErrSystemRoleProtected`，HTTP 映射为 `409 Conflict`。
- 拒绝路径不改变角色、角色权限绑定、policy revision counter、policy revision、outbox event，也不发送 policy change 通知。
- 在并发 seed 更新持有角色行锁时，普通写等待其提交，随后读取最新 `IsSystem=true` 并拒绝。
- 保持 seed 通过独立受信端口维护系统角色和基线绑定，不引入兼容开关、双路径或回退逻辑。

**Non-Goals:**

- 不修改角色和权限数据库结构，不增加 trigger、stored procedure 或 migration。
- 不修改普通角色的可写字段、角色权限 application 批量查询和事务内权限重校验语义。
- 不修改超级管理员 wildcard policy、用户角色绑定、policy reload、Redis watcher、outbox dispatcher 或多副本收敛流程。
- 不在 `common/`、`internal/shared` 或 `internal/integration` 引入 role feature 业务保护 helper。

## Decisions

### Decision: 普通写端口共享事务内角色锁与系统标记检查

在 `role/infrastructure/postgres` 内提供未导出的 helper。helper 必须通过当前 `*ent.Tx` 按外部 `role_id` 查询角色并调用 `ForUpdate()`；不存在时返回 `ErrRoleNotFound`，`IsSystem=true` 时返回 `ErrSystemRoleProtected`，否则返回锁定角色。`RoleStore.Update`、`SetActive` 和 `RolePermissionStore.Add`、`Replace`、`Remove` 必须在各自 `transactPolicyChange` mutation closure 的第一步调用该 helper。

metadata UPDATE 继续按锁定角色内部 ID 写入，并额外限定 `is_system=false`。角色权限 mutation 依赖同一事务持有的角色行锁和 `IsSystem` 检查，再执行权限重校验及关系写入。所有能把普通角色提升为系统角色的受信 UPDATE 都会与该行锁互斥，因此检查与普通写入具有明确线性化顺序。

不采用 application-only 检查，因为它不能封闭检查与提交之间的 TOCTOU 窗口；不采用数据库 trigger，因为受信 seed 必须维护系统记录，且现有分离端口和事务行锁已经能在不引入 schema 变更的前提下表达边界。

### Decision: 删除 domain/application 保护分支，由 store 错误作为唯一权威结果

删除 `RoleMutation` 与 `ProtectSystemMutation`。`UpdateRole` 和 `SetRoleActive` 不再为了系统保护或 no-op 在事务外读取角色，规范化输入后直接调用普通 store。角色权限 command 不再以普通角色查询结果作为系统保护依据；permission application 端口的现有合法性校验继续保留，最终角色存在性和可写性由 transaction 内 store 检查决定。

因此系统角色即使提交与当前值相同的 metadata 或状态，也会进入普通 store 并返回 `ErrSystemRoleProtected`。普通角色目标值相同的写请求按统一 mutation 流程提交，不保留旧的 application no-op 快速路径。

不保留 application 防御性副本检查，因为两个保护源会再次产生字段遗漏、错误优先级和并发语义分歧。

### Decision: 保护失败复用 transactPolicyChange 回滚边界

锁定 helper 在 mutation closure 内、任何业务写入之前返回 `ErrSystemRoleProtected`。`transactPolicyChange` 收到错误后立即回滚，不执行 revision counter 分配、revision 插入、outbox 插入或 commit。application 只在 store 成功返回 revision 后通知 policy change，因此保护失败不会发送通知或触发 reload。

不在事务外预先创建 revision 或补偿删除 outbox；拒绝操作从一开始就不产生数据库事实。

### Decision: seed 只保留现有受信端口

`SeedRoleStore.UpsertSystemRole`、`SeedRolePermissionStore.EnsureSystemBindings` 和 `SyncSystemBindings` 继续负责系统基线维护，不调用普通写 helper，也不暴露给 HTTP command service。普通 store 和 seed store 可以继续由现有 concrete adapter 实现，但 Fx 接线与 application 依赖必须保持接口隔离。

不增加 feature flag、特殊请求 header、调用方白名单或普通 store 的 bypass 参数；系统写权限只能由 seed 端口的类型边界表达。

### Decision: 对外错误固定为 409 并同步 OpenAPI

沿用现有 `ErrSystemRoleProtected` contract error，不新增错误码。metadata、状态和角色权限 controller 均通过统一 response renderer 输出 `409 Conflict`。修正相关 Swagger 注解后运行 `make user-service-openapi-generate`，提交三个 OpenAPI 生成物。

不保留旧成功响应，也不将保护错误伪装成 `404` 或 `403`。

## Risks / Trade-offs

- [Risk] `SELECT ... FOR UPDATE` 会让针对同一角色的普通写与 seed 串行化，增加局部等待 → Mitigation：锁粒度限定为单个角色行，事务内在锁后立即完成校验和 mutation，不引入跨角色锁。
- [Risk] metadata UPDATE 与角色权限 mutation 的锁顺序不一致可能产生死锁 → Mitigation：所有普通角色写都先锁角色，再访问 permission 或 role_permissions，并用并发 PostgreSQL 测试固定顺序。
- [Risk] 删除 application no-op 快速路径会让普通角色相同值更新产生 revision 和 outbox → Mitigation：将其作为统一写语义接受并更新测试；不为旧优化重新增加事务外分支。
- [Risk] 仅验证返回错误而遗漏事务副作用 → Mitigation：集成测试在每个保护场景前后比较角色、绑定、revision counter、revision 和 outbox 精确快照，并断言 notifier 零调用。
- [Trade-off] 同一 concrete adapter 仍实现普通与 seed 接口 → 依赖隔离由消费侧最小端口和 Fx 接线保证，避免为了类型拆分复制数据库实现。

## Migration Plan

1. 在同一 user-service 版本内完成普通 store 行锁保护、application/domain 清理、测试、HTTP 注解和 OpenAPI 生成物更新。
2. 不执行数据库 migration 或数据回填；发布前运行 RBAC seed，使已有系统角色 metadata 和权限绑定先收敛到代码基线。
3. 依次运行定向 Go 测试、OpenAPI 生成检查、`make user-service-architecture-lint`、OpenSpec validate；全部预期变更暂存后运行 `make lint` 和 `make verify`。
4. 安全边界不提供兼容回滚。若新版本在接流量前失败，可整体回退镜像；若接流量后发现运行时问题，必须先在入口禁用角色普通写路由，再回退或 fix-forward，避免重新暴露系统角色写入漏洞。

## Open Questions

无。

## Verification

- `go test ./user-service/internal/features/role/domain ./user-service/internal/features/role/application/command`
- `go test ./user-service/internal/features/role/infrastructure/postgres ./user-service/internal/features/role/transport/http`
- `make user-service-openapi-generate`
- `make user-service-architecture-lint`
- `openspec validate enforce-atomic-system-role-protection`
- 预期变更暂存后运行 `make lint` 和 `make verify`
