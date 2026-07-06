## 1. 基线扫描

- [x] 1.1 扫描 `user-service/internal/features/role/transport/http` 中 controller、request、response、mapper、input preparer 和现有测试，列出角色生命周期、角色权限绑定和用户角色绑定 endpoint 的覆盖缺口。
- [x] 1.2 确认现有 `mock_generate.go` 与 `mock_*_test.go` 是否已覆盖 `RoleCommandService` 和 `RoleQueryService`；如不足，使用既有生成入口更新 mock 生成物。
- [x] 1.3 确认本次只修改 role HTTP boundary 测试和本 change artifacts，不修改生产 HTTP API、OpenAPI、RBAC 授权、数据库 schema、Atlas migration 或部署资产。

## 2. 角色生命周期 Controller 覆盖

- [x] 2.1 为 `ListRoles` 补齐成功、非法 cursor/query、query service 错误和分页 envelope 映射测试。
- [x] 2.2 为 `CreateRole` 补齐成功、JSON bind/validation 失败、input preparer 失败、command service 错误和 `201 Created` envelope 映射测试。
- [x] 2.3 为 `GetRole` 补齐成功、非法 `role_id`、query service not found/internal 错误和 role response 映射测试。
- [x] 2.4 为 `UpdateRole` 和 `SetRoleStatus` 补齐 URI+JSON 组合绑定、成功、validation/input 失败、command service 错误和 role response 映射测试。

## 3. 角色权限绑定 Controller 覆盖

- [x] 3.1 为 `ListRolePermissions` 补齐成功、非法 `role_id`、query service 错误和 permission response 映射测试。
- [x] 3.2 为 `ReplaceRolePermissions` 补齐成功、非法 ID 集合、command service 错误和 permission 列表 envelope 映射测试。
- [x] 3.3 为 `AddRolePermission` 和 `RemoveRolePermission` 补齐成功、非法 `role_id` / `permission_id`、command service conflict/not found/internal 错误和 response 映射测试。

## 4. 用户角色绑定 Controller 覆盖

- [x] 4.1 为 `ListUserRoles` 补齐成功、非法 `user_id`、query service 错误和 role response 映射测试。
- [x] 4.2 为 `ReplaceUserRoles` 补齐成功、非法 ID 集合、command service 错误和 role 列表 envelope 映射测试。
- [x] 4.3 为 `AddUserRole` 和 `RemoveUserRole` 补齐成功、非法 `user_id` / `role_id`、command service conflict/not found/internal 错误和 response 映射测试。

## 5. 边界约束和格式化

- [x] 5.1 确认新增测试使用 `testify/require` 或必要的 `assert` 语义化断言，不新增机械 `Fail` / `Failf` 或手写兼容 helper。
- [x] 5.2 运行 `rg "legacy|compat|旧|code|alias|Failf?\\(" user-service/internal/features/role/transport/http --glob '*_test.go'`，确认没有新增旧字段、旧 binding、旧 envelope、旧错误码或兼容断言路径；如有命中，逐项确认不是兼容断言。
- [x] 5.3 对修改过的 Go 测试文件运行 `gofmt`，并确认没有未使用 import 或生成物 drift。

## 6. 验证

- [x] 6.1 运行 `go test ./user-service/internal/features/role/transport/http` 并确认通过。
- [x] 6.2 运行 `go test ./user-service/internal/features/role/...` 并确认通过。
- [x] 6.3 运行 `openspec validate cover-role-http-boundary-no-compat` 并确认通过。
- [x] 6.4 将本次预期代码、测试和 OpenSpec 产物加到暂存区后运行 `make lint`；如果被其他 active change 或 runtime 文件阻塞，记录具体原因且不把该项标为完成。
- [x] 6.5 保持本次预期变更已暂存后运行 `make verify`；如果被其他 active change 或 runtime 文件阻塞，记录具体原因且不把该项标为完成。
