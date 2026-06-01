## ADDED Requirements

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
