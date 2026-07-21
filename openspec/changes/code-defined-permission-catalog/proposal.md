## Why

当前权限目录同时允许代码基线、数据库状态和管理员写接口改变权限定义，导致可授权路由与权限数据可能漂移，也让权限启停和系统标记引入了不必要的运行时状态。项目尚未正式上线，适合现在收敛为“代码定义、数据库只读投影”，以稳定 `permission_id` 维护角色绑定，并在 CI 中阻止路由与权限基线不一致。

## What Changes

- **BREAKING** 权限只能由 `rbacbaseline.DefaultPermissions()` 定义；管理员不再能够创建、修改、启用或停用权限。
- **BREAKING** 权限 HTTP API 只保留权限列表和用户有效权限查询，删除创建、详情、更新、启停和公开 route diff 共 6 个接口。
- **BREAKING** 从 Permission 全链路删除 `Active`、`IsSystem`、`active` 和 `is_system`，同时删除数据库列、索引、请求响应字段、列表过滤条件和相关错误语义；`Role.Active` 与 `Role.IsSystem` 保持不变。
- RBAC seed 按稳定 `permission_id` 将代码基线 upsert 到数据库；权限 method 或 path 变化时沿用原 ID，权限删除由受控 migration 先清理 `role_permissions` 再删除 `permissions`，不依赖运行时自动删除。
- 清理已删除权限 HTTP 路由对应的 6 条权限及其角色绑定，并在数据变更后通过显式 reload 或滚动重启收敛 Casbin policy。
- 删除 permission command 能力、无用 transport/application/infrastructure 代码和生产 route diff 装配；保留列表、有效权限、seed upsert、角色绑定查询和基于 HTTP method + route template 的 Casbin 鉴权。
- 角色权限绑定只要求权限存在；Casbin policy 继续只加载启用角色对应的权限绑定。
- 将 route diff 从公开运行时诊断迁移为 CI/测试门禁：构建真实 Gin route graph，比对 `/api/v1` 受保护路由与 `rbacbaseline.DefaultPermissions()`，发现 missing 或 stale 时失败，并排除认证公开接口、会话控制接口和 `OPTIONS`。
- 同步更新 Ent schema、Atlas migration、E2E 初始化 SQL、OpenAPI 注解与生成物、Fx/route 装配、测试、产品和架构文档以及 RBAC 主规格。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 将权限目录改为代码定义且数据库只读投影，收窄公开权限 API，移除权限状态和系统标记，引入路由基线 CI 一致性门禁，并调整 seed、角色绑定和 Casbin policy 来源语义。

## Impact

- API 与 OpenAPI：`user-service/internal/features/permission/transport/http/`、路由注册测试、`user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml` 删除 6 个公开接口及相关 schema 字段。
- 数据与迁移：`user-service/ent/schema/`、Ent 生成代码、Atlas SQL migration、`permissions` 表、相关索引、`role_permissions` 清理和 E2E 初始化 SQL 发生破坏性 schema/data migration。
- RBAC 业务：`user-service/internal/shared/rbacbaseline/`、permission application/domain/store、role 权限绑定、seed 和 Casbin policy loader 的契约及测试发生变化。
- 运行时装配与观测：permission Fx providers、route registrar、route diff scanner/application query/metrics 的生产接线将被删除或收缩，现有 policy reload、Redis policy sync 和启用角色过滤继续保留。
- 文档与规格：更新 `openspec/specs/rbac-access-control/spec.md`、`docs/PRODUCT.md` 和 `docs/ARCHITECTURE.md`；实现需通过生成、migration、OpenAPI、架构 lint 和完整验证门禁。
