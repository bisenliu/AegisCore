## Why

`user-service/internal/features/permission/infrastructure/casbin/policy.go` 同时承载 policy loader、用户角色 resolver、Fx 输入输出和本地缓存构造，文件职责已经超过单一 policy 加载语义。

将 loader、resolver 和 provider 组装拆到职责清晰的小文件，可以降低 RBAC 授权热路径维护成本，并让后续审查 policy SQL、用户角色缓存和 Fx provider 行为时边界更明确。

## What Changes

- 将 `policy.go` 保留为 policy loader 专属文件，只承载 policy 数据结构、`Loader`、`entLoader` 和 policy 加载逻辑。
- 将 `UserRoleResolver`、`entUserRoleResolver`、`loadRolesForUser` 和 `cloneRoleIDs` 迁移到 `user_role_resolver.go`。
- 将 `UserRoleResolverParams`、`UserRoleResolverResult` 和 `NewUserRoleResolver` 迁移到 `fx.go` 或 `user_role_cache.go`，使 Fx provider 与缓存构造职责独立。
- 保持 `package casbin`、导出 API、Fx provider 行为、localcache 配置名、统计源 name tag、lifecycle hook 和缓存 clone/singleflight 行为不变。
- 保持 Casbin policy 内容、policy 加载 SQL/Ent predicate、用户角色查询条件、RBAC baseline、route diff 逻辑和 permission feature 外部接口不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 整理 permission Casbin infrastructure 的 policy loader、用户角色 resolver 和 Fx provider 文件归属，保持 RBAC 授权、策略加载、用户角色解析缓存和策略同步需求语义不变。

## Impact

- 受影响代码：
  - `user-service/internal/features/permission/infrastructure/casbin/policy.go`
  - `user-service/internal/features/permission/infrastructure/casbin/user_role_resolver.go`
  - `user-service/internal/features/permission/infrastructure/casbin/fx.go` 或 `user_role_cache.go`
- 相关测试范围：`user-service/internal/features/permission/infrastructure/casbin` 包测试，重点覆盖 policy loader、超级管理员 wildcard、用户角色缓存命中、失效和并发 miss 合并。
- 相关架构验证：`make user-service-architecture-lint`。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署资产、观测指标名称、安全契约、共享契约或外部 RBAC 行为。
