## MODIFIED Requirements

### Requirement: Register standard HTTP routes and middleware

系统必须注册健康检查、用户 API 路由、Swagger 文档路由和共享 HTTP 中间件。HTTP 基础中间件必须先注入 trace-id，再执行 panic recovery、请求日志和 CORS；trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。用户服务运行时 MUST 通过服务级 HTTP 路由入口注册完整用户服务 HTTP surface，该入口 MUST 使用明确表达用户服务 HTTP 范围的命名，并 MUST 按系统路由、Swagger 文档路由、版本化 API、公共认证路由、受保护认证路由和用户资源路由组织总装逻辑。用户和认证业务 endpoint 的具体路由注册 MUST 由对应 feature 的 `transport/http/routes.go` 拥有；服务级路由入口只创建分组、挂载认证中间件并调用 feature route registration。用户服务运行时 MUST 将 `config.App.Name` 作为健康检查响应中的服务名来源传入系统路由，MUST NOT 在健康检查 handler 中硬编码服务名或设置代码级默认服务名；健康检查成功状态值 MUST 使用路由包内拥有的常量表达。用户服务运行时 MUST 通过路由局部分组控制认证中间件挂载：健康检查、Swagger 文档、登录、刷新和受限改密入口 MUST 保持公开访问；退出当前设备、退出全部设备和用户资料 API MUST 挂载认证中间件。用户服务运行时 MUST 在注册认证中间件时传入 Fx 注入的 Zap logger。请求日志的 `client_ip` 字段必须使用 Gin `Context.ClientIP()` 的结果。后续 Casbin 授权中间件 MUST 挂载在认证中间件之后、业务 handler 之前的受保护路由子分组中。

#### Scenario: Health endpoint returns service status
- **Given** HTTP server 已启动且配置包含 `app.name: aegiscore-user-services`
- **When** 调用方请求 `GET /healthz`
- **Then** 系统返回 HTTP 200
- **Then** 响应包含 `status: ok`
- **Then** 响应包含 `service` 且值来自 `config.App.Name`
- **Then** 系统 MUST NOT 在健康检查 handler 中硬编码服务名

#### Scenario: User API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 请求被路由到用户 feature `transport/http` controller 的 `GetByUserID`

#### Scenario: Create user API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 请求被路由到用户 feature `transport/http` controller 的 `CreateUser`

#### Scenario: Auth routes are grouped by credential requirements
- **Given** HTTP server 已启动
- **When** 查看 `/api/v1/auth` 路由注册
- **Then** `POST /api/v1/auth/login`、`POST /api/v1/auth/refresh` 和 `POST /api/v1/auth/change-password` MUST 通过认证 feature `RegisterPublicRoutes` 注册在公开路由分组
- **Then** `POST /api/v1/auth/logout` 和 `POST /api/v1/auth/logout-all` MUST 通过认证 feature `RegisterProtectedRoutes` 注册在已挂载认证中间件的路由分组

#### Scenario: Swagger routes are registered when enabled
- **Given** HTTP server 已启动且 Swagger 已启用
- **When** 调用方请求 `GET /swagger/index.html`
- **Then** 请求被路由到 Swagger UI handler
- **Then** `GET /docs` 和 `GET /api-docs` 重定向到 `/swagger/index.html`

#### Scenario: Request middleware is applied
- **Given** 任意 HTTP 请求进入服务
- **When** Gin engine 处理请求
- **Then** 请求经过 trace id、panic recovery、request logging 和 CORS 中间件
- **Then** trace id 中间件必须在 request logging、recovery 和认证中间件之前执行
- **Then** 认证中间件 MUST 仅应用于受保护路由分组

#### Scenario: Authentication middleware receives runtime logger
- **Given** 用户服务 Fx app 已注入 Zap logger
- **When** 用户服务运行时创建受保护路由分组并注册认证中间件
- **Then** 系统 MUST 将该 Zap logger 传入共享认证中间件
- **Then** 认证中间件 MUST 使用同一个 logger 输出认证相关日志

#### Scenario: Public routes bypass authentication
- **Given** HTTP server 已启动
- **When** 调用方请求 `/healthz`、`/swagger/index.html`、`/docs`、`/api-docs`、`/api/v1/auth/login`、`/api/v1/auth/refresh` 或 `/api/v1/auth/change-password` 且未携带普通 Access Token
- **Then** 用户服务运行时 MUST 允许这些公开路径继续由对应 handler 处理
- **Then** 系统 MUST NOT 因缺少普通 Access Token 返回认证中间件产生的 HTTP 401

#### Scenario: Protected APIs require authentication
- **Given** HTTP server 已启动
- **When** 调用方请求 `/api/v1/users`、`/api/v1/users/:user_id`、`/api/v1/auth/logout` 或 `/api/v1/auth/logout-all` 且未携带有效 Bearer token
- **Then** 用户服务运行时 MUST 在进入 controller 前拒绝请求
- **Then** 系统 MUST 返回 HTTP 401 和统一失败响应信封

#### Scenario: Route registration is grouped by API surface
- **Given** 用户服务注册 HTTP 路由
- **When** 实现组织服务级路由总装和 feature-local route registration
- **Then** 系统路由、Swagger 文档路由、版本化 API、公共认证路由、受保护认证路由和用户资源路由 MUST 有清晰分组边界
- **Then** 用户资料业务 routes MUST 位于 `features/user/transport/http/routes.go`
- **Then** 认证业务 routes MUST 位于 `features/auth/transport/http/routes.go`
- **Then** 拆分 MUST 保持现有路径、HTTP 方法、handler 绑定和认证边界等价

### Requirement: Runtime composes concrete feature store implementations at the bootstrap boundary
HTTP 服务运行时 SHALL 通过 feature-owned Fx modules 装配具体 feature 实现。用户服务启动时，`features/user/module.go` MUST 提供用户资料 app service、HTTP controller 和 `features/user/infra/postgres` provider；`features/auth/module.go` MUST 提供认证 app service、HTTP controller、`features/auth/infra/postgres` provider 和 `features/auth/infra/redis` provider。`bootstrap.AppModule` MUST 引入 feature modules，并继续负责共享配置、Zap logger、timezone、validation、具名 PostgreSQL/Redis runtime、Ent client、Gin engine、HTTP server 和路由总装。现有 `user_db` Ent client、`cache_redis` Redis client 和 auth 配置依赖 MUST 保持不变。

#### Scenario: Bootstrap imports feature modules
- **Given** Fx app 装配用户服务依赖
- **When** bootstrap 创建 `AppModule`
- **Then** `AppModule` MUST 引入 `features/user.Module` 和 `features/auth.Module`
- **Then** bootstrap MUST NOT 逐个列出用户和认证 feature 内部 app service、HTTP controller 或 infra adapter provider
- **Then** feature modules MUST NOT 创建 PostgreSQL/Redis runtime clients 或 Ent clients

#### Scenario: User feature module provides PostgreSQL user profile adapter
- **Given** Fx app 装配用户 feature 依赖
- **When** 用户 feature module 创建用户资料 infra provider
- **Then** provider MUST 使用 `features/user/infra/postgres.NewUserStore` 或等价构造函数
- **Then** provider MUST 注入具名 `user_db` Ent client
- **Then** 下游 service MUST 接收 `userapp.UserProfileStore` 抽象

#### Scenario: Auth feature module provides credential and session adapters
- **Given** Fx app 装配认证 feature 依赖
- **When** 认证 feature module 创建认证凭据、token version 和会话 infra provider
- **Then** provider MUST 使用 `features/auth/infra/postgres.NewCredentialStore` 和 `features/auth/infra/redis.NewSessionStore` 或等价构造函数
- **Then** provider MUST 注入具名 `user_db` Ent client、具名 `cache_redis` Redis client 和 auth 配置
- **Then** 下游 auth service 和认证中间件 MUST 接收 `authapp.UserCredentialStore`、`authapp.UserTokenVersionStore`、`authapp.AuthSessionStore` 和 `authapp.TokenVersionValidator` 抽象

#### Scenario: Startup dependencies remain unchanged
- **Given** 用户服务通过 CLI 启动
- **When** Fx app 初始化 runtime 依赖
- **Then** 系统 MUST 继续只初始化自身声明的 `cache_redis`、`user_db` 和 `common_db` 运行时依赖
- **Then** 系统 MUST NOT 因 feature module 拆分新增 Redis、PostgreSQL、Ent client 或 HTTP 路由依赖
