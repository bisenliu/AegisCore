## Why

当前 user-service 的 `/api/v1` 路由通过三类 Fx value group 和 feature-local route registrar 间接注入。该方式保留了认证、授权分层，但对当前固定的 auth、permission、role 和 user 核心 feature 来说样板较多，排查路由图需要在 feature `fx.go`、`route_registrar.go`、transport routes 和 router composition 之间多次跳转。

本变更希望在不改变 HTTP API、认证、token version 或 RBAC 授权语义的前提下，改为由 user-service composition root 集中注册固定 feature 路由，使路由图更直接、更容易验证和维护。

## What Changes

- 将 `/api/v1` 固定 feature 路由改为集中注册：auth public、auth authenticated、permission、role 和 user route 由统一 route composition 明确挂载。
- 保留现有三层访问边界：public route 不经过普通 access token，authenticated route 经过 token version validator，authorized route 先认证再经过 RBAC authorizer。
- 移除或停止使用仅用于固定 feature 路由转发的 feature-local route registrar 和三类路由 Fx value group。
- 保持现有 HTTP path、method、OpenAPI 注解、controller 行为、RBAC permission baseline 和 route graph 测试语义不变。
- 不引入插件式 feature route discovery，也不增加运行时配置开关。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 调整 user-service HTTP route graph 的装配要求，从通过 route registrar contract 接入固定 feature 路由，改为由 composition root 显式集中挂载固定 feature 路由，同时保持访问层级和可验证性要求不变。

## Impact

- 影响代码：`user-service/internal/router/`、`user-service/internal/providers/`、`user-service/internal/features/*/fx.go`、可能删除或停用 `user-service/internal/features/*/route_registrar.go`。
- 影响测试：更新 route graph、provider graph 和 architecture/lint 相关测试，使其校验集中注册后的访问层级和必需依赖。
- API 影响：不改变公开 HTTP path、method、请求响应结构、OpenAPI 文档内容或错误契约。
- 安全影响：必须保持受保护 auth route 认证校验、permission/role/user 业务路由 RBAC 授权、缺失 token version validator 或 authorizer 时拒绝注册的 fail-closed 行为。
- 数据和部署影响：不改变 Ent schema、Atlas migration、Redis key、Casbin policy 数据、Docker/Compose/Kubernetes/Helm 资产。
