## ADDED Requirements

### Requirement: Use explicit controller handler bindings in route registration

用户服务 HTTP 运行时 SHALL 在路由注册中绑定明确表达业务动作的 controller handler 名称。实现 MUST 保持现有 HTTP 路径、方法、公开/受保护路由分组、认证中间件挂载顺序和响应行为不变。

#### Scenario: User routes bind explicit handlers
- **Given** 用户服务 HTTP 路由已注册
- **When** 开发者检查用户资源路由 handler 绑定
- **Then** `GET /api/v1/users` MUST 绑定 `UserController.ListUsers`
- **Then** `POST /api/v1/users` MUST 绑定 `UserController.CreateUser`
- **Then** `GET /api/v1/users/:user_id` MUST 继续绑定 `UserController.GetByID`

#### Scenario: Auth routes bind explicit session handlers
- **Given** 用户服务 HTTP 路由已注册
- **When** 开发者检查认证路由 handler 绑定
- **Then** `POST /api/v1/auth/login` MUST 绑定 `AuthController.LoginUser`
- **Then** `POST /api/v1/auth/refresh` MUST 绑定 `AuthController.RefreshToken`
- **Then** `POST /api/v1/auth/change-password` MUST 继续绑定 `AuthController.ChangePassword`
- **Then** `POST /api/v1/auth/logout` MUST 绑定 `AuthController.LogoutCurrentSession`
- **Then** `POST /api/v1/auth/logout-all` MUST 绑定 `AuthController.LogoutAllSessions`

#### Scenario: Route surface remains unchanged
- **Given** controller handler 标识符已重命名
- **When** 用户服务注册 HTTP 路由
- **Then** 所有现有用户和认证 API 的 path、HTTP method、公开/受保护分组和中间件顺序 MUST 保持不变
