## Context

`user-service/internal/features/permission/infrastructure/casbin/policy.go` 当前同时包含四类职责：Casbin policy 数据模型和加载端口、Ent-backed policy loader、用户角色 resolver 与 Ent 查询、以及 `NewUserRoleResolver` 的 Fx 输入输出和 localcache 构造。

这些职责都属于 permission feature 的 Casbin infrastructure 包，但它们的变更原因不同：policy loader 关注角色权限绑定和超级管理员 wildcard，用户角色 resolver 关注授权热路径的用户角色查询和缓存 clone/失效，Fx provider 关注配置读取、生命周期 hook 和 `StatsSource` 暴露。把它们留在同一文件会让后续审查 RBAC 授权行为时难以快速定位影响范围。

本 change 只做同包文件重组，保持 `package casbin` 不变，不新增子包，不移动到 `common`、`internal/shared` 或 `internal/integration`，也不改变 HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全契约。

## Goals / Non-Goals

**Goals:**

- 将 `policy.go` 保留为 policy loader 专属文件，承载 policy 数据结构、`Loader`、`LoaderParams`、`NewPolicyLoader`、`entLoader` 和 policy 加载逻辑。
- 将 `UserRoleResolver`、`entUserRoleResolver`、`RolesForUser`、失效方法、`loadRolesForUser` 和 `cloneRoleIDs` 迁移到 `user_role_resolver.go`。
- 将 `rbacUserRolesCacheName`、`UserRoleResolverParams`、`UserRoleResolverResult` 和 `NewUserRoleResolver` 迁移到 `user_role_cache.go`。
- 将 `roleSubject` 放到独立的小型 subject helper 文件，避免 `policy.go` 因单个 helper 继续承担混合职责。
- 保持导出符号、未导出类型名、Fx name tag、localcache 参数映射、lifecycle hook、policy wildcard、Ent 查询条件和测试可见行为不变。

**Non-Goals:**

- 不改变 Casbin model、policy 内容、角色 subject 格式或超级管理员 wildcard 语义。
- 不改变 policy loader 的 SQL/Ent predicate、排序、错误包装或 edge 加载方式。
- 不改变用户角色缓存的配置名、TTL/capacity 映射、clone 函数、singleflight 行为、失效方法或 `StatsSource` 暴露。
- 不改变 permission、role、user 或 auth feature 的外部接口。
- 不调整 RBAC baseline、route diff、policy sync、Redis policy version 或 Pub/Sub 逻辑。

## Decisions

### Decision: 保持同包拆分，不新增子包

所有新文件继续使用 `package casbin`。这样可以保留现有导出 API、同包测试白盒覆盖、`entUserRoleResolver` 未导出结构和 helper 函数的可见性，不引入 import cycle 或跨包装配层。

备选方案是新增 `loader`、`resolver` 或 `cache` 子包，但这些代码仍紧密服务 Casbin engine 构造和授权热路径。拆出子包会迫使未导出 helper 变成 public API，或新增 adapter 层，收益不足。

### Decision: 使用 `user_role_cache.go` 承载 resolver provider

`NewUserRoleResolver` 既是 Fx provider，也是 localcache 构造点，职责不同于纯粹的 resolver 查询实现。将其放入 `user_role_cache.go` 可以让 `user_role_resolver.go` 专注 resolver 端口、方法和回源查询，也避免在当前没有包级 `fx.go` 的目录里新增一个过宽的装配文件。

备选方案是创建 `fx.go` 并放入所有 Fx 输入输出。该方案在未来可能变成多个 provider 的集中入口，但本次只有 resolver cache provider 需要迁移，命名为 cache 更能表达当前职责。

### Decision: `policy.go` 保留 loader Fx 输入与构造函数

`LoaderParams` 和 `NewPolicyLoader` 与 `entLoader` 的生命周期、依赖和实现一一对应，放在 `policy.go` 可以让 policy 构造路径从 Fx 输入到 Ent 查询完整聚合在一个文件中，同时延续包内 policy 文件命名惯例。

备选方案是把 `LoaderParams` 移入 `fx.go` 或 `policy_provider.go`。该方案会把 loader 的构造入口和实现拆开，阅读 policy 加载路径时需要跨更多文件跳转。

### Decision: 保留行为测试，不新增仅覆盖文件位置的测试

现有 `policy_test.go` 已覆盖 active role/permission 过滤、超级管理员 wildcard、角色 subject、用户角色缓存命中、失效和并发 miss 合并。本次重组不改变稳定行为，因此不新增只断言文件拆分的测试。

备选方案是新增结构性测试或 lint 规则检查符号所在文件，但 Go 测试不适合维护文件布局约束；本次由 code review、tasks 检查和架构 lint 保障。

## Risks / Trade-offs

- [Risk] 拆分 imports 时遗漏 `fmt`、`fx`、`config`、`localcache` 或 Ent predicate 依赖，导致编译失败。-> Mitigation：拆分后运行 `gofmt` 和 `go test ./internal/features/permission/infrastructure/casbin`。
- [Risk] 移动 `NewUserRoleResolver` 时误改 `name:"rbac_user_roles_cache"` 或 `local_cache.rbac_user_roles` 配置读取，影响 metrics/provider 注入。-> Mitigation：原样迁移 provider 代码，并保留 `TestNewUserRoleResolverRequiresConfigInstance` 覆盖缺配置错误。
- [Risk] 移动 `cloneRoleIDs` 或 loader 回调时影响缓存返回值隔离或并发 miss 合并。-> Mitigation：保留 `cloneRoleIDs` 函数签名和 `localcache.New` 参数顺序，运行缓存命中、失效和并发测试。
- [Risk] 移动 `roleSubject` 或 `policyWildcard` 时影响 enforcer policy 添加或超级管理员 wildcard 测试。-> Mitigation：保留常量值和 subject 格式，运行 policy loader 与 enforcer 相关测试。
- [Risk] `policy.go` 删除或缩减后遗漏同包引用。-> Mitigation：使用 `rg` 检查 `PolicySet`、`LoaderParams`、`UserRoleResolverParams`、`roleSubject` 等符号引用，再运行包测试。

## Migration Plan

1. 保留 `policy.go`，只承载 policy 数据结构、loader 端口、loader Fx 输入、`entLoader`、`NewPolicyLoader` 和 `loadPermissionRules`。
2. 新增 `user_role_resolver.go`，迁入用户角色 resolver 端口、实现、失效方法、`loadRolesForUser` 和 `cloneRoleIDs`。
3. 新增 `user_role_cache.go`，迁入 `rbacUserRolesCacheName`、resolver Fx 输入输出和 `NewUserRoleResolver`。
4. 新增 subject helper 文件或等价小文件，承载 `roleSubject`；将 `policyWildcard` 留在 `policy.go`。
5. 从 `policy.go` 移除 resolver、缓存 provider 和 subject helper 等非 loader 职责。
6. 运行 `gofmt` 于 Casbin infrastructure 包变更文件。
7. 在 `user-service` 模块运行 `go test ./internal/features/permission/infrastructure/casbin`。
8. 在仓库根目录运行 `make user-service-architecture-lint`。

回滚方式：把迁出的符号恢复到 `policy.go` 并删除新增拆分文件。该 change 不涉及数据、配置、API、OpenAPI、部署或发布顺序迁移。

## Open Questions

无。
