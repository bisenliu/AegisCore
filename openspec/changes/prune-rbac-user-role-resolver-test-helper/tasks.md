## 1. 测试 helper 整理

- [x] 1.1 在 `user-service/internal/features/permission/infrastructure/casbin/policy.go` 中删除仅被同包测试使用的 `newUserRoleResolver` helper。
- [x] 1.2 在 `user-service/internal/features/permission/infrastructure/casbin/policy_test.go` 中将 `newTestUserRoleResolver` 改为直接返回 `&entUserRoleResolver{cache: cache}`。
- [x] 1.3 保留测试 helper 内 `localcache.New` 构造、loader、`cloneRoleIDs` 和 `t.Cleanup(cache.Close)`，确保缓存生命周期和测试注入方式不变。

## 2. 语义保持检查

- [x] 2.1 确认 `NewUserRoleResolver` 的 Fx 输入输出、lifecycle hook、cache 配置和 `UserRoleResolverResult` 暴露方式没有变化。
- [x] 2.2 确认 `RolesForUser`、`InvalidateUserRole`、`InvalidateAllUserRoles`、`loadRolesForUser` 和 `cloneRoleIDs` 行为没有变化。
- [x] 2.3 使用 `rg newUserRoleResolver` 确认 helper 已从生产文件和测试引用中移除。

## 3. 验证与收尾

- [x] 3.1 在 `user-service` 模块运行 `go test ./internal/features/permission/infrastructure/casbin`，确认 permission casbin 包测试通过。
- [x] 3.2 检查 `git diff`，确认代码改动只包含目标 `policy.go`、`policy_test.go` 和本 change artifacts。
- [x] 3.3 实现完成后将对应 tasks checkbox 更新为 `- [x]`，并确认 `openspec status --change prune-rbac-user-role-resolver-test-helper` 状态正常。
