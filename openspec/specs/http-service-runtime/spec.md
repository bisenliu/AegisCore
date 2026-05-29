# http-service-runtime

## Purpose

HTTP 服务运行时能力负责通过 CLI 启动用户服务、组装 Fx 依赖、注册 Gin 中间件和路由，并在进程终止时优雅关闭 HTTP server。

## Requirements

### Requirement: Start service through CLI

系统必须提供 `aegiscore-user-services serve` 命令，并允许通过 `--config` 指定 YAML 配置路径。用户服务启动时必须初始化自己声明的运行时依赖，包括 HTTP server、具名 PostgreSQL 连接池和具名 Redis client。

#### Scenario: Start with explicit config path
- **Given** 调用方提供有效配置文件路径 `./user-services/configs/config.yaml`
- **When** 执行 `go run ./user-services/cmd serve --config ./user-services/configs/config.yaml`
- **Then** CLI 创建 Fx app
- **Then** 系统加载配置并启动 HTTP server
- **Then** HTTP server 监听配置中的 `http.host` 和 `http.port`
- **Then** 系统初始化用户服务声明的 `cache_redis` Redis client

#### Scenario: Start command uses default config path
- **Given** 调用方在 `user-services` 模块上下文运行服务
- **When** 执行 `serve` 且没有传入 `--config`
- **Then** CLI 使用默认路径 `./configs/config.yaml`

#### Scenario: Startup fails when dependencies are unavailable
- **Given** 配置引用的 `cache_redis` Redis 或 PostgreSQL 不可连接
- **When** Fx app 启动基础设施生命周期
- **Then** 启动过程返回错误
- **Then** 服务不应假装已经健康可用

### Requirement: Register standard HTTP routes and middleware

系统必须注册健康检查、用户 API 路由和共享 HTTP 中间件。HTTP 中间件必须先注入 trace-id，再执行 panic recovery、请求日志和 CORS。trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。

#### Scenario: Health endpoint returns service status
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /healthz`
- **Then** 系统返回 HTTP 200
- **Then** 响应包含 `status: ok` 和 `service: aegiscore-user-services`

#### Scenario: User API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /api/v1/users/:id`
- **Then** 请求被路由到 `UserController.GetByID`

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

#### Scenario: Recovery log includes trace-id
- **Given** HTTP handler 发生 panic
- **When** recovery 中间件恢复 panic 并输出错误日志
- **Then** 日志必须包含 `trace-id`、panic 内容和 stack 字段
- **Then** HTTP 响应仍必须使用 `common/response.Envelope` 失败格式

### Requirement: Shutdown gracefully

系统必须在收到中断或 SIGTERM 后停止接受新请求，并在配置的 shutdown timeout 内关闭 HTTP server。用户服务声明的 Redis client 和 PostgreSQL 连接池必须随 Fx app stop 释放。

#### Scenario: Process receives termination signal
- **Given** 服务正在运行
- **When** 进程收到 `os.Interrupt` 或 `SIGTERM`
- **Then** CLI 触发 Fx app stop
- **Then** HTTP server 使用 `http.shutdown_timeout` 或默认 `10s` 执行 graceful shutdown
- **Then** 用户服务声明的 `cache_redis` Redis client 被关闭

#### Scenario: ListenAndServe returns server closed
- **Given** HTTP server 正常关闭
- **When** `ListenAndServe` 返回 `http.ErrServerClosed`
- **Then** 系统不应将其记录为服务失败
