## 1. 基线扫描

- [x] 1.1 扫描 `user-service/internal/features/permission/transport/http` 中 controller、request、response、mapper、input preparer 和现有测试，列出权限目录、用户有效权限和 route diff endpoint 的覆盖缺口。
- [x] 1.2 确认现有 `mock_generate.go` 与 `mock_*_test.go` 是否已覆盖 `PermissionCommandService` 和 `PermissionQueryService`；如不足，使用既有生成入口更新 mock 生成物。
- [x] 1.3 确认本次只修改 permission HTTP boundary 测试和本 change artifacts，不修改生产 HTTP API、OpenAPI、RBAC 授权、policy sync、Redis watcher、数据库 schema、Atlas migration 或部署资产。

## 2. 权限目录 Controller 覆盖

- [x] 2.1 为 `ListPermissions` 补齐成功、非法 cursor/query、query service 错误和分页 envelope 映射测试。
- [x] 2.2 为 `CreatePermission` 补齐成功、JSON bind/validation 失败、input preparer 失败、command service 错误和 `201 Created` envelope 映射测试。
- [x] 2.3 为 `GetPermission` 补齐成功、非法 `permission_id`、query service not found/internal 错误和 permission response 映射测试。
- [x] 2.4 为 `UpdatePermission` 补齐 URI+JSON 组合绑定、成功、validation/input 失败、command service 错误和 permission response 映射测试。
- [x] 2.5 为 `EnablePermission`、`DisablePermission` 和内部 `setPermissionActive` 补齐成功、非法 `permission_id`、command service 错误和 permission response 映射测试。

## 3. 有效权限和 route diff Controller 覆盖

- [x] 3.1 为 `ListUserEffectivePermissions` 补齐成功、非法 `user_id`、query service 错误和有效权限 response 映射测试。
- [x] 3.2 为 `RouteDiff` 补齐成功、query service 错误和 missing/stale/mismatch response 映射测试。
- [x] 3.3 覆盖 mapper 和 input preparer 在 controller 路径中的关键分支，确保分页、UUID、方法、路径、启用状态和 route diff 诊断字段均有断言。

## 4. 边界约束和格式化

- [x] 4.1 确认新增测试使用 `testify/require` 或必要的 `assert` 语义化断言，不新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 或手写兼容 helper。
- [x] 4.2 运行 `rg "legacy|compat|旧|resource|action|alias|Failf?|FailNowf?\\(" user-service/internal/features/permission/transport/http --glob '*_test.go'`，确认没有新增旧权限资源路径、旧 action/resource 字段语义、旧 binding、旧 envelope、旧授权绕过或兼容断言路径；如有命中，逐项确认不是兼容断言。
- [x] 4.3 对修改过的 Go 测试文件运行 `gofmt`，并确认没有未使用 import 或生成物 drift。

## 5. 验证

- [x] 5.1 运行 `go test -coverprofile=/tmp/permission-http-cover.out ./user-service/internal/features/permission/transport/http` 并确认通过，且包覆盖率显著高于当前 42.6%。
- [x] 5.2 运行 `go tool cover -func=/tmp/permission-http-cover.out`，确认 Permission controller 未覆盖 handler 和主要 mapper/input preparer 均有覆盖。
- [x] 5.3 运行 `go test ./user-service/internal/features/permission/...` 并确认通过。
- [x] 5.4 运行 `openspec validate cover-permission-http-handlers-no-compat` 并确认通过。
- [x] 5.5 将本次预期代码、测试和 OpenSpec 产物加到暂存区后运行 `make lint`；如果被其他 active change 或 runtime 文件阻塞，记录具体原因且不把该项标为完成。
- [x] 5.6 保持本次预期变更已暂存后运行 `make verify`；如果被其他 active change 或 runtime 文件阻塞，记录具体原因且不把该项标为完成。
