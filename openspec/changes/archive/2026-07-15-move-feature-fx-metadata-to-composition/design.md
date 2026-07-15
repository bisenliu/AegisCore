## Context

当前 user-service 的 feature 分层要求 application/domain 保持业务内核属性，composition 和基础设施层负责框架接线。auth feature 已将 Fx metadata 留在 feature `fx.go`，但 role 与 permission 的 command service 构造参数仍直接嵌入 `fx.In`，使 application command 包 import `go.uber.org/fx`。

这类依赖不改变业务行为，却让直接构造单元测试被 DI 框架形态影响，也让 architecture lint 难以阻止后续 application/domain 包继续引入框架 metadata。本变更只收敛 composition 边界，不调整 RBAC command/query 语义、port 所有权、HTTP DTO、数据库 schema、OpenAPI 或部署资产。

## Goals / Non-Goals

**Goals:**

- role/permission application command 生产包移除仅为 DI 使用的 Fx import、`fx.In` 和 Fx struct tag。
- application command 构造器接收强类型普通参数，并继续保持 `PolicyChangeNotifier` 等必需依赖的 fail-fast 语义。
- 无 named/optional 或配置转换需求的 application 构造器由 feature `fx.go` 直接注册；确有 Fx metadata 时由 composition adapter 承载并转换。
- application/domain 生产包的 Fx import、`fx.In` 和 Fx DI tag 被 architecture lint 禁止，并由违规 fixture 覆盖。
- role/permission 直接构造单元测试无需 Fx，feature module 测试仍验证正式 Fx graph 可构图和启动。

**Non-Goals:**

- 不改变权限、角色、角色权限、用户角色绑定、policy reload、用户角色缓存失效或跨副本 policy sync 的业务行为。
- 不改变 HTTP API、OpenAPI、transport DTO、application port 所有权、Ent schema、Atlas migration 或部署配置。
- 不重组 feature 目录，不引入 `fx.Private`，不为了减少参数数量创建无业务意义的大接口。
- 不禁止 infrastructure、transport 或 feature composition 在合理边界内使用 Fx metadata。

## Decisions

1. application command 使用强类型普通参数，feature `fx.go` 直接注册构造器。

   rationale：role/permission command 的依赖类型互不相同且全部必需，没有 named、optional 或配置转换需求。直接参数使构造签名成为唯一依赖清单，新增或删除依赖时所有调用方和 Fx graph 都会由编译或构图校验强制同步，同时保持 application 框架无关和现有端口粒度。

   alternatives considered：普通 application `Deps` 加 feature `fx.In` Params adapter 能隔离框架，但会为相同依赖维护两份字段清单，且本场景没有 Fx metadata 可承载；保留 `fx.In` 并只改 lint 不能解决应用层框架耦合。若未来出现同类型依赖、named/optional metadata 或配置转换，再在 composition 引入 Params adapter。

2. 不移动 port 定义、不合并接口。

   rationale：现有 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup` 和 `PolicyChangeNotifier` 由消费侧 application 拥有，符合边界规则；为了 DI 便利合并为大接口会削弱最小协作者契约。

   alternatives considered：创建 `RoleCommandDependencies` 大接口可以减少字段数量，但会把不同协作者调用折叠成一类依赖，降低测试 expectation 的精确性。

3. architecture lint 检查 application/domain 生产包中的 Fx 依赖形态。

   rationale：仅移除当前 import 无法防止回归；lint 应直接扫描 `user-service/internal/features/{user,auth,role,permission}/**/{application,domain}/` 生产 Go 文件中的 `go.uber.org/fx`、`fx.In` 和 Fx DI tag。

   alternatives considered：只依赖 code review 或 package import graph 不够稳定；只禁止 import 无法命中通过 tag 残留的 DI metadata。

4. spec delta 使用新增分层约束，不修改既有业务需求。

   rationale：本变更是 RBAC 能力的架构边界强化，不改变已有权限、角色、授权或同步结果；新增 requirement 能明确未来必须保持的结构约束，同时避免重写大段既有业务 requirement。

## Risks / Trade-offs

- [Risk] application 构造器签名变化造成测试或 Fx provider 调用未同步。Mitigation：直接注册构造器，使编译和 Fx module 构图测试共同命中遗漏调用方。
- [Risk] 未来引入 named/optional 依赖后直接注册不再足够。Mitigation：仅在出现真实 metadata 或配置转换需求时于 feature composition 增加局部 Params adapter。
- [Risk] lint 规则过宽误伤 infrastructure、transport 或 composition 中合理的 Fx 使用。Mitigation：lint 只限定 application/domain 生产包；测试文件和 feature 根 `fx.go` 不纳入禁止范围。
- [Risk] 只处理 command 包而遗漏其他 application/domain Fx import。Mitigation：实现时先全仓审计 user/auth/role/permission application/domain 生产包，再用 lint 和 `rg` 验证。

## Migration Plan

1. 将 role/permission command service 参数对象改为强类型普通参数，移除 application command 包 Fx import。
2. 在 role/permission feature `fx.go` 的 `fx.Provide` 中直接注册 application 构造器，不保留无 metadata 价值的重复 Params adapter。
3. 更新 role/permission command 单元测试，使其直接传入普通参数；更新或保留 Fx module 测试覆盖正式构图。
4. 更新 architecture lint 与违规 fixture，禁止 application/domain 生产包新增 Fx import、`fx.In` 或 Fx DI tag。
5. 更新 `docs/ARCHITECTURE.md` 的 feature 分层说明。
6. 运行相关 feature 测试、`make user-service-architecture-lint`、`openspec validate move-feature-fx-metadata-to-composition`，暂存预期变更后运行 `make lint` 和 `make verify`。

Rollback strategy：本变更不涉及数据库或外部契约；若构图或 lint 规则出现问题，可以回退代码和文档变更，恢复原构造器注册，不需要数据迁移或运行时回滚步骤。

## Open Questions

无。当前目标、范围和验收标准已明确。
