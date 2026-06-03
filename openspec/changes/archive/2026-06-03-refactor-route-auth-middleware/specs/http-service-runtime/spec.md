## MODIFIED Requirements

### Requirement: Register standard HTTP routes and middleware

系统必须注册健康检查、用户 API 路由、Swagger 文档路由和共享 HTTP 中间件。HTTP 基础中间件必须先注入 trace-id，再执行 panic recovery、请求日志和 CORS；trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。用户服务运行时 MUST 通过路由局部分组控制认证中间件挂载：健康检查、Swagger 文档、登录、刷新和受限改密入口 MUST 保持公开访问；退出当前设备、退出全部设备和用户资料 API MUST 挂载认证中间件。用户服务运行时 MUST 在注册认证中间件时传入 Fx 注入的 Zap logger。请求日志的 `client_ip` 字段必须使用 Gin `Context.ClientIP()` 的结果。后续 Casbin 授权中间件 MUST 挂载在认证中间件之后、业务 handler 之前的受保护路由子分组中。

#### Scenario: Health endpoint returns service status
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /healthz`
- **Then** 系统返回 HTTP 200
- **Then** 响应包含 `status: ok` 和 `service: aegiscore-user-services`

#### Scenario: User API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 请求被路由到 `UserController.GetByID`

#### Scenario: Create user API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 请求被路由到 `UserController.Create`

#### Scenario: Auth routes are grouped by credential requirements
- **Given** HTTP server 已启动
- **When** 查看 `/api/v1/auth` 路由注册
- **Then** `POST /api/v1/auth/login`、`POST /api/v1/auth/refresh` 和 `POST /api/v1/auth/change-password` MUST 注册在公开路由分组
- **Then** `POST /api/v1/auth/logout` 和 `POST /api/v1/auth/logout-all` MUST 注册在已挂载认证中间件的路由分组

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

#### Scenario: Trace id is propagated to Go context
- **Given** 请求头包含 `X-Trace-ID`
- **When** trace id 中间件处理请求
- **Then** 系统必须将该值写入 Gin context
- **Then** 系统必须将该值写入 `c.Request.Context()`
- **Then** 系统必须将该值写入 `X-Trace-ID` 响应头

#### Scenario: Trace id is generated when missing
- **Given** 请求头不包含 `X-Trace-ID`
- **When** trace id 中间件处理请求
- **Then** 系统必须生成新的 trace-id
- **Then** 系统必须将生成值写入 Gin context、Go context 和响应头

#### Scenario: Request log includes trace-id
- **Given** HTTP 请求已完成
- **When** request logging 中间件输出请求日志
- **Then** 日志必须包含 `trace-id`、method、path、status、latency 和 client_ip 字段
- **Then** `client_ip` 字段必须等于 Gin `Context.ClientIP()` 的结果

#### Scenario: Recovery log includes trace-id
- **Given** HTTP handler 发生 panic
- **When** recovery 中间件恢复 panic 并输出错误日志
- **Then** 日志必须包含 `trace-id`、panic 内容和 stack 字段
- **Then** HTTP 响应仍必须使用 `common/response.Envelope` 失败格式

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

#### Scenario: Authorization middleware has a stable future mounting point
- **Given** 后续用户服务接入 Casbin 授权中间件
- **When** 服务为需要细粒度授权的业务 API 注册路由
- **Then** Casbin 中间件 MUST 挂载在认证中间件之后
- **Then** Casbin 中间件 MUST 在对应业务 handler 执行之前完成授权判定
