## Why

RBAC seed 在补齐或同步系统角色权限绑定时，需要把 catalog 中的外部 `permission_id` 批量解析为权限表内部 ID。当前 `RolePermissionStore.permissionsByExternalIDs` 对去重后的每个 `permission_id` 逐个调用 `getPermissionByExternalID`，会产生 N 次权限查询。

随着系统权限数量增长，`rbac seed` 和 `rbac seed --sync-system-bindings` 的总耗时会随权限数量线性放大，并增加数据库往返次数。该查询发生在绑定写入前，当前不会直接拉长 `SyncSystemBindings` 的显式事务窗口，但会拖慢整个 seed 运维流程。

## What Changes

- 将 role PostgreSQL adapter 中 seed 使用的权限 ID 解析从逐个查询改为一次 `WHERE permission_id IN (...)` 批量查询。
- 保留输入去重行为，避免重复 catalog 权限造成重复解析或重复绑定。
- 校验批量查询返回数量必须覆盖所有去重后的外部权限 ID；缺失权限继续返回现有角色权限 not found 语义。
- 保持 `EnsureSystemBindings` 和 `SyncSystemBindings` 的既有业务语义不变：默认补齐绑定，显式同步时才删除多余绑定。
- 不修改 HTTP `ReplaceRolePermissions` 的 application 层逐个启用权限校验；该路径是相邻性能优化，不属于本次变更范围。

## Capabilities

### Modified Capabilities

- `rbac-seed-workflow`: 优化系统角色权限绑定 seed 前的权限 ID 解析，降低权限数量较大时的数据库查询次数。

### New Capabilities

无。

## Impact

- 影响 `user-service/internal/features/role/infrastructure/postgres/role_permission_store.go` 中 seed 角色权限绑定解析 helper。
- 影响该 adapter 的 PostgreSQL 测试，需覆盖批量查询、重复输入去重、缺失权限报错和 seed 绑定行为不变。
- 不变更 Ent schema、Atlas migration、HTTP API、OpenAPI 文档、Casbin policy 结构或 RBAC catalog。
- 不新增 `openspec/` 或 OPSX 工件。
