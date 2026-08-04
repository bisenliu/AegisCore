## 1. 实现 permission 批量查询

- [x] 1.1 在 `user-service/internal/features/permission/application/ports.go` 的 `PermissionStore` 增加 `GetByPermissionIDs`，并在 PostgreSQL store 实现空输入非 nil 空 slice、首次出现顺序去重、单次 `PermissionIDIn` 查询、map 重排、缺失任一 ID 整体返回 `permissiondomain.ErrPermissionNotFound` 及既有 `%w` 错误包装语义。
- [x] 1.2 扩展 `user-service/internal/features/permission/infrastructure/postgres/permission_store_test.go`，覆盖空输入且零数据库查询、单个 ID、多个 ID、重复 ID 首次顺序、数据库返回顺序与输入不同、任一 ID 缺失且无部分结果。
- [x] 1.3 使用真实 PostgreSQL test fixture 创建 100 条和 1000 条测试权限，以测试专用 SQL 查询计数器分别断言 `GetByPermissionIDs` 的 permission lookup SQL 查询次数均为 1，且不修改生产权限基线。

## 2. 切换角色权限完整替换路径

- [x] 2.1 在 role application 的 `PermissionLookup` 增加 `GetByPermissionIDs`，由 `user-service/internal/features/role/infrastructure/postgres/permission_lookup.go` 调用 permission application 批量端口一次并按原顺序映射为 `[]PermissionReference`；保留 `GetByPermissionID` 供 `AddRolePermission` 使用。
- [x] 2.2 修改 `ReplaceRolePermissions`，保留角色存在性校验和 `uniqueUUIDs`，删除逐 permission ID 查询循环，只调用一次 `GetByPermissionIDs` 并将有序结果原样传给现有 `RolePermissionStore.Replace`；不得删除事务内 `lockedPermissionsByExternalIDs` 批量重校验或修改 policy reload 流程。
- [x] 2.3 从 `user-service/` 对 permission query 与 role command 的 mock 生成入口运行定向 `go generate`，同步生成 mocks 和 `user-service/internal/features/role/fx_test.go` 等接口测试替身；确认没有 Ent、migration 或 OpenAPI 生成物变更。
- [x] 2.4 更新 role command 测试，断言 `ReplaceRolePermissions` 只调用一次 `GetByPermissionIDs`、不再逐个调用 `GetByPermissionID`、lookup 失败时不调用 `RolePermissionStore.Replace` 或通知、成功时传给 Replace 的权限顺序等于去重后的首次出现顺序，并保持 `AddRolePermission` 单权限测试。
- [x] 2.5 更新 role PostgreSQL permission lookup adapter 测试，覆盖空、单个、多个、重复、顺序映射与 `permissiondomain.ErrPermissionNotFound` 的 `errors.Is` 传播；保留并验证 `RolePermissionStore.Replace` 事务内批量重校验和缺失权限回滚测试。

## 3. 定向验证

- [x] 3.1 运行 `go test ./user-service/internal/features/permission/infrastructure/postgres`，确认批量查询语义及 100/1000 ID SQL 查询计数测试通过。
- [x] 3.2 运行 `go test ./user-service/internal/features/role/application/command`，确认完整替换只执行一次批量 lookup，缺失权限不产生绑定写入。
- [x] 3.3 运行 `go test ./user-service/internal/features/role/infrastructure/postgres`，确认 adapter 映射与事务内 revalidation、完整替换和回滚语义通过。
- [x] 3.4 运行 `make user-service-architecture-lint` 和 `openspec validate optimize-role-permission-bulk-lookup`。

## 4. 合并前门禁

- [x] 4.1 检查 `git diff`，确认 application 中不存在逐 permission ID 查询循环，完整路径仅包含一次 application 批量查询和一次事务内批量 revalidation，且无 Ent、migration、OpenAPI、部署、生产权限基线或无关文件变更。
- [x] 4.2 暂存本 change 的全部预期代码、生成 mock、测试和 OpenSpec artifacts；再次运行定向 `go generate` 并执行 `git diff --exit-code`，确认没有未暂存的生成物 drift。
- [x] 4.3 在预期变更全部暂存后运行 `make lint`；仅在命令通过后将本任务标记完成。
- [x] 4.4 在预期变更全部暂存后运行 `make verify`；仅在相关测试、生成检查和最终 `git diff --exit-code` 全部通过后将本任务及 change 标记完成。
