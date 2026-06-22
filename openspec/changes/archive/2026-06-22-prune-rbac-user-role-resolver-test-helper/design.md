## Context

`user-service/internal/features/permission/infrastructure/casbin/policy.go` 当前同时包含生产用 `NewUserRoleResolver` 和一个仅被同包测试调用的 `newUserRoleResolver`。生产构造路径已经在 `NewUserRoleResolver` 中直接返回 `&entUserRoleResolver{cache: cache}`，测试 helper 再通过 `newUserRoleResolver(cache)` 间接构造相同结构。

本次变更归属 `rbac-access-control` capability，但不改变 RBAC 用户角色绑定、Casbin 授权保护或策略同步的稳定行为。改动只发生在 permission feature 的 Casbin infrastructure 包和同包测试中，不涉及 `common`、`internal/shared`、`internal/integration`、数据库 migration、OpenAPI 生成物、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 删除 `policy.go` 中仅被测试使用的 `newUserRoleResolver`。
- 让 `policy_test.go` 的测试 helper 直接构造 `&entUserRoleResolver{cache: cache}`。
- 保持用户角色缓存的命中、失效、并发 miss 合并和 Ent 查询行为不变。
- 运行 permission casbin 包测试验证行为不回退。

**Non-Goals:**

- 不修改 `NewUserRoleResolver` 的 Fx 构造逻辑或对外 provider 形态。
- 不调整 `entUserRoleResolver` 的字段、缓存配置、失效策略或查询 SQL/Ent predicate。
- 不拆分 `policy.go` 文件结构。
- 不新增、修改或删除 OpenAPI、数据库 schema、部署或观测配置。

## Decisions

### Decision: 测试直接构造未导出的 resolver 结构

选择在 `policy_test.go` 的 `newTestUserRoleResolver` 中返回 `&entUserRoleResolver{cache: cache}`，因为测试与生产代码同属 `casbin` 包，可以访问未导出的结构和字段。这让生产文件只保留真实运行路径需要的构造函数。

备选方案是保留 `newUserRoleResolver` 并标注测试用途，但这会继续把测试便利函数留在生产文件中。另一个备选方案是新增 test-only helper 文件，不过当前只需要一处构造表达式，新增文件会让改动比问题本身更重。

### Decision: 不修改 Fx provider 和缓存实现

`NewUserRoleResolver` 仍负责读取配置、创建 `localcache.Cache`、注册 lifecycle `OnStop` 并返回 `UserRoleResolverResult`。本次只删除冗余 helper，不触碰生产 resolver 生命周期和统计源导出。

备选方案是让 `NewUserRoleResolver` 也调用某个共享构造函数，但这会重新引入本次要移除的间接层，且没有新的复用价值。

### Decision: 使用现有包级测试作为行为保护

现有测试已经覆盖 active role 过滤、缓存命中、单用户失效后重载，以及并发 miss 合并。实现后运行 `go test ./internal/features/permission/infrastructure/casbin`，作为本次变更的主要验证。

备选方案是运行整个 `make verify`，但本次是两处同包实现级清理，permission casbin 包测试能更快反馈目标行为；合并前仍可按常规流程运行更广验证。

## Risks / Trade-offs

- [Risk] 直接构造结构可能让测试和实现字段更紧密。-> Mitigation：测试本来就是同包白盒测试，目标正是覆盖缓存字段注入；若未来字段变化，同包测试应同步更新。
- [Risk] 删除 helper 后遗漏唯一调用点会导致编译失败。-> Mitigation：使用 `rg newUserRoleResolver` 确认引用，并运行 casbin 包测试。
- [Risk] 误改 `NewUserRoleResolver` 可能影响 Fx 装配或缓存统计。-> Mitigation：实现时只删除 helper 和测试返回表达式，不改 provider 签名、返回类型和 lifecycle 代码。

## Migration Plan

实施步骤是先删除 `policy.go` 的 `newUserRoleResolver`，再把 `policy_test.go` 的测试 helper 返回值改为直接构造 `entUserRoleResolver`，最后运行 permission casbin 包测试。该变更没有数据库、配置、API 或发布顺序要求；如需回滚，恢复 helper 并让测试 helper 再次调用它即可。

## Open Questions

无。
