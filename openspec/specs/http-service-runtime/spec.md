# http-service-runtime

## Purpose

HTTP 服务运行时能力负责通过 CLI 启动用户服务、组装 Fx 依赖、注册 Gin 中间件和路由，并在进程终止时优雅关闭 HTTP server。

## Requirements

### Requirement: Start service through CLI

系统必须提供 `aegiscore-user-services serve` 命令，并允许通过 `--config` 指定 YAML 配置路径。用户服务启动时必须在自身 Fx 装配中显式提供公共配置和 Zap logger，并初始化自己声明的运行时依赖，包括 HTTP server、具名 PostgreSQL 连接池和具名 Redis client。

#### Scenario: Start with explicit config path
- **Given** 调用方提供有效配置文件路径 `./user-services/configs/config.yaml`
- **When** 执行 `go run ./user-services/cmd serve --config ./user-services/configs/config.yaml`
- **Then** CLI 创建 Fx app
- **Then** 用户服务启动装配显式提供共享配置和 Zap logger provider
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

#### Scenario: Service app does not depend on common infrastructure module
- **Given** 用户服务创建 Fx app
- **When** 查看用户服务启动装配
- **Then** 用户服务不得通过 `common/infrastructure.Module` 注入公共依赖
- **Then** 用户服务必须手动依次提供公共配置、Zap logger 和自身声明的运行时依赖

### Requirement: Initialize configured timezone during user service startup

用户服务运行时 MUST 在 Fx 启动图中引入共享 timezone module，并使用加载后的共享配置初始化进程本地时区。该初始化 MUST 与用户服务声明的 HTTP server、Redis、PostgreSQL 和 Ent runtime 依赖一起参与启动失败处理。

#### Scenario: Start service with configured timezone
- **Given** 用户服务配置文件包含 `system.timezone: Asia/Shanghai`
- **When** 执行 `go run ./user-services/cmd serve --config ./user-services/configs/config.yaml`
- **Then** Fx app MUST 引入共享 timezone module
- **Then** 服务启动时 MUST 将进程本地时区初始化为 `Asia/Shanghai`
- **Then** HTTP server、日志和业务代码中依赖 `time.Local` 的行为 MUST 使用该时区

#### Scenario: Startup fails for invalid timezone
- **Given** 用户服务配置文件包含无效的 `system.timezone`
- **When** 执行 `serve` 命令启动服务
- **Then** Fx app 启动 MUST 返回错误
- **Then** HTTP server MUST NOT 假装已经健康可用
- **Then** 错误 MUST 保留共享 timezone 初始化失败上下文

#### Scenario: Timezone startup does not add extra datastore dependencies
- **Given** 用户服务引入共享 timezone module
- **When** Fx app 启动用户服务
- **Then** timezone 初始化 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client 或 HTTP 路由
- **Then** 用户服务仍 MUST 只连接自己声明的 `cache_redis`、`user_db` 和 `common_db` 运行时依赖

### Requirement: Register standard HTTP routes and middleware

系统必须注册健康检查、用户 API 路由、Swagger 文档路由和共享 HTTP 中间件。HTTP 中间件必须先注入 trace-id，再执行 panic recovery、请求日志、CORS 和认证策略。trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。用户服务运行时 MUST 对 `/api/v1` 业务路由启用认证，并 MUST 保持健康检查和 Swagger 文档路径可公开访问。用户服务运行时 MUST 在注册认证中间件时传入 Fx 注入的 Zap logger。请求日志的 `client_ip` 字段必须使用 Gin `Context.ClientIP()` 的结果。

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

#### Scenario: Authentication middleware receives runtime logger
- **Given** 用户服务 Fx app 已注入 Zap logger
- **When** 用户服务运行时创建 Gin engine 并注册认证中间件
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
- **When** 调用方请求 `/healthz`、`/swagger/index.html`、`/docs` 或 `/api-docs` 且未携带认证 header
- **Then** 用户服务运行时 MUST 允许这些公开路径继续由对应 handler 处理
- **Then** 系统 MUST NOT 因缺少认证 header 返回 HTTP 401

#### Scenario: Versioned user APIs require authentication
- **Given** HTTP server 已启动
- **When** 调用方请求 `/api/v1/users` 或 `/api/v1/users/:id` 且未携带有效 Bearer token
- **Then** 用户服务运行时 MUST 在进入 controller 前拒绝请求
- **Then** 系统 MUST 返回 HTTP 401 和统一失败响应信封

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

### Requirement: Provide high-throughput HTTP timeout defaults

用户服务示例配置 MUST 提供适合较高请求量和 keep-alive 复用场景的 HTTP timeout 基线。默认 YAML 配置 MUST 设置 `http.read_timeout` 为 `30s`、`http.write_timeout` 为 `60s`、`http.idle_timeout` 为 `120s`、`http.shutdown_timeout` 为 `25s`。这些值 MUST 通过现有 YAML 与 `AEGISCORE_` 环境变量覆盖机制加载，HTTP server MUST 使用加载后的配置值。

#### Scenario: Load default HTTP timeouts from config
- **Given** 用户服务使用 `user-services/configs/config.yaml` 启动
- **When** 系统加载 HTTP runtime 配置
- **Then** `http.read_timeout` MUST 为 `30s`
- **Then** `http.write_timeout` MUST 为 `60s`
- **Then** `http.idle_timeout` MUST 为 `120s`
- **Then** `http.shutdown_timeout` MUST 为 `25s`

#### Scenario: HTTP server uses configured timeout values
- **Given** 配置加载得到 HTTP timeout 值
- **When** 用户服务创建 HTTP server 并执行 graceful shutdown
- **Then** HTTP server MUST 使用配置中的 read、write 和 idle timeout
- **Then** graceful shutdown MUST 使用配置中的 shutdown timeout

#### Scenario: Environment can override timeout defaults
- **Given** 部署环境通过 `AEGISCORE_` 前缀环境变量覆盖 HTTP timeout 配置
- **When** `common/config.Load` 反序列化运行时配置
- **Then** 用户服务 MUST 使用环境变量覆盖后的 timeout 值
- **Then** 配置加载器 MUST NOT 对 timeout 执行额外 required 或范围校验

### Requirement: HTTP runtime naming cleanup preserves service contract
HTTP 服务运行时相关命名标准化 SHALL 只修改内部组装名称、局部变量、内部类型或文档表达，不得改变 CLI、路由注册、中间件顺序、健康检查、Swagger 暴露或优雅关闭行为。

#### Scenario: Runtime module names are standardized
- **WHEN** 实现修改 `user-services/internal/bootstrap`、`user-services/internal/router` 或 `user-services/cmd` 中的内部命名
- **THEN** 服务启动命令、HTTP 路由、Fx 依赖关系和关闭流程 MUST 与修改前保持等价

#### Scenario: Service identity name is reviewed
- **WHEN** 审查发现 `user-services` 复数命名或服务名语义可改进
- **THEN** 本变更 MUST 保留该名称，并将目录、module path、CLI 名或服务标识重命名视为单独 breaking change
