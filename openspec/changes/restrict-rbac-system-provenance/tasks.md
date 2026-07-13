## 1. 普通 API 创建路径收紧

- [x] 1.1 从 `CreateRoleRequest` 和 `CreatePermissionRequest` 删除公开 `system` 请求字段，并更新相关 swagger 注解或示例。
- [x] 1.2 从 `CreateRoleCommand`、`CreatePermissionCommand`、`CreateRoleInput` 和 `CreatePermissionInput` 删除调用方可控的 `IsSystem` 字段。
- [x] 1.3 更新 role/permission HTTP input preparer，使公开创建请求不再构造系统标记。
- [x] 1.4 更新 role/permission command service，使普通创建路径固定构造非系统数据。
- [x] 1.5 更新 `RoleStore.Create` 和 `PermissionStore.Create`，普通 create 显式写入 `is_system=false`。

## 2. Seed-only 系统写入

- [x] 2.1 从 `SeedRoleInput` 和 `SeedPermissionInput` 移除调用方可控的 `IsSystem` 字段，保留 seed 所需业务字段和 `ReactivateSystem`。
- [x] 2.2 更新 RBAC seed service 构造输入逻辑，使系统性由 seed port 语义决定，而不是由输入字段决定。
- [x] 2.3 更新 `UpsertSystemRole` 和 `UpsertSystemPermission`，固定写入 `is_system=true`，重复 seed 更新路径也必须保持系统标记。
- [x] 2.4 确认 `AssignSuperAdmin`、角色权限绑定、用户角色绑定、Casbin policy sync 和用户角色缓存语义不受本变更影响。

## 3. OpenAPI 与规格同步

- [x] 3.1 运行 `make user-service-openapi-generate` 生成 OpenAPI 文档，确认公开创建请求 schema 不含 `system`。
- [x] 3.2 检查 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml` diff，确认没有无关 drift。
- [x] 3.3 运行 `make user-service-architecture-lint` 验证 OpenSpec 与架构边界同步。

## 4. 测试覆盖

- [x] 4.1 更新 role HTTP controller/input 测试，反转当前 `system=true` 透传断言，覆盖公开 API 不制造系统角色。
- [x] 4.2 更新 permission HTTP controller/input 测试，反转当前 `system=true` 透传断言，覆盖公开 API 不制造系统权限。
- [x] 4.3 更新 role/permission command service 测试，覆盖普通创建固定非系统数据。
- [x] 4.4 更新 role/permission PostgreSQL store 测试，覆盖普通 create 写 `is_system=false`，seed upsert 写 `is_system=true`。
- [x] 4.5 更新 RBAC seed service 测试，确认 seed 输入不再携带系统标记，系统性由 seed port 写入语义保证。
- [x] 4.6 运行相关包测试：`go test ./user-service/internal/features/role/... ./user-service/internal/features/permission/... ./user-service/cmd`。

## 5. 最终验证

- [x] 5.1 运行 `make test`，确认普通测试集通过。
- [x] 5.2 使用 `git diff --exit-code -- user-service/docs` 或等效检查确认 OpenAPI 生成物已提交到工作树且无二次生成 drift。
- [x] 5.3 将本次预期代码、OpenAPI、OpenSpec artifacts 和相关文档变更加入暂存区。
- [x] 5.4 运行 `make lint`，未通过时修复后重新运行。
- [x] 5.5 运行 `make verify`，未通过时修复后重新运行。
- [x] 5.6 确认 `openspec status --change restrict-rbac-system-provenance` 显示 apply 所需 artifacts 完整，且所有实现任务 checkbox 已按实际完成状态更新。
