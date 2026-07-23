## Context

permission feature 当前在 Fx composition 层组装 RBAC policy engine、watcher 和 user-role resolver/cache。`registerRBACLifecycle` 需要在启动时先启动 user-role resolver/cache，再执行 policy 初始化并启动 watcher；停止时需要停止 watcher 并关闭 resolver/cache。

现状中 lifecycle hook 注入的是 `UserRoleCacheCloser`，启动阶段通过 type assertion 探测该对象是否还有 `Start(context.Context) error`。这让“可启动资源”的契约隐藏在“可关闭资源”中，导致装配参数语义不准确，测试也无法直接表达缺失启动能力应视为装配契约问题。

本变更只调整 user-service permission feature 内部 Go 装配契约，不改变 HTTP API、OpenAPI、数据库 schema、migration、部署清单、观测资产或 `common` 共享契约。

## Goals / Non-Goals

**Goals:**

- 在 permission composition 层定义显式 `userRoleResolverLifecycle` 接口，包含 `Start(context.Context) error` 和 `Close() error`。
- 让 `UserRoleResolverResult` 显式输出 lifecycle 视图，避免 lifecycle hook 从 resolver 或 closer 中做隐式探测。
- 让 `RegisterRBACLifecycleParams` 直接依赖显式 lifecycle，并在 `OnStart` 中先调用 `UserRoles.Start(ctx)`。
- 保持停止路径继续调用 `stopRBACLifecycle(ctx, params.Watcher.Stop, params.UserRoles)`，确保 watcher stop 和 resolver close 的错误聚合语义不退化。
- 更新 `fx_test.go` 覆盖显式 lifecycle、启动失败短路和停止清理行为。

**Non-Goals:**

- 不改变 user-role resolver/cache 的缓存算法、key、stats 或 fail-closed 授权语义。
- 不改变 policy loader、watcher、Redis policy sync 或 Casbin engine 的业务逻辑。
- 不新增 `common` helper、共享接口或跨 feature abstraction。
- 不修改公开 API、OpenAPI 文档、数据库结构、migration 或部署配置。

## Decisions

- 在 `permission` composition 边界定义未导出的 `userRoleResolverLifecycle`。

  选择原因：该接口只服务于 permission feature 的 Fx 生命周期装配，属于服务内 composition 细节。未导出接口可以避免把内部 lifecycle 契约扩散到 application、domain、infrastructure 或 `common`。

  备选方案：复用现有 `UserRoleCacheCloser` 并保留 type assertion。该方案改动最小，但继续隐藏启动能力，不能解决语义问题。

- `UserRoleResolverResult` 同时输出 `Resolver`、`Stats` 和 `Lifecycle`。

  选择原因：resolver、stats 和 lifecycle 是同一底层有状态对象的不同视图。composition 层通过普通 Go 赋值暴露多个端口，符合“有状态资源单实例多视图”的既有 RBAC 规格。

  备选方案：让 lifecycle hook 直接依赖 resolver concrete implementation。该方案会把具体实现泄露到 lifecycle 装配参数，降低替换和测试弹性。

- `registerRBACLifecycle` 启动时直接调用 `params.UserRoles.Start(ctx)`，失败时返回错误并不初始化 engine 或启动 watcher。

  选择原因：user-role resolver/cache 是授权热路径依赖，启动失败应阻止后续启动步骤，避免 watcher 在依赖未就绪时运行。

  备选方案：启动失败后继续初始化 policy 并让授权 fail-closed。该方案会让服务处于更复杂的部分启动状态，不利于定位 lifecycle 错误。

- 保持 `stopRBACLifecycle` 作为停止错误聚合入口。

  选择原因：既有规格要求 watcher stop 和 cache close 同时失败时保留全部 cause，且 cache close 必须在 watcher stop 失败时仍执行。本变更只替换传入对象的接口类型，不改变停止语义。

## Risks / Trade-offs

- [Risk] Fx named/type 绑定调整可能导致 lifecycle 依赖未提供而装配失败。→ Mitigation：更新 `UserRoleResolverResult` 和 Fx 测试，确保 resolver provider 同时提供 lifecycle 视图。
- [Risk] 启动失败路径变为直接返回错误，可能暴露之前被忽略的 resolver 启动错误。→ Mitigation：这是预期的 fail-fast 行为；测试覆盖 `UserRoles.Start` 返回错误时 engine 和 watcher 不启动。
- [Risk] 新接口与旧 closer 名称并存可能造成阅读混淆。→ Mitigation：将 lifecycle hook 参数改为显式 `UserRoles userRoleResolverLifecycle`，并只在需要关闭语义的内部 helper 保留 closer 角色。

## Migration Plan

- 代码迁移：先调整 `fx_authorization.go` 的 result 输出，再调整 `fx_lifecycle.go` 的参数和启动逻辑，最后同步 `fx_test.go`。
- 回滚策略：若出现装配回归，可回退本次 permission feature 内部接口变更；无需回滚数据库、OpenAPI 或部署资产。
- 验证方式：运行相关 permission feature 测试；规格或文档变更后运行 `make user-service-architecture-lint`。

## Open Questions

- 无。
