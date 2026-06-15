# Design

## Ownership

`permission/application/rbacbaseline` 是系统 RBAC 基线唯一 owner。该包只承载稳定基线数据和无副作用校验，不访问 Ent、Redis、Gin、Casbin 或外部系统。

包内包含：

- `SuperAdminRoleID`
- `RoleSpec`
- `PermissionSpec`
- `RolePermissionSpec`
- `DefaultRoles`
- `DefaultPermissions`
- `DefaultRolePermissions`

Role feature 的 seed service 消费该包，将系统角色、系统权限和默认绑定映射为 role/permission application ports 的输入。Permission Casbin adapter 消费同一个 `SuperAdminRoleID`，补充内置超级管理员 wildcard policy。

## Dependency Shape

```text
permission/application/rbacbaseline
  ↑
  ├─ role/application/seed
  └─ permission/infrastructure/casbin
```

该依赖方向保持 `common` 不承载业务语义，也避免新增横向 `internal/shared`。RBAC baseline 是用户服务 permission feature 的业务基线，不是跨服务 runtime primitive。

## Removed Entrypoints

删除以下旧入口，避免双来源：

- `permission/application/catalog`
- `role/application/catalog`

历史 `docs/changes/*` 中的旧路径说明保留为历史上下文；长期规则文档更新为当前结构。

## Behavior

本变更只调整基线数据的代码归属，不改变数据内容：

- 超级管理员角色 ID 保持 `00000000-0000-0000-0000-000000000001`。
- 三个系统用户权限 ID、HTTP method 和 route template 保持不变。
- 超级管理员默认绑定三个系统用户权限保持不变。
- Casbin wildcard policy 仍由 Casbin adapter 追加，`*` 仍是 adapter 私有实现细节。

## Tests

新增或迁移 baseline 测试，覆盖：

- 角色 ID 合法且唯一。
- 权限 ID 合法且唯一。
- 权限 route identity 唯一。
- 默认角色权限绑定不引用未知角色或未知权限。
- 超级管理员角色存在且系统标记完整。

现有 seed 和 Casbin 测试更新为消费 `rbacbaseline`。
