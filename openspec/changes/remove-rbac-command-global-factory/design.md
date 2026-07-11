## Context

`user-service/cmd/rbac.go` 同时承载 RBAC seed、超级管理员绑定和超级管理员创建命令的依赖装配。当前测试通过替换 package-level 可变工厂注入替身，这让多个测试并行执行时共享同一全局可变状态，存在串扰和 race 风险。

本次改动只位于 `user-service` CLI 边界和 OpenSpec change 文档，不改变 `common`、feature application、Ent schema、HTTP API、OpenAPI、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 将 RBAC CLI 依赖工厂从 package-level 可变变量改为显式参数。
- 让 `runRBACSeedCommand`、`runAssignSuperAdminCommand`、`runCreateSuperAdminCommand` 在调用时接收局部依赖工厂。
- 让测试 helper 返回局部 runner 或局部依赖注入点，使相关测试可独立、并行运行。
- 保持现有命令业务行为、输出文本、超时、配置加载、数据库连接和 Ent client 清理顺序不变。

**Non-Goals:**

- 不新增 RBAC CLI 子命令或配置项。
- 不改变 RBAC seed、角色权限基线、超级管理员绑定或超级管理员创建逻辑。
- 不调整 HTTP RBAC 授权、policy sync、Casbin policy loader 或本地缓存行为。
- 不引入新的共享接口、外部依赖、数据库 migration 或 OpenAPI 生成物。

## Decisions

1. 使用显式 factory 参数贯穿命令 runner。

   `newRBACCommand` 或其内部子命令构造函数在生产路径中传入真实依赖工厂；测试路径构造局部 runner 时传入替身工厂。备选方案是继续保留 package-level 变量并加锁保护，但该方案仍会让测试和生产入口依赖隐藏全局状态，且锁只能降低 race 风险，不能解决串扰和可读性问题。

2. 保留既有依赖生命周期。

   真实依赖工厂仍负责配置加载、数据库资源创建、Ent client 组装和 cleanup 链接；runner 仍按现有顺序调用 cleanup。备选方案是把依赖拆成多个细粒度 provider，但本次问题只在全局替身，不需要扩大重构范围。

3. 测试只替换局部依赖，不增加生产专用测试分支。

   测试 helper 改为返回绑定替身 factory 的局部 runner 或命令对象。备选方案是为测试新增额外全局开关或 build tag，但会引入与业务无关的额外路径，不符合最小改动。

## Risks / Trade-offs

- [Risk] 函数签名调整可能遗漏生产调用点或测试 helper。→ Mitigation：通过 `go test -race ./cmd -run 'TestRBAC|TestCreateSuperAdmin|TestAssignSuperAdmin|TestNormalize|TestChainCleanup'` 覆盖 RBAC command 相关路径。
- [Risk] 依赖 factory 显式传递后 cleanup 顺序被意外改变。→ Mitigation：保留既有 cleanup 链构造和 `TestChainCleanup` 覆盖。
- [Risk] 规格新增的是 CLI 内部装配约束，粒度小于业务接口规格。→ Mitigation：仅在 `rbac-access-control` 中新增与 RBAC CLI 引导稳定性相关的要求，不拆分新 capability。

## Migration Plan

该变更无需数据库、配置、OpenAPI 或部署迁移。发布时随普通 user-service CLI 代码发布即可；如需回滚，回滚本 change 的代码和 OpenSpec delta 即可恢复原有实现。

## Open Questions

无。
