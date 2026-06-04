## ADDED Requirements

### Requirement: Use explicit user list handler names

用户列表查询能力 SHALL 在 controller 和路由注册中使用能独立表达用户列表语义的 handler 名称。实现 MUST 保持 `GET /api/v1/users` 的请求参数、认证要求、响应信封、分页语义、错误语义和分层职责不变。

#### Scenario: User list route uses explicit handler name
- **Given** 用户列表路由已注册
- **When** 开发者检查 `GET /api/v1/users` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `UserController.ListUsers`
- **Then** 路由 MUST NOT 引用 `UserController.List`

#### Scenario: User list API behavior remains unchanged
- **Given** 调用方携带有效 Bearer token
- **When** 调用方请求 `GET /api/v1/users`
- **Then** 系统 MUST 继续通过统一响应信封返回分页用户列表
- **Then** controller、service 和 repository 的职责边界 MUST 保持不变
