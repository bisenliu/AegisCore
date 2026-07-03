## ADDED Requirements

### Requirement: RBAC 路由注册测试覆盖
系统 MUST 使用 router 包测试覆盖权限、角色和用户角色路由在 user-service 聚合路由中的注册结果，确保 RBAC 保护接口只注册在当前 `/api/v1` 路由图并经过当前认证和授权中间件链。

#### Scenario: 权限和角色路由注册
- **WHEN** PermissionController 和 RoleController 均已提供给 `registerV1Routes`
- **THEN** 测试 MUST 验证权限目录、route diff、用户有效权限、角色生命周期、角色权限绑定和用户角色绑定路由注册在 `/api/v1` 下
- **AND** 测试 MUST 验证这些路由进入当前认证和 RBAC 授权中间件链

#### Scenario: 可选 controller 条件注册
- **WHEN** PermissionController 或 RoleController 为 nil
- **THEN** `registerV1Routes` MUST 不注册对应权限或角色可选路由
- **AND** auth 路由和 user 路由 MUST 继续按当前路径注册
- **AND** 测试 MUST NOT 通过旧路径兼容别名补偿缺失的可选路由
