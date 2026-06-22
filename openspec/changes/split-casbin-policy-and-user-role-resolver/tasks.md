## 1. 文件职责拆分

- [x] 1.1 保留 `user-service/internal/features/permission/infrastructure/casbin/policy.go` 作为 policy loader 文件，承载 `policyWildcard`、`PolicySet`、`PermissionRule`、`Loader`、`LoaderParams`、`entLoader`、`NewPolicyLoader`、`LoadPolicies` 和 `loadPermissionRules`。
- [x] 1.2 新增 `user-service/internal/features/permission/infrastructure/casbin/user_role_resolver.go`，迁入 `UserRoleResolver`、`entUserRoleResolver`、`RolesForUser`、`InvalidateUserRole`、`InvalidateAllUserRoles`、`loadRolesForUser` 和 `cloneRoleIDs`。
- [x] 1.3 新增 `user-service/internal/features/permission/infrastructure/casbin/user_role_cache.go`，迁入 `rbacUserRolesCacheName`、`UserRoleResolverParams`、`UserRoleResolverResult` 和 `NewUserRoleResolver`。
- [x] 1.4 新增 `user-service/internal/features/permission/infrastructure/casbin/subject.go` 或等价小文件，迁入 `roleSubject`。
- [x] 1.5 从 `policy.go` 中移除 resolver、缓存 provider 和 subject helper 等混合职责，只保留 policy loader 相关代码。

## 2. 行为保持检查

- [x] 2.1 确认 `NewPolicyLoader`、`LoaderParams`、`PolicySet`、`PermissionRule`、`UserRoleResolver`、`UserRoleResolverParams`、`UserRoleResolverResult` 和 `NewUserRoleResolver` 的导出名称、签名与 Fx tag 保持不变。
- [x] 2.2 确认 policy loader 仍只加载启用角色、启用权限和角色权限绑定，并继续追加 `rbacbaseline.SuperAdminRoleID` 的 wildcard policy。
- [x] 2.3 确认 `loadRolesForUser` 仍只返回启用角色，仍排除已软删除用户，并保持按 `role_id` 排序。
- [x] 2.4 确认 `NewUserRoleResolver` 仍读取 `local_cache.rbac_user_roles`，仍以 `name:"rbac_user_roles_cache"` 暴露 `localcache.StatsSource`，并保留 `fx.Lifecycle` `OnStop` 关闭 hook。
- [x] 2.5 确认 `cloneRoleIDs`、用户角色缓存失效方法和并发 miss 合并行为没有被改写。

## 3. 格式化与验证

- [x] 3.1 对 `user-service/internal/features/permission/infrastructure/casbin` 变更 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-service` 模块运行 `go test ./internal/features/permission/infrastructure/casbin`，确认 permission casbin 包测试通过。
- [x] 3.3 在仓库根目录运行 `make user-service-architecture-lint`，确认架构 lint 通过。
- [x] 3.4 使用 `rg "type PolicySet|type UserRoleResolver|func NewUserRoleResolver|func loadRolesForUser|func roleSubject" user-service/internal/features/permission/infrastructure/casbin` 检查符号落点符合拆分目标。
- [x] 3.5 检查 `git diff`，确认代码改动只包含目标 casbin 包文件和本 change artifacts，没有 Casbin policy 内容、SQL/Ent predicate、RBAC baseline、route diff、OpenAPI、migration 或部署资产的无关变化。
- [x] 3.6 实现完成后将对应 tasks checkbox 更新为 `- [x]`，并确认 `openspec status --change split-casbin-policy-and-user-role-resolver` 状态正常。
