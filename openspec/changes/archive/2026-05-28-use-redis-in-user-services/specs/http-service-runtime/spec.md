## MODIFIED Requirements

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
