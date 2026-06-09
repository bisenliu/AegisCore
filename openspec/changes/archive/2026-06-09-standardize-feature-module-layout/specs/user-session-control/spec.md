## MODIFIED Requirements

### Requirement: Authentication sessions use repository abstraction with Redis implementation boundary
用户会话控制能力 SHALL 通过认证 app 层声明的 `authapp.AuthSessionStore` 抽象管理 token version、Refresh Token 会话和用户活跃会话索引，具体 Redis 实现 MUST 位于 `user-services/internal/features/auth/infra/redis` 包。service 层 MUST NOT 定义或持有 Redis session store 具体实现。

#### Scenario: Auth service depends on auth session repository abstraction
- **Given** 登录、刷新、退出当前设备、退出全部设备或修改密码流程需要访问会话状态
- **When** auth service 调用会话数据访问层
- **Then** auth service MUST 依赖 `authapp.AuthSessionStore` 或更高层 session lifecycle 组件
- **Then** auth service MUST 使用 `authdomain.AuthSession` 表达会话数据
- **Then** auth service MUST NOT 依赖 Redis client 或 `features/auth/infra/redis` 私有实现类型

#### Scenario: Session not found error remains mappable
- **Given** Redis 中不存在指定 Refresh Token 会话记录
- **When** auth service 读取会话
- **Then** Redis 实现 MUST 返回 `authdomain.ErrAuthSessionNotFound`
- **Then** auth service MUST 继续将该错误映射为未认证或 token 无效响应

#### Scenario: Token version mismatch remains mappable
- **Given** token claims 或会话记录中的 `token_version` 与服务端当前版本不一致
- **When** 系统校验 token version
- **Then** auth session store 或 lifecycle 组件 MUST 返回认证领域 token version mismatch 错误
- **Then** 系统 MUST 继续拒绝刷新、受保护请求或改密凭据校验

#### Scenario: Redis session storage behavior remains compatible
- **Given** `features/auth/infra/redis` 承载认证会话 Redis 实现
- **When** 系统创建、读取、删除或批量删除认证会话
- **Then** Redis key 格式、Refresh Token 会话 TTL、用户活跃会话 ZSet 和过期 member 清理行为 MUST 与迁移前保持一致
- **Then** token version 缓存未命中时 Redis 实现 MUST 只报告缓存未命中或等价结果，由认证会话 service 组件或 token version resolver 回源 PostgreSQL

### Requirement: Use explicit authentication session handler names
用户会话控制能力 SHALL 在 auth `transport/http` controller 和路由注册中使用能独立表达认证或会话动作的 handler 名称。实现 MUST 保持 `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/logout`、`/api/v1/auth/logout-all` 和 `/api/v1/auth/change-password` 的 HTTP 契约、认证边界、响应信封、错误语义、Redis 会话行为和 token version 行为不变。

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
- **Then** transport/http、app service、session infra 和 PostgreSQL infra 的职责边界 MUST 保持不变
