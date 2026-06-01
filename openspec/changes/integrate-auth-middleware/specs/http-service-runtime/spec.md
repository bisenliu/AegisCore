## MODIFIED Requirements

### Requirement: Register standard HTTP routes and middleware

系统必须注册健康检查、用户 API 路由、Swagger 文档路由和共享 HTTP 中间件。HTTP 中间件必须先注入 trace-id，再执行 panic recovery、请求日志、CORS 和认证策略。trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。用户服务运行时 MUST 对 `/api/v1` 业务路由启用认证，并 MUST 保持健康检查和 Swagger 文档路径可公开访问。

#### Scenario: Health endpoint returns service status
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /healthz`
- **Then** 系统返回 HTTP 200
- **Then** 响应包含 `status: ok` 和 `service: aegiscore-user-services`

#### Scenario: User API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /api/v1/users/:id`
- **Then** 请求被路由到 `UserController.GetByID`

#### Scenario: Create user API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 请求被路由到 `UserController.Create`

#### Scenario: Swagger routes are registered when enabled
- **Given** HTTP server 已启动且 Swagger 已启用
- **When** 调用方请求 `GET /swagger/index.html`
- **Then** 请求被路由到 Swagger UI handler
- **Then** `GET /docs` 和 `GET /api-docs` 重定向到 `/swagger/index.html`

#### Scenario: Request middleware is applied
- **Given** 任意 HTTP 请求进入服务
- **When** Gin engine 处理请求
- **Then** 请求经过 trace id、panic recovery、request logging、CORS 和认证相关中间件
- **Then** trace id 中间件必须在 request logging、recovery 和认证中间件之前执行

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

#### Scenario: Recovery log includes trace-id
- **Given** HTTP handler 发生 panic
- **When** recovery 中间件恢复 panic 并输出错误日志
- **Then** 日志必须包含 `trace-id`、panic 内容和 stack 字段
- **Then** HTTP 响应仍必须使用 `common/response.Envelope` 失败格式

#### Scenario: Public routes bypass authentication
- **Given** HTTP server 已启动
- **When** 调用方请求 `/healthz`、`/swagger/index.html`、`/docs` 或 `/api-docs` 且未携带认证 header
- **Then** 用户服务运行时 MUST 允许这些公开路径继续由对应 handler 处理
- **Then** 系统 MUST NOT 因缺少认证 header 返回 HTTP 401

#### Scenario: Versioned user APIs require authentication
- **Given** HTTP server 已启动
- **When** 调用方请求 `/api/v1/users` 或 `/api/v1/users/:id` 且未携带有效 Bearer token
- **Then** 用户服务运行时 MUST 在进入 controller 前拒绝请求
- **Then** 系统 MUST 返回 HTTP 401 和统一失败响应信封
