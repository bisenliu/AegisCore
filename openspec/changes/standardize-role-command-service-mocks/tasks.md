## 1. 迁移准备

- [x] 1.1 梳理 `user-service/internal/features/role/application/command/service_test.go` 中 role、user-role、role-permission、permission lookup 和 policy change notifier 测试协作者的使用点，并按角色生命周期、用户角色绑定、角色权限绑定和通知路径分组。
- [x] 1.2 确认 `mock_generate.go` 已覆盖 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup`，且 `mock_permission_test.go` 已覆盖 `PolicyChangeNotifier`，不新增跨包共享 mock 仓库。

## 2. 测试迁移

- [x] 2.1 将角色创建、更新、启用、停用和系统角色保护相关测试改为使用 `NewMockRoleStore`，并通过 expectation 表达输入归一化、错误映射、禁止写入和写入成功后的 policy change 通知。
- [x] 2.2 将用户角色添加、移除和替换相关测试改为使用 `NewMockRoleStore`、`NewMockUserRoleStore` 和 `NewMockPolicyChangeNotifier`，并通过 matcher 或 `DoAndReturn` 表达角色 ID 去重、角色存在性查询、绑定替换结果和用户角色缓存失效通知。
- [x] 2.3 将角色权限添加、移除和替换相关测试改为使用 `NewMockPermissionLookup`、`NewMockRolePermissionStore` 和 `NewMockPolicyChangeNotifier`，并明确断言权限查找失败或权限不可用时不会执行绑定写入或通知。
- [x] 2.4 为角色写操作、用户角色绑定和角色权限绑定成功后 `NotifyPolicyChanged` 返回错误的路径声明生成 mock expectation，并断言 command service 按既有语义吞掉通知失败。

## 3. 清理与生成物

- [x] 3.1 删除 `service_test.go` 中不再使用的 `roleTestStore`、`userRoleTestStore`、`rolePermissionTestStore`、`permissionLookupTestStore`、`recordingRolePolicyChangeNotifier` 类型及其方法。
- [x] 3.2 保留只负责构造输入、领域对象、fixture 或 matcher 的 helper，确认不存在实现 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup` 或 `PolicyChangeNotifier` 的新手写 double。
- [x] 3.3 执行 `make user-service-generate`，检查 `user-service/internal/features/role/application/command/mock_test.go` 和 `mock_permission_test.go` 无 mockgen drift。

## 4. 验证

- [x] 4.1 执行 `cd user-service && go test ./internal/features/role/application/command`。
- [x] 4.2 执行 `make user-service-architecture-lint`。
- [x] 4.3 执行 `openspec validate standardize-role-command-service-mocks`。
- [x] 4.4 暂存本次预期代码、测试和 OpenSpec 变更后执行 `make lint`。
- [ ] 4.5 在本次预期变更已暂存的前提下执行 `make verify`，确认没有生成物 drift 或未纳入暂存区的意外变更。
