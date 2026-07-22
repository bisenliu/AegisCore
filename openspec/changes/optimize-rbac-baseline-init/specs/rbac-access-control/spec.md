## ADDED Requirements

### Requirement: 默认系统角色权限基线维护

系统 MUST 在 `internal/shared/rbacbaseline` 中集中维护默认系统角色及其默认权限绑定。`DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 的公开函数签名 MUST 保持稳定；默认角色权限绑定 MUST 只引用 `DefaultPermissions()` 中的稳定 `PermissionID`。

#### Scenario: 当前默认基线行为保持不变

- **WHEN** 代码调用 `rbacbaseline.DefaultRoles()`
- **THEN** 系统 MUST 仍然只返回内置超级管理员角色作为当前默认角色
- **WHEN** 代码调用 `rbacbaseline.DefaultRolePermissions()`
- **THEN** 系统 MUST 仍然返回超级管理员角色到全部默认权限的绑定
- **AND** 绑定集合 MUST 不包含重复的 `RoleID` 与 `PermissionID` 组合

#### Scenario: 默认角色绑定引用已知基线

- **WHEN** 系统展开默认角色权限绑定
- **THEN** 每条绑定的 `RoleID` MUST 引用 `DefaultRoles()` 返回的已知默认角色
- **AND** 每条绑定的 `PermissionID` MUST 引用 `DefaultPermissions()` 返回的已知默认权限

#### Scenario: 未来默认角色显式维护权限

- **WHEN** 后续新增非超级管理员默认系统角色
- **THEN** 该角色的默认权限 MUST 在角色 catalog block 中显式列出 `PermissionID`
- **AND** 系统 MUST NOT 按 `Module`、model、read/write、路由前缀或其他粗粒度集合自动推导该角色的默认权限
- **AND** 系统 MUST NOT 为了表达默认角色权限引入 `PermissionSet` 别名层

#### Scenario: 超级管理员全量绑定例外

- **WHEN** 系统展开超级管理员默认权限绑定
- **THEN** 系统 MUST 允许超级管理员使用内部 helper 自动绑定 `DefaultPermissions()` 中的全部权限
- **AND** 该自动全量绑定例外 MUST NOT 扩展到其他默认系统角色
