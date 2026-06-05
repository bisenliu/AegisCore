## ADDED Requirements

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
