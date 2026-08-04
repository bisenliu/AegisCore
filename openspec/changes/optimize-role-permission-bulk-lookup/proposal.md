## Why

当前 `ReplaceRolePermissions` 在 application 层对去重后的每个 `permission_id` 分别查询 PostgreSQL，导致完整替换写路径产生 O(N) 次权限查询；当请求包含 100 或 1000 个权限 ID 时，查询次数随集合大小线性增长。需要将该校验收敛为一次批量查询，同时保持缺失权限拒绝写入以及事务内批量重校验、完整替换和回滚语义不变。

## What Changes

- 为 permission application 查询端口和 PostgreSQL store 增加 `GetByPermissionIDs`，定义空输入、首次出现顺序去重、单次 `IN` 查询、缺失任一权限整体失败以及按去重输入顺序返回的语义。
- 为 role application 消费的 `PermissionLookup` 及其 PostgreSQL adapter 增加批量查询能力，将 permission domain 结果映射为有序的 `PermissionReference` 集合。
- 修改 `ReplaceRolePermissions`，保留现有 ID 去重和角色存在性校验，改为只调用一次 `GetByPermissionIDs`，并将有序结果交给现有 `RolePermissionStore.Replace`。
- 保留 `GetByPermissionID` 供 `AddRolePermission` 使用，并保留 `RolePermissionStore.Replace` 事务内现有的批量权限重校验，不增加逐条查询回退、feature flag、兼容开关或双读逻辑。
- 更新 mocks 和相关测试，覆盖空、单个、多个、重复、乱序返回、缺失权限、写入短路，以及 100/1000 个 ID 均只产生一次 permission lookup SQL 查询。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 明确角色权限完整替换必须在 application 层批量校验整个权限集合，校验查询次数不得随 ID 数量增长，任一权限不存在时不得执行绑定替换，并保持事务内完整替换与回滚语义。

## Impact

- Go 代码：影响 `user-service/internal/features/permission/application`、`user-service/internal/features/permission/infrastructure/postgres`、`user-service/internal/features/role/application` 和 `user-service/internal/features/role/infrastructure/postgres` 的端口、adapter、command、mocks 与测试。
- API 与契约：不修改 HTTP request DTO、公开 API 或错误对外契约；保留 `permissiondomain.ErrPermissionNotFound` 的 `errors.Is` 语义。
- 数据库：只调整现有权限表的查询方式，不修改 Ent schema、Atlas migration 或 Ent 生成代码。
- 授权与一致性：不修改 Casbin 授权循环、policy reload、事务内权限重校验、policy revision 或通知逻辑。
- 交付资产：不修改 OpenAPI 生成物、部署清单、观测资产、`common/` 共享契约或生产权限基线。
