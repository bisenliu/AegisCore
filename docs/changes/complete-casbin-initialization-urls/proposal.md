## Why

当前 Casbin/RBAC 初始化基线只包含用户资料接口的系统权限 URL，但实际已经接入 JWT + RBAC 授权中间件的业务接口还包括权限目录、角色管理、用户角色绑定和角色权限绑定。`rbac seed` 依赖 `permission/application/rbacbaseline` 写入系统权限；当 catalog 缺少已受保护路由时，seed 后执行 route diff 会出现 `missing_in_permissions`，非超级管理员角色也无法通过系统初始化流程获得这些接口授权。

需要基于当前项目已有 HTTP 路由补齐 Casbin 初始化相关 URL 配置，保持 URL 命名、Gin route template、HTTP method 和项目现有 REST 风格一致，并让 seed、route diff、Casbin policy loader 和多实例 policy refresh 流程继续兼容。

## What Changes

- 完善 `user-service/internal/features/permission/application/rbacbaseline` 中的系统权限 URL catalog。
- 覆盖当前所有已由 `permissionhttp.Authorize` 保护的业务路由：
  - 用户资料：`/api/v1/users`
  - 权限目录：`/api/v1/permissions`
  - 角色管理：`/api/v1/roles`
  - 用户角色绑定：`/api/v1/users/:user_id/roles`
  - 角色权限绑定：`/api/v1/roles/:role_id/permissions`
- 保持 auth 公有接口、auth 仅认证接口、健康检查、Swagger 和 `OPTIONS` 不进入 RBAC 初始化 catalog。
- 为每个系统权限分配稳定 `permission_id`，并补齐超级管理员默认角色权限绑定。
- 增强 catalog 测试和 route scanner/route diff 测试，确保初始化 URL 与现有可授权路由一致。
- 如当前 Swagger 注解或开发文档没有清楚说明接口用途、请求方式、入参与返回结构，同步补充说明。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-baseline-url-catalog`: 系统 RBAC 初始化权限 catalog 覆盖当前全部已受 Casbin RBAC 保护的业务接口，并与 Gin route template 保持一致。

## Impact

- 影响 `user-service/internal/features/permission/application/rbacbaseline/catalog.go` 和对应测试。
- 影响 RBAC seed 结果：后续执行 `make seed-rbac` 会新增或更新缺失的系统权限记录，并给超级管理员角色补齐默认绑定。
- 影响 route diff 期望：seed 后 `GET /api/v1/permissions/route-diff` 不应再报告当前已注册 RBAC 业务路由缺失。
- 不改变数据库 schema、Ent schema、migration、Casbin model、subject 格式或授权中间件执行顺序。
- 不新增 OpenSpec/OPSX 工件；本变更记录位于仓库现有 `docs/changes/`。
