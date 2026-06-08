# http-service-runtime

## Purpose

HTTP 服务运行时能力负责通过 CLI 启动用户服务、组装 Fx 依赖、注册 Gin 中间件和路由，并在进程终止时优雅关闭 HTTP server。

## Requirements

### Requirement: Start service through CLI

系统必须提供 `aegiscore-user-services serve` 命令，并允许通过 `--config` 指定 YAML 配置路径。用户服务启动时必须在自身 Fx 装配中显式提供公共配置和 Zap logger，并初始化自己声明的运行时依赖，包括 HTTP server、具名 PostgreSQL 连接池和具名 Redis client。CLI 启动和停止 Fx app 的 timeout MUST 使用语义独立的具名常量表达；启动超时表达 Fx app start budget，停止超时表达 Fx app stop budget，二者不得与 HTTP server graceful shutdown fallback 混用命名。

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
- **Then** 用户服务不得通过 `common/runtime/infrastructure.Module` 注入公共依赖
- **Then** 用户服务必须手动依次提供公共配置、Zap logger 和自身声明的运行时依赖

#### Scenario: CLI lifecycle timeouts are named separately
- **Given** CLI 启动或停止 Fx app
- **When** 实现创建 start context 和 stop context
- **Then** 启动 context MUST 使用表达 Fx app start budget 的启动超时常量
- **Then** 停止 context MUST 使用表达 Fx app stop budget 的停止超时常量
- **Then** 停止超时常量名称 MUST NOT 暗示它只控制 HTTP server shutdown

### Requirement: Report HTTP listen failures during startup

HTTP 服务运行时 MUST 在 Fx `OnStart` 完成前绑定配置中的 HTTP host 和 port。若监听器创建、端口绑定或地址绑定失败，启动过程 MUST 向 Fx 返回错误，服务 MUST NOT 在 HTTP server 实际不可用时报告启动成功。成功绑定后，HTTP server MUST 继续异步处理请求，并在正常关闭时继续忽略 `http.ErrServerClosed`。

#### Scenario: Startup fails when HTTP port is already in use
- **Given** 配置中的 `http.host` 和 `http.port` 指向一个已被占用的本地监听地址
- **When** Fx app 启动 HTTP server 生命周期
- **Then** `OnStart` MUST 返回包含监听失败上下文的错误
- **Then** 服务 MUST NOT 假装已经健康可用

#### Scenario: Startup succeeds only after HTTP listener is bound
- **Given** 配置中的 HTTP 地址可监听
- **When** Fx app 启动 HTTP server 生命周期
- **Then** `OnStart` MUST 在 HTTP listener 成功绑定后才返回成功
- **Then** HTTP server MUST 使用该 listener 异步处理请求

#### Scenario: Normal shutdown remains non-failing
- **Given** HTTP server 已成功启动
- **When** Fx app 停止并触发 graceful shutdown
- **Then** HTTP server MUST 使用现有 shutdown timeout 规则关闭
- **Then** `http.ErrServerClosed` MUST NOT 被记录为服务失败

### Requirement: Trigger application shutdown on unexpected HTTP serve failure

HTTP 服务运行时 MUST 在 HTTP listener 成功绑定并异步执行 `Serve` 后，检测 HTTP server 的非预期退出。若 `Serve` 返回的错误不是 `http.ErrServerClosed`，系统 MUST 记录失败并触发 Fx 应用级 shutdown，使进程进入现有停止流程。正常 graceful shutdown 导致的 `http.ErrServerClosed` MUST 继续被视为预期结果，不得记录为服务失败，也不得作为新的失败触发额外处理。

#### Scenario: Unexpected serve failure triggers application shutdown
- **Given** HTTP listener 已成功绑定且 `OnStart` 已返回成功
- **When** HTTP server 的异步 `Serve` 返回非 `http.ErrServerClosed` 错误
- **Then** 系统 MUST 记录 HTTP server 失败日志
- **Then** 系统 MUST 触发 Fx 应用级 shutdown
- **Then** 服务 MUST NOT 只依赖日志表示 HTTP server 已不可用

#### Scenario: Normal server shutdown remains non-failing
- **Given** HTTP server 已成功启动
- **When** Fx app 停止并导致 `Serve` 返回 `http.ErrServerClosed`
- **Then** 系统 MUST NOT 将该错误记录为 HTTP server 失败
- **Then** 系统 MUST NOT 因该错误再次触发应用级 shutdown

#### Scenario: Startup listen failure still fails OnStart
- **Given** 配置中的 HTTP host 和 port 不可监听
- **When** Fx app 启动 HTTP server lifecycle
- **Then** `OnStart` MUST 返回包含监听失败上下文的错误
- **Then** 系统 MUST NOT 进入异步 serve 成功运行状态

### Requirement: Log HTTP startup runtime identity
HTTP 服务运行时 SHALL 在 HTTP server 启动日志中输出关键运行时身份上下文。`NewHTTPServer` 的启动日志 MUST 保留现有启动日志消息和监听地址字段，并 MUST 追加服务名、运行环境和系统时区字段。服务名 MUST 来自 `config.App.Name`，运行环境 MUST 来自 `config.App.Environment`，时区 MUST 来自 `config.System.Timezone`。该日志增强 MUST NOT 改变 HTTP server 监听、启动失败返回、异步 serve、正常关闭或 `http.ErrServerClosed` 处理语义。

#### Scenario: Startup log includes service identity context
- **Given** 用户服务配置包含 `app.name: aegiscore-user-services`、`app.environment: local` 和 `system.timezone: Asia/Shanghai`
- **When** HTTP server lifecycle `OnStart` 执行启动日志
- **Then** 启动日志 MUST 继续包含 HTTP 监听地址字段
- **Then** 启动日志 MUST 包含服务名字段且值为 `aegiscore-user-services`
- **Then** 启动日志 MUST 包含运行环境字段且值为 `local`
- **Then** 启动日志 MUST 包含时区字段且值为 `Asia/Shanghai`

#### Scenario: Startup log preserves existing HTTP server behavior
- **Given** HTTP server 按配置启动
- **When** 启动日志追加服务名、运行环境和时区字段
- **Then** HTTP server MUST 继续在 `OnStart` 中先绑定监听地址再返回成功
- **Then** 监听失败 MUST 继续向 Fx 返回错误
- **Then** `http.ErrServerClosed` MUST 继续不被记录为服务失败
- **Then** 路由注册、中间件顺序、健康检查、Swagger 暴露和优雅关闭行为 MUST 保持不变

#### Scenario: Empty runtime identity values are logged as configured
- **Given** 配置中的 `app.name`、`app.environment` 或 `system.timezone` 为空
- **When** HTTP server lifecycle `OnStart` 执行启动日志
- **Then** 启动日志 MUST 使用配置中的空值输出对应字段
- **Then** 系统 MUST NOT 因这些字段为空而在 `NewHTTPServer` 中执行额外配置校验
- **Then** 系统 MUST NOT 使用代码级默认服务名补齐启动日志字段

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

系统必须注册健康检查、用户 API 路由、Swagger 文档路由和共享 HTTP 中间件。HTTP 基础中间件必须先注入 trace-id，再执行 panic recovery、请求日志和 CORS；trace-id 必须来自 `X-Trace-ID` 请求头或由系统生成，并必须写入 Gin context、Go `context.Context` 和 `X-Trace-ID` 响应头。共享中间件必须对外提供 `TraceID()` Gin middleware。用户服务运行时 MUST 通过服务级 HTTP 路由入口注册完整用户服务 HTTP surface，该入口 MUST 使用明确表达用户服务 HTTP 范围的命名，并 MUST 按系统路由、Swagger 文档路由、版本化 API、公共认证路由、受保护认证路由和用户资源路由组织注册逻辑。用户服务运行时 MUST 将 `config.App.Name` 作为健康检查响应中的服务名来源传入系统路由，MUST NOT 在健康检查 handler 中硬编码服务名或设置代码级默认服务名；健康检查成功状态值 MUST 使用路由包内拥有的常量表达。用户服务运行时 MUST 通过路由局部分组控制认证中间件挂载：健康检查、Swagger 文档、登录、刷新和受限改密入口 MUST 保持公开访问；退出当前设备、退出全部设备和用户资料 API MUST 挂载认证中间件。用户服务运行时 MUST 在注册认证中间件时传入 Fx 注入的 Zap logger。请求日志的 `client_ip` 字段必须使用 Gin `Context.ClientIP()` 的结果。后续 Casbin 授权中间件 MUST 挂载在认证中间件之后、业务 handler 之前的受保护路由子分组中。

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
- **Then** 请求被路由到 `UserController.GetByUserID`

#### Scenario: Create user API route is registered under versioned prefix
- **Given** HTTP server 已启动
- **When** 调用方请求 `POST /api/v1/users`
- **Then** 请求被路由到 `UserController.CreateUser`

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
- **Then** HTTP 响应仍必须使用 `common/contract/response.Envelope` 失败格式

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

#### Scenario: User service HTTP route entrypoint is explicitly scoped
- **Given** 维护者查看 `user-services/internal/router` 中的路由注册入口
- **When** 入口函数注册用户服务完整 HTTP surface
- **Then** 入口名称 MUST 明确表达用户服务 HTTP 路由范围
- **Then** 实现 MUST NOT 使用仅表达泛化路由注册的名称承载完整用户服务 HTTP surface

#### Scenario: Route registration is grouped by API surface
- **Given** 用户服务注册 HTTP 路由
- **When** 实现组织 `user-services/internal/router` 包内路由注册逻辑
- **Then** 系统路由、Swagger 文档路由、版本化 API、公共认证路由、受保护认证路由和用户资源路由 MUST 有清晰分组边界
- **Then** 拆分 MUST 保持现有路径、HTTP 方法、handler 绑定和认证边界等价

### Requirement: Shutdown gracefully

系统必须在收到中断或 SIGTERM 后停止接受新请求，并在配置的 shutdown timeout 内关闭 HTTP server。用户服务声明的 Redis client 和 PostgreSQL 连接池必须随 Fx app stop 释放。HTTP server 在配置未提供有效 shutdown timeout 时 MUST 使用具名 HTTP graceful shutdown 默认超时常量，当前默认值 MUST 保持为 `10s`。CLI/Fx app stop budget MUST 是外层停止预算，不得小于用户服务默认 YAML 配置中的 `http.shutdown_timeout`，以避免默认配置的 HTTP graceful shutdown 被外层 stop context 提前截断。

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

#### Scenario: Default shutdown timeout is named
- **Given** HTTP 配置未提供有效 shutdown timeout
- **When** HTTP server stop hook 创建 graceful shutdown context
- **Then** 系统 MUST 使用具名 HTTP graceful shutdown 默认超时常量
- **Then** 默认关闭超时 MUST 保持为 `10s`

#### Scenario: Fx stop budget covers default HTTP shutdown config
- **Given** 用户服务默认 YAML 配置声明 `http.shutdown_timeout: 25s`
- **When** CLI 创建 Fx app stop context
- **Then** Fx app stop timeout MUST 大于或等于 `25s`
- **Then** 默认配置下 HTTP server graceful shutdown MUST NOT 因外层 Fx stop context 只有 `15s` 而被提前截断

#### Scenario: Shutdown timeout names are not ambiguous
- **Given** 维护者查看 `user-services/cmd/main.go` 和 `user-services/internal/bootstrap/server.go`
- **When** 维护者比较 CLI stop timeout 与 HTTP shutdown timeout fallback
- **Then** 常量名称 MUST 表达二者处于不同层级
- **Then** 维护者 MUST 能判断 CLI stop timeout 是整个 Fx app 停止预算，HTTP shutdown timeout 是 server graceful shutdown 预算

### Requirement: Preserve upstream context metadata during CLI stop

用户服务 CLI 停止 Fx app 时，系统 MUST 使用从 `runServe` 上游 context 派生的 stop root context，以保留调用方注入的 context values。该 stop root context MUST NOT 直接继承终止信号触发后的取消状态；Fx app stop context MUST 继续使用 `fxAppStopTimeout` 表达独立停止预算。

#### Scenario: Stop context preserves upstream values
- **Given** 调用方使用携带 context value 的上游 context 调用 `runServe`
- **When** CLI 收到 `os.Interrupt` 或 `SIGTERM` 并触发 Fx app stop
- **Then** Fx app stop hooks MUST 能通过 stop context 读取该上游 context value
- **Then** stop context MUST 继续使用 CLI/Fx app stop timeout 作为停止预算

#### Scenario: Stop context does not inherit signal cancellation
- **Given** 服务运行 context 因 `os.Interrupt` 或 `SIGTERM` 变为已取消
- **When** CLI 创建传给 Fx app stop hooks 的 stop context
- **Then** stop context MUST NOT 因该终止信号而已经处于取消状态
- **Then** stop hooks MUST 仍可在 `fxAppStopTimeout` 预算内执行清理逻辑

#### Scenario: Runtime surface remains unchanged
- **Given** CLI stop context 创建策略已调整
- **When** 用户服务通过 `aegiscore-user-services serve` 启动并停止
- **Then** CLI 命令名、`--config` 参数、HTTP 路由、响应信封、认证边界和 runtime 依赖初始化 MUST 保持不变
- **Then** HTTP server graceful shutdown MUST 继续使用现有配置和默认 timeout 规则

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
- **When** `common/runtime/config.Load` 反序列化运行时配置
- **Then** 用户服务 MUST 使用环境变量覆盖后的 timeout 值
- **Then** 配置加载器 MUST NOT 对 timeout 执行额外 required 或范围校验

### Requirement: HTTP runtime naming cleanup preserves service contract
HTTP 服务运行时相关命名标准化 SHALL 只修改内部组装名称、局部变量、内部类型或文档表达，不得改变 CLI、路由注册、中间件顺序、健康检查、Swagger 暴露或优雅关闭行为。用户服务 Fx 运行时组装模块的 Go 符号名称 MUST 明确表达进程级运行时装配职责，并 MUST NOT 使用容易与 `internal/service` 业务 service 层混淆的名称。

#### Scenario: Runtime module names are standardized
- **WHEN** 实现修改 `user-services/internal/bootstrap`、`user-services/internal/router` 或 `user-services/cmd` 中的内部命名
- **THEN** 服务启动命令、HTTP 路由、Fx 依赖关系和关闭流程 MUST 与修改前保持等价

#### Scenario: Fx app module name reflects composition root scope
- **Given** 维护者查看 `user-services/internal/bootstrap/app.go` 中的用户服务 Fx 模块定义
- **When** 该模块装配 timezone、validation、PostgreSQL、Redis、JWT、Ent、repository、service、controller、Gin engine、HTTP server 和路由注册
- **Then** Go 符号名称 MUST 使用 `AppModule` 表达应用组合根职责
- **Then** Go 符号名称 MUST NOT 使用 `UserServiceModule`

#### Scenario: Service identity name is reviewed
- **WHEN** 审查发现 `user-services` 复数命名或服务名语义可改进
- **THEN** 本变更 MUST 保留该名称，并将目录、module path、CLI 名或服务标识重命名视为单独 breaking change

### Requirement: Runtime composes concrete repository implementations at the bootstrap boundary
HTTP 服务运行时 SHALL 在 `user-services/internal/bootstrap` 组合根中装配具体 repository 实现。用户服务启动时 MUST 通过 `repository/postgres` provider 提供 `repository.UserRepository`，并通过 `repository/redis` provider 提供 `repository.AuthSessionRepository`，同时保持现有 `user_db` Ent client、`cache_redis` Redis client 和 auth 配置依赖不变。

#### Scenario: Bootstrap provides PostgreSQL user repository
- **Given** Fx app 装配用户服务依赖
- **When** bootstrap 创建用户仓储 provider
- **Then** bootstrap MUST 使用 `postgres.NewUserRepository`
- **Then** provider MUST 注入具名 `user_db` Ent client
- **Then** 下游 service MUST 接收 `repository.UserRepository` 抽象

#### Scenario: Bootstrap provides Redis auth session repository
- **Given** Fx app 装配用户服务依赖
- **When** bootstrap 创建认证会话仓储 provider
- **Then** bootstrap MUST 使用 `redis.NewAuthSessionRepository`
- **Then** provider MUST 注入具名 `cache_redis` Redis client、`repository.UserRepository` 和 auth 配置
- **Then** 下游 auth service 和认证中间件 MUST 接收 `repository.AuthSessionRepository` 抽象

#### Scenario: Startup dependencies remain unchanged
- **Given** 用户服务通过 CLI 启动
- **When** Fx app 初始化 runtime 依赖
- **Then** 系统 MUST 继续只初始化自身声明的 `cache_redis`、`user_db` 和 `common_db` 运行时依赖
- **Then** 系统 MUST NOT 因 repository 实现分包新增 Redis、PostgreSQL、Ent client 或 HTTP 路由依赖

### Requirement: Split user service bootstrap by runtime responsibility
系统 SHALL 将 `user-services/internal/bootstrap` 组合根按职责组织为聚焦源文件，至少区分 Fx app/module 装配、Gin engine 创建、认证 provider、路由注册和 HTTP server lifecycle。Fx app/module 装配文件 MUST 使用职责名称，例如 `app.go` 或 `fx.go`，拆分完成后 MUST NOT 保留 `bootstrap/bootstrap.go` 作为泛化聚合文件。拆分 MUST 保持服务启动命令、Fx provider/invoke 集合、Gin 中间件顺序、路由注册、认证中间件挂载和优雅关闭行为等价。

#### Scenario: Bootstrap file split preserves Fx app creation
- **Given** 调用方通过 `bootstrap.NewApp(configPath)` 创建用户服务 Fx app
- **When** bootstrap 代码按职责拆分为多个源文件
- **Then** `NewApp` MUST 继续提供 `commoninfra.ConfigPath(configPath)`、共享配置 provider 和 Zap logger provider
- **Then** `AppModule` MUST 继续引入 timezone 与 validation module
- **Then** Fx app/module 装配 MUST 位于 `app.go` 或 `fx.go` 等职责命名文件中
- **Then** 实现 MUST NOT 继续使用 `bootstrap.go` 承载泛化 bootstrap 聚合职责
- **Then** 用户服务声明的 provider 和 invoke 行为 MUST 与拆分前等价

#### Scenario: Middleware and routes remain equivalent
- **Given** HTTP server 启动并注册 Gin engine
- **When** Gin engine 创建和路由注册逻辑移动到聚焦文件
- **Then** trace-id、recovery、request logger 和 CORS 中间件顺序 MUST 保持不变
- **Then** 健康检查、Swagger、用户 API 和认证 API 路由 MUST 保持当前注册语义
- **Then** 受保护路由的认证中间件挂载 MUST 保持不变

#### Scenario: HTTP server lifecycle remains equivalent
- **Given** 用户服务创建 HTTP server
- **When** HTTP server 创建和 lifecycle hook 移动到聚焦文件
- **Then** server MUST 继续监听配置中的 host 和 port
- **Then** read、write、idle 和 shutdown timeout MUST 继续使用加载后的 HTTP 配置和默认关闭超时规则
- **Then** `http.ErrServerClosed` MUST 继续不被记录为服务失败

### Requirement: Use explicit controller handler bindings in route registration
用户服务 HTTP 运行时 SHALL 在路由注册中绑定明确表达业务动作的 controller handler 名称。实现 MUST 保持现有 HTTP 路径、方法、公开/受保护路由分组、认证中间件挂载顺序和响应行为不变。

#### Scenario: User routes bind explicit handlers
- **Given** 用户服务 HTTP 路由已注册
- **When** 开发者检查用户资源路由 handler 绑定
- **Then** `GET /api/v1/users` MUST 绑定 `UserController.ListUsers`
- **Then** `POST /api/v1/users` MUST 绑定 `UserController.CreateUser`
- **Then** `GET /api/v1/users/:user_id` MUST 继续绑定 `UserController.GetByUserID`

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
