## Why

当前 `DefaultRoles()` 和 `DefaultRolePermissions()` 分开维护，且默认绑定逻辑直接表达为超级管理员绑定全部权限。未来如果新增默认系统角色，开发者需要同时修改角色列表和绑定展开逻辑，容易引入分支堆叠或按模块、模型粗粒度推导权限，从而造成误授权。

## What Changes

- 在 `internal/shared/rbacbaseline` 中把默认角色元数据和该角色的默认权限来源放入同一个内部 catalog block 维护。
- 保留现有公开函数签名：`DefaultRoles() []RoleSpec`、`DefaultPermissions() []PermissionSpec` 和 `DefaultRolePermissions() []RolePermissionSpec`。
- 明确超级管理员是唯一允许使用“全部默认权限”自动绑定的内置角色。
- 明确未来新增默认角色时必须显式列出 `PermissionID`，不得按 `Module`、model、read/write 或其他粗粒度集合自动推导。
- 保持当前运行行为不变：默认角色仍然只有超级管理员，超级管理员仍然绑定全部默认权限。
- 调整 baseline 测试，验证绑定引用已知角色、已知权限、无重复，并继续验证超级管理员绑定全部默认权限。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 补充默认系统角色与默认角色权限绑定的维护约束，要求非超级管理员默认角色显式维护权限 ID 列表并禁止粗粒度自动推导。

## Impact

- 代码影响：`user-service/internal/shared/rbacbaseline/catalog.go` 和 `user-service/internal/shared/rbacbaseline/catalog_test.go`。
- API/OpenAPI 影响：无，公开 HTTP API、请求和响应契约不变。
- 数据库影响：无，不修改 Ent schema，不新增 Atlas migration。
- 安全影响：降低未来新增默认角色时因按模块或集合推导权限导致误授权的风险。
- 运维影响：RBAC seed 的最终基线输出保持不变；全新数据库和既有数据库 seed 行为不变。
