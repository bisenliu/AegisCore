## 1. 迁移准备

- [x] 1.1 梳理 `user-service/internal/features/role/application/seed/service_test.go` 中 role、permission、role-permission 和 user-role seed store double 的使用点，并按默认 seed、重复 seed、reactivate、sync bindings 和 assign super admin 场景分组。
- [x] 1.2 确认 `mock_generate.go` 已覆盖 `SeedRoleStore`、`SeedRolePermissionStore`、`SeedUserRoleStore`，且 `mock_permission_test.go` 已覆盖 `SeedPermissionStore`，不新增跨 feature seed mock 或共享测试替身包。

## 2. 测试迁移

- [x] 2.1 将默认 seed 场景改为使用 `NewMockSeedRoleStore`、`NewMockSeedPermissionStore` 和 `NewMockSeedRolePermissionStore`，并通过 `gomock.InOrder`、循环和 matcher 表达 `rbacbaseline.DefaultRoles()`、`DefaultPermissions()`、`DefaultRolePermissions()` 对应调用数量、参数和返回统计。
- [x] 2.2 将重复 seed 场景改为使用生成 mock 返回已存在写入结果，并断言 `SeedResult` 的 inserted、updated 和 binding added 统计保持既有语义。
- [x] 2.3 将 reactivate 场景改为通过 matcher 断言 `SeedRoleInput.ReactivateSystem` 和 `SeedPermissionInput.ReactivateSystem` 参数，并保留角色、权限和绑定调用顺序断言。
- [x] 2.4 将 sync bindings 场景改为通过 `SeedRolePermissionStore.SyncSystemBindings` expectation 表达新增和删除绑定统计，并确认该场景不调用 `EnsureSystemBindings`。
- [x] 2.5 将 assign super admin 场景改为使用 `NewMockSeedUserRoleStore`，并覆盖新增绑定和已有绑定两类返回结果，断言角色 ID 来自 `rbacbaseline.SuperAdminRoleID`。

## 3. 清理与生成物

- [x] 3.1 删除 `service_test.go` 中不再使用的 `seedRoleTestStore`、`seedPermissionTestStore`、`seedRolePermissionTestStore`、`seedUserRoleTestStore` 类型及其方法。
- [x] 3.2 保留只负责构造 service fixture、输入 matcher、UUID 解析或 baseline 期望的 helper，确认不存在实现 `SeedRoleStore`、`SeedPermissionStore`、`SeedRolePermissionStore` 或 `SeedUserRoleStore` 的新手写 double。
- [x] 3.3 执行 `make user-service-generate`，检查 `user-service/internal/features/role/application/seed/mock_generate.go` 和 `mock_permission_test.go` 无 mockgen drift。

## 4. 验证

- [x] 4.1 执行 `cd user-service && go test ./internal/features/role/application/seed`。
- [x] 4.2 执行 `make user-service-architecture-lint`。
- [x] 4.3 执行 `openspec validate standardize-rbac-seed-service-mocks`。
- [x] 4.4 暂存本次预期代码、测试和 OpenSpec 变更后执行 `make lint`。
- [x] 4.5 在本次预期变更已暂存的前提下执行 `make verify`，确认没有生成物 drift 或未纳入暂存区的意外变更。
- [x] 4.6 自动化执行器完成并清理临时文件后，执行 `git diff --exit-code` 确认无未暂存 drift。
