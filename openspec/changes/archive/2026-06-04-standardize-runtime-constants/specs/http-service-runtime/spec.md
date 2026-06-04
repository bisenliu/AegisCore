## MODIFIED Requirements

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
- **Then** 用户服务不得通过 `common/infrastructure.Module` 注入公共依赖
- **Then** 用户服务必须手动依次提供公共配置、Zap logger 和自身声明的运行时依赖

#### Scenario: CLI lifecycle timeouts are named separately
- **Given** CLI 启动或停止 Fx app
- **When** 实现创建 start context 和 stop context
- **Then** 启动 context MUST 使用表达 Fx app start budget 的启动超时常量
- **Then** 停止 context MUST 使用表达 Fx app stop budget 的停止超时常量
- **Then** 停止超时常量名称 MUST NOT 暗示它只控制 HTTP server shutdown

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
