## ADDED Requirements

### Requirement: Use explicit authentication session handler names

用户会话控制能力 SHALL 在 auth controller 和路由注册中使用能独立表达认证或会话动作的 handler 名称。实现 MUST 保持 `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/logout`、`/api/v1/auth/logout-all` 和 `/api/v1/auth/change-password` 的 HTTP 契约、认证边界、响应信封、错误语义、Redis 会话行为和 token version 行为不变。

#### Scenario: Login route uses explicit handler name
- **Given** 公开认证路由已注册
- **When** 开发者检查 `POST /api/v1/auth/login` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `AuthController.LoginUser`
- **Then** 路由 MUST NOT 引用 `AuthController.Login`

#### Scenario: Refresh route uses explicit handler name
- **Given** 公开认证路由已注册
- **When** 开发者检查 `POST /api/v1/auth/refresh` 的 Gin handler 引用
- **Then** 路由 MUST 引用 `AuthController.RefreshToken`
- **Then** 路由 MUST NOT 引用 `AuthController.Refresh`

#### Scenario: Logout routes use explicit session handler names
- **Given** 受保护认证路由已注册
- **When** 开发者检查退出当前设备和退出全部设备的 Gin handler 引用
- **Then** `POST /api/v1/auth/logout` 路由 MUST 引用 `AuthController.LogoutCurrentSession`
- **Then** `POST /api/v1/auth/logout-all` 路由 MUST 引用 `AuthController.LogoutAllSessions`
- **Then** 路由 MUST NOT 引用 `AuthController.Logout` 或 `AuthController.LogoutAll`

#### Scenario: Authentication session API behavior remains unchanged
- **Given** 调用方按现有契约请求认证或会话接口
- **When** 系统处理登录、刷新 token、退出当前设备或退出全部设备请求
- **Then** 系统 MUST 保持现有 token 签发、刷新、撤销、token version 和统一响应行为
- **Then** controller、service、session store 和 repository 的职责边界 MUST 保持不变
