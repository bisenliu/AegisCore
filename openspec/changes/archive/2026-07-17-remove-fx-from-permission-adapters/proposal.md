## Why

permission 的 PostgreSQL、Redis 和 Casbin adapter 构造 API 当前混入 `fx.In`、named tag、`fx.As` 和 `fx.Self` 等 DI metadata，导致授权引擎、policy sync 与应用端口的装配依赖 Fx/Dig 语义，难以在普通 Go 代码和测试中显式复用同一个有状态实例。

本变更通过不兼容方式收敛 adapter 构造边界，使 permission infrastructure 可以用普通 Go 参数直接装配，同时把同一 `Engine`、Redis policy `Store` 和 `VersionTracker` 显式赋值给 concrete/interface 视图，避免重复构造造成状态分裂。

## What Changes

- **BREAKING**：移除 permission PostgreSQL、Redis、Casbin adapter constructor 中的 `fx.In` Params、named result/tag、`fx.As`、`fx.Self` 和 Dig 相关 metadata。
- **BREAKING**：生产 Fx composition 改为调用无 DI metadata 的 constructor，并通过普通 Go 赋值将同一有状态实例暴露为 application/authorization ports。
- **BREAKING**：adapter 测试改为显式构造依赖，不再依赖 Fx/Dig tag 或 wrapper 验证装配。
- 修改 `rbac-access-control` 规格，要求 permission adapter 构造 API 使用显式 concrete/interface 赋值表达端口暴露关系。
- 保持 Casbin policy、route diff、Redis key/version/PubSub、权限 API、fail-closed 授权语义不变。
- 不迁移 Casbin initial load、watcher `Start/Stop` 或用户角色缓存 `Close` 生命周期。
- 不删除 permission feature Fx module。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：补充 permission adapter 构造和端口暴露约束，要求以普通 Go 参数与显式赋值取代 Fx/Dig metadata，并禁止为 concrete/interface 视图重复构造同一类有状态实例。

## Impact

- 影响代码：`user-service/internal/features/permission/infrastructure/postgres`、`redis`、`casbin` 相关 adapter constructor 和测试。
- 影响装配：`user-service/internal/features/permission/fx.go` 仍保留 Fx module，但需要直接适配新签名，不通过 `fx.As`、`fx.Self`、named result 或 DI Params 暴露 permission ports。
- 影响规格：新增 `openspec/changes/remove-fx-from-permission-adapters/specs/rbac-access-control/spec.md` delta。
- 不影响 HTTP API、OpenAPI、数据库 schema、Casbin policy 内容、Redis key/version/PubSub 协议、route diff、部署资产和 fail-closed 行为。
- 验证需要覆盖 permission infrastructure 测试、OpenSpec 校验、架构 lint、仓库 lint 和 verify。
