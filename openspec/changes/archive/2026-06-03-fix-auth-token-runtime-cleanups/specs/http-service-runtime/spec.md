## MODIFIED Requirements

### Requirement: Start service through CLI

系统必须提供 `aegiscore-user-services serve` 命令，并允许通过 `--config` 指定 YAML 配置路径。用户服务启动时必须在自身 Fx 装配中显式提供公共配置和 Zap logger，并初始化自己声明的运行时依赖，包括 HTTP server、具名 PostgreSQL 连接池和具名 Redis client。CLI 启动和停止 Fx app 的 timeout MUST 使用语义独立的具名常量表达，即使当前默认值相同也 MUST 分别维护启动超时和停止超时。

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
- **Then** 启动 context MUST 使用启动超时常量
- **Then** 停止 context MUST 使用停止超时常量
- **Then** 两个常量当前值 MUST 均保持为 `15s`

### Requirement: Shutdown gracefully

系统必须在收到中断或 SIGTERM 后停止接受新请求，并在配置的 shutdown timeout 内关闭 HTTP server。用户服务声明的 Redis client 和 PostgreSQL 连接池必须随 Fx app stop 释放。HTTP server 在配置未提供有效 shutdown timeout 时 MUST 使用具名默认关闭超时常量，当前默认值 MUST 保持为 `10s`。

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
- **Then** 系统 MUST 使用具名默认关闭超时常量
- **Then** 默认关闭超时 MUST 保持为 `10s`
