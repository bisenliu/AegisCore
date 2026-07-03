## ADDED Requirements

### Requirement: 用户资料路由注册测试覆盖
系统 MUST 使用 router 包测试覆盖用户资料路由在 user-service 聚合路由中的注册结果，确保用户接口只存在于当前 `/api/v1/users` 路由图并受当前认证和 RBAC 授权中间件链保护。

#### Scenario: 用户资料路由注册
- **WHEN** `registerV1Routes` 注册当前 `/api/v1` 路由组
- **THEN** 测试 MUST 验证用户创建、用户详情和用户列表路由注册在 `/api/v1/users` 下
- **AND** 测试 MUST 验证这些用户资料路由进入当前认证和 RBAC 授权中间件链
- **AND** 测试 MUST NOT 为 `/api/users`、`/v1/users` 或其他旧用户路径保留兼容断言
