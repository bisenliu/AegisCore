## 1. 测试基础设施

- [x] 1.1 整理 `user-service/internal/features/role/infrastructure/postgres` 同包 PostgreSQL 测试辅助函数，复用当前 Ent schema、外部 UUID 字段和现有 container harness。
- [x] 1.2 增加角色、用户、权限和绑定测试数据 helper，避免引入旧 internal ID、旧 role code 或兼容查询 helper。

## 2. RoleStore 覆盖

- [x] 2.1 覆盖 `RoleStore.Create`、`GetByRoleID`、`GetByRoleIDs` 的成功、空列表、重复 ID、not found 和唯一约束冲突路径。
- [x] 2.2 覆盖 `RoleStore.List` 的外部 UUID 排序、分页、active/is_system 过滤和空结果路径。
- [x] 2.3 覆盖 `RoleStore.Update` 和 `SetActive` 的成功、not found 和唯一约束冲突路径。

## 3. UserRoleStore 覆盖

- [x] 3.1 覆盖 `UserRoleStore.Add`、`ListByUserID` 和 `Remove` 的成功、重复绑定、缺失用户、缺失角色和缺失绑定路径。
- [x] 3.2 覆盖 `UserRoleStore.Replace` 的成功替换、空集合、缺失用户、缺失角色、重复 role ID 和失败不破坏旧绑定路径。

## 4. RolePermissionStore 覆盖

- [x] 4.1 覆盖 `RolePermissionStore.Add`、`ListByRoleID` 和 `Remove` 的成功、重复绑定、缺失角色、缺失权限和缺失绑定路径。
- [x] 4.2 覆盖 `RolePermissionStore.Replace` 的成功替换、空集合、重复 permission ID、inactive/missing permission 和事务回滚路径。

## 5. 断言规范与验证

- [x] 5.1 检查新增测试中 `t.Fatal`、`t.Error`、`Fail`、`Failf` 使用符合 `docs/TESTING.md` 例外规则，常见断言使用语义化 `require` 或允许边界内的 `assert`。
- [x] 5.2 运行 `go test -cover ./user-service/internal/features/role/infrastructure/postgres`。
- [x] 5.3 运行 `go test -coverprofile /tmp/role-postgres.cover ./user-service/internal/features/role/infrastructure/postgres` 和 `go tool cover -func /tmp/role-postgres.cover`，确认 `RoleStore`、`UserRoleStore`、`RolePermissionStore` 主要 CRUD 方法有覆盖。
- [x] 5.4 运行 `openspec validate cover-role-postgres-stores-no-compat`。
- [x] 5.5 暂存本次预期代码和 OpenSpec 变更后运行 `make lint` 与 `make verify`；若因已知 runtime 文件或环境限制无法完成，记录具体原因和已替代运行的验证命令。
