## ADDED Requirements

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
