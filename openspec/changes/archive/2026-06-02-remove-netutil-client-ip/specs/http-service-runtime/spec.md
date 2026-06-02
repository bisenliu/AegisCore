## MODIFIED Requirements

### Requirement: Register standard HTTP routes and middleware

系统必须注册健康检查、用户 API 路由、Swagger 文档路由和共享 HTTP 中间件。HTTP 中间件必须先注入 trace-id，再执行 panic recovery、请求日志和 CORS。trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。请求日志的 `client_ip` 字段必须使用 Gin `Context.ClientIP()` 的结果。

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
- **Then** 请求经过 trace id、panic recovery、request logging 和 CORS 中间件
- **Then** trace id 中间件必须在 request logging 和 recovery 之前执行

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
