# Consolidate RBAC Baseline

## What

将系统内置 RBAC 基线统一收敛到 permission feature 的 `application/rbacbaseline` 包，作为超级管理员角色、系统权限和系统角色权限绑定的唯一来源。

## Why

当前 RBAC 基线分散在 role 与 permission 两个 catalog 包中，Casbin policy loader 还额外硬编码了一份超级管理员角色 ID。这会带来稳定 ID 漂移、默认绑定与权限目录不一致，以及新增系统权限时需要同时修改多个 owner 的风险。

RBAC 授权由 permission feature 拥有，系统 RBAC 基线也应由 permission application 层提供明确边界。Role seed 和 Casbin infrastructure 只消费该基线，不再各自维护重复常量。

## Scope

- 新增 `user-service/internal/features/permission/application/rbacbaseline`。
- 删除旧的 `permission/application/catalog` 和 `role/application/catalog` 入口。
- 更新 RBAC seed、Casbin policy loader 和相关测试引用。
- 更新长期架构文档，明确 RBAC baseline owner。

## Non-Goals

- 不修改数据库 schema、Ent generated code 或 Atlas migration。
- 不改变 HTTP API、Swagger 契约或 Redis key。
- 不改变 Casbin model、policy subject 格式或授权行为。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Validation

- `go test ./internal/features/permission/application/rbacbaseline ./internal/features/role/application/seed ./internal/features/permission/infrastructure/casbin`
- `rg -n "application/catalog|defaultSuperAdminRoleID|rolecatalog|permissioncatalog" user-service/internal/features`
