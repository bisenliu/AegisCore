## Why

`role` feature 的 PostgreSQL infrastructure 目前对角色、用户角色绑定和角色权限绑定 store 的 CRUD 路径覆盖不足，导致外部 UUID 字段、当前 Ent schema、事务回滚和领域错误映射缺少回归保护。需要在不保留旧表结构、旧查询语义或兼容 helper 的前提下补齐测试，确保 RBAC 持久化边界按当前规格稳定工作。

## What Changes

- 为 `RoleStore` 补齐创建、按 `role_id` 查询、批量查询、列表、更新和启停测试。
- 为 `UserRoleStore` 补齐按用户查询、替换和移除用户角色绑定测试。
- 为 `RolePermissionStore` 补齐按角色查询、添加、替换和移除角色权限绑定测试。
- 覆盖 not found、唯一约束冲突、空列表、重复 ID、事务回滚和领域错误映射。
- 新增测试遵循 `docs/TESTING.md` 中固化的断言规范，常见断言使用语义化 `require` 或允许边界内的 `assert`。
- 不修改 Ent schema、Atlas migration、application port、HTTP API、OpenAPI 或部署资产。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确 Role PostgreSQL store 必须以当前 Ent schema 和外部 UUID 字段实现角色与绑定持久化，并保持当前领域错误映射和事务一致性。

## Impact

- 影响代码范围：`user-service/internal/features/role/infrastructure/postgres/` 同包测试，以及必要时仅限测试辅助函数。
- 影响验证：需要运行 `go test -cover ./user-service/internal/features/role/infrastructure/postgres`、`go tool cover -func` 和 `openspec validate cover-role-postgres-stores-no-compat`。
- 不影响外部 API、数据库 schema、Atlas migration、OpenAPI 文档、运行时配置、部署流程或共享契约。
