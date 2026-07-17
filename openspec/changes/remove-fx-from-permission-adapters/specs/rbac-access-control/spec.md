## ADDED Requirements

### Requirement: Permission adapter 显式装配边界

permission 的 PostgreSQL、Redis 和 Casbin infrastructure adapter 构造 API MUST 使用普通 Go 参数表达必需依赖，并 MUST NOT 在 adapter constructor 中暴露或要求 Fx/Dig metadata。生产 Fx composition MAY 在 feature composition 边界选择具名资源和生命周期挂钩，但 MUST 通过显式 Go 赋值暴露 concrete 与 application/authorization port 视图。

#### Scenario: adapter constructor 不携带 DI metadata

- **WHEN** 构造 permission `PermissionStore`、policy `Loader`、Casbin `Engine` 或 Redis policy `Store`
- **THEN** constructor MUST 接收普通强类型参数或无 DI metadata 的 options
- **AND** constructor MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag、`fx.As`、`fx.Self`、named result 或 group result

#### Scenario: composition 显式选择服务资源

- **WHEN** 正式 permission Fx module 装配 PostgreSQL、Redis、policy loader、policy store、version tracker 或 authorization engine
- **THEN** 具名 `primary_db`、`cache_redis` 或生命周期依赖的选择 MUST 留在 `features/permission/fx.go` composition 边界
- **AND** PostgreSQL、Redis 和 Casbin adapter package 的生产构造 API MUST NOT import Fx 或 Dig 只为读取这些 tags

#### Scenario: 同一 Engine 暴露多个端口

- **WHEN** composition 需要同时提供 Casbin concrete `Engine`、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine`
- **THEN** composition MUST 构造一个 `Engine` 实例并通过普通 Go 赋值暴露这些端口
- **AND** 系统 MUST NOT 为 concrete、authorization port 或 reload port 重复构造有状态 `Engine`

#### Scenario: 同一 Redis Store 暴露发布端口

- **WHEN** composition 需要同时提供 Redis policy `Store` concrete 视图和 `permissionapplication.PolicyVersionPublisher` 等接口视图
- **THEN** composition MUST 构造一个 `Store` 实例并通过普通 Go 赋值暴露这些端口
- **AND** 系统 MUST NOT 为 concrete 和 interface 视图重复构造有状态 Redis policy store 或 version tracker

#### Scenario: 行为保持不变

- **WHEN** permission adapter 构造 API 从 Fx/Dig metadata 改为普通 Go 参数
- **THEN** 权限目录、route diff、Casbin policy、授权 fail-closed、Redis policy version、Pub/Sub、用户角色缓存失效和多副本同步语义 MUST 保持不变
- **AND** 本变更 MUST NOT 迁移 Casbin initial load、watcher `Start/Stop` 或用户角色缓存 `Close` 生命周期
