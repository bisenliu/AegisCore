## Context

RBAC runtime 组件目前在 permission feature 的 Fx composition 中以 named/private provider 组装，再通过多个 public projection provider 单独暴露给上层模块或相邻 feature。`Authorizer`、`PolicyHealth`、`PolicyWatcherStatus`、`PolicyChangeNotifier`、policy initializer、watcher runner 和 user-role resolver lifecycle 本质上属于同一组 permission runtime，但当前依赖图没有显式表达这一点。

本变更只收敛 user-service 内部装配方式，不改变 RBAC 授权语义、policy reload/sync 行为、health/status 输出、HTTP API、OpenAPI、数据库 schema、部署清单或 `common` 共享能力。

## Goals / Non-Goals

**Goals:**

- 用 `PermissionRuntime` 表达 permission feature 对外稳定运行时依赖集合，减少 named/private 到 public 的逐项转发样板。
- 保持有状态 RBAC 资源单实例多视图：Casbin engine、policy store、watcher、version tracker、cache 和 resolver 不因聚合对象重复构造。
- 保持现有父模块和 role feature 的稳定依赖可解析，并让迁移后的依赖关系更集中、更容易测试。
- 更新 Fx 测试，确认 runtime 聚合对象、授权、健康检查、watcher 状态、notifier 和 lifecycle hook 仍按预期组装。

**Non-Goals:**

- 不修改权限、角色、用户角色或有效权限的业务规则。
- 不修改 Casbin model、policy loader、Redis policy version、Pub/Sub payload、watcher 补偿逻辑或 fail-closed 语义。
- 不新增 `common`、`internal/shared` 或 `internal/integration` 代码。
- 不修改 HTTP route、DTO、响应 envelope、OpenAPI 生成物、Ent schema、Atlas migration 或部署/观测资产。

## Decisions

### Decision 1: 在 permission composition 边界引入 `PermissionRuntime`

`PermissionRuntime` 放在 `user-service/internal/features/permission` 的 Fx 组装边界，字段使用既有 application/authorization 接口和 composition 私有 lifecycle 接口。它不是 domain/application 概念，不下沉到 `common` 或 `internal/shared`。

备选方案是保留所有单独 public projection provider。该方案行为风险最低，但继续扩散样板并让同一组 runtime 组件的聚合关系隐含在多个函数和 named tag 中。

### Decision 2: 聚合对象只收敛投影，不改变实例所有权

`newPermissionRuntime` 接收已经构造好的 authorizer、health、watcher、notifier、initializer 和 user-role resolver lifecycle，并用普通 Go 赋值暴露字段。watcher 同时作为 `WatcherStatus` 和 watcher runner 时必须复用同一实例，避免为了接口视图重复构造有状态组件。

备选方案是在 `PermissionRuntime` 内部构造这些组件。该方案会把 provider 参数、named resource 选择和错误处理塞进聚合构造函数，模糊 composition 与 constructor 边界，也更容易违反现有 framework-neutral adapter 约束。

### Decision 3: 消费方优先依赖聚合对象或从聚合对象解包的稳定 contract

permission 内部 lifecycle hook 可以直接使用 `PermissionRuntime` 获取 initializer、watcher 和 user-role resolver lifecycle。父级 routes/health 和 role feature 可以根据现有模块边界选择继续消费稳定接口，或改为消费 `*PermissionRuntime` 后读取字段；无论哪种方式，不能导入 permission infrastructure concrete implementation。

备选方案是一次性把所有消费方都改为 `*PermissionRuntime`。该方案最彻底，但会把只需要单一 contract 的调用方绑定到更大的聚合类型，可能降低依赖最小化程度。

## Risks / Trade-offs

- [Risk] 聚合对象字段漏接或 provider 顺序调整导致 Fx graph 缺少依赖。→ Mitigation: 更新 `fx_test.go` 覆盖 `*PermissionRuntime` 和原有 public contract 的解析，并运行相关 package 测试。
- [Risk] watcher 同一实例多视图在迁移中被误拆成多个实例。→ Mitigation: `newPermissionRuntime` 只接收既有 watcher 实例并将其同时赋给 `WatcherStatus` 和 `Watcher` 字段，测试验证状态视图可用。
- [Risk] 消费方依赖聚合对象后扩大跨 feature 耦合。→ Mitigation: 只在 composition/provider 边界使用 `PermissionRuntime`；application/domain 仍只依赖消费侧最小 port。
- [Trade-off] 保留少量解包 provider 可兼容父模块现有稳定接口，但无法完全消除所有 provider 函数。→ Mitigation: 只保留对外 contract 必需的解包，删除纯 named/private 转发样板。

## Migration Plan

- 修改 permission Fx provider，新增 `PermissionRuntime` 和构造函数，移除或收敛重复 public projection provider。
- 调整 permission lifecycle、父级 routes/health 和 role feature 的 Fx 参数，使其通过聚合对象或保留的稳定解包 contract 获取依赖。
- 更新 `user-service/internal/features/permission/fx_test.go` 覆盖新的依赖图。
- 运行相关 Go 测试；因不涉及 API/schema/deploy，不需要 OpenAPI 生成、Ent 生成或 Atlas migration。
- 回滚方式是恢复原有逐项 public projection provider 和消费方参数，因不涉及持久化数据或外部契约，无需数据迁移。

## Open Questions

- 无。
