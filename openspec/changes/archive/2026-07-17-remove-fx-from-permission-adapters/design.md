## Context

permission feature 当前已经把部分 application service 调整为普通强类型参数，但 PostgreSQL、Redis、Casbin infrastructure adapter 和 `fx.go` 装配仍保留 Fx/Dig metadata。典型表现包括 constructor 入参嵌入 `fx.In`、资源通过 `name:"primary_db"` 或 `name:"cache_redis"` 注入、`fx.As`/`fx.Self` 同时暴露 concrete 与 interface，以及 cache/watcher 生命周期仍在 adapter constructor 内直接接收 `fx.Lifecycle`。

本变更聚焦 permission adapter 构造 API 的显式化，目标是在不改变 RBAC 行为的前提下，让授权引擎、policy loader、Redis policy store 和版本跟踪器能通过普通 Go 参数装配，并在 Fx composition 中由显式 provider 函数完成资源选择、生命周期挂钩和 interface 赋值。

## Goals / Non-Goals

**Goals:**

- 移除 `PermissionStore`、policy `Loader`、Casbin `Engine`、Redis policy `Store` 构造路径中的 `fx.In` Params、named tags、`fx.As`、`fx.Self` 和 Dig metadata。
- 在生产 `permission/fx.go` 中用普通 provider 函数适配具名 PostgreSQL/Redis 资源，并显式返回 concrete 与 application/authorization ports。
- 保证同一个 Casbin `Engine` 同时作为 concrete、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine` 使用。
- 保证同一个 Redis policy `Store` 同时作为 concrete、`PolicyVersionPublisher` 或其他所需接口视图使用，`VersionTracker` 也通过同一实例显式暴露。
- 更新 adapter 测试和 Fx composition 测试，证明接口视图没有重复构造有状态实例。

**Non-Goals:**

- 不迁移 Casbin initial load 的生命周期语义，`RegisterInitialLoad` 可继续由 composition 通过 Fx lifecycle 调用。
- 不迁移 Redis watcher `Start/Stop` 生命周期或用户角色缓存 `Close` 生命周期。
- 不删除 permission feature Fx module，也不要求整个服务脱离 Fx。
- 不改变 Casbin policy、route diff、Redis key/version/PubSub、权限 HTTP API、OpenAPI、数据库 schema、部署资产或 fail-closed 授权行为。

## Decisions

1. adapter constructor 使用普通参数，资源选择留在 composition。

   `postgres.NewPermissionStore` 接收 `*ent.Client`；policy loader 接收 `*ent.Client`、用户解析依赖和 logger/metrics 等原有业务依赖；Redis store 接收 `*rediscmd.Client` 和无 DI metadata 的 options。具名资源映射只在 `permission/fx.go` 的 provider 入参中出现。

   备选方案是继续让 adapter 暴露 `fx.In` Params，但这会把 DI 框架语义留在 infrastructure API，无法满足普通 Go 装配目标。

2. concrete/interface 暴露用显式 Go 返回值或赋值，不使用 `fx.As`/`fx.Self`。

   `permission/fx.go` 可以定义局部 result struct 或 provider 函数返回多个强类型值；provider 内先构造一个 concrete，再将其赋给 interface 变量返回。这样可以用指针相等测试证明 authorization/reload port 指向同一 `Engine`，publisher concrete/interface 视图指向同一 `Store`。

   备选方案是保留 `fx.Annotate(..., fx.As(...))`，但它隐藏了复用关系，测试也更容易只验证 graph 能启动而不是验证同一实例。

3. 生命周期迁移暂不扩大范围。

   本 change 只移除目标 adapter 构造 API 中的 DI metadata。`RegisterInitialLoad`、watcher `Start/Stop` 和用户角色缓存 `Close` 的生命周期位置如果已存在 Fx 依赖，可以继续留在后续生命周期专项中处理。

   备选方案是同时迁移所有 Fx lifecycle，但会扩大改动面，并与本次明确的不做事项冲突。

4. 规格只增加 RBAC 分层与组合边界的要求，不新增 capability。

   该需求属于 `rbac-access-control` 的架构约束演进，不改变业务功能、HTTP API 或数据模型，因此通过 delta 增加 requirement 最小化主规格变更。

## Risks / Trade-offs

- [Risk] provider 函数返回多个接口时可能误构造多次 `Engine` 或 `Store`，导致 reload、授权、发布视图状态分裂。→ Mitigation：在测试中断言 concrete/interface 指向同一实例，并在实现中先赋 concrete 再赋接口变量。
- [Risk] 移除 named tags 后可能拿错 PostgreSQL 或 Redis 资源。→ Mitigation：仅在 `permission/fx.go` 的 composition provider 入参保留服务级具名资源选择，adapter constructor 不感知 tag。
- [Risk] 过度清理 Fx import 可能误触 watcher/cache 生命周期。→ Mitigation：按范围排除后续生命周期文件，保留必要 lifecycle 挂钩并只移动调用点。
- [Risk] 不兼容 constructor 变更会破坏测试或调用点。→ Mitigation：同步更新所有生产调用点和 adapter 测试，不保留旧 wrapper，避免双路径长期共存。

## Migration Plan

1. 修改 permission PostgreSQL、Casbin、Redis adapter constructor 签名，移除 `fx.In` Params、named tags 和 adapter 内 DI metadata。
2. 在 `permission/fx.go` 中添加显式 provider，负责读取具名 `primary_db`、`cache_redis`、配置和 logger，并返回 concrete/interface 视图。
3. 更新 adapter 与 feature Fx 测试，覆盖同一实例复用、普通参数构造和 graph 装配。
4. 运行 `cd user-service && go test ./internal/features/permission/infrastructure/... -count=1`、`openspec validate remove-fx-from-permission-adapters`、`make user-service-architecture-lint`。
5. 暂存预期变更后运行 `make lint` 和 `make verify`。

回滚方式：该变更尚未改变持久化数据、API 或部署资产；如实现阶段出现无法接受的装配风险，可回滚本 change 的代码和 spec delta，恢复原 Fx/Dig constructor API。

## Open Questions

- 无。
