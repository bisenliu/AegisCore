## ADDED Requirements

### Requirement: user-service 进程生命周期超时配置

系统 MUST 支持通过 YAML 配置文件声明 `aegiscore-user-services serve` 的 Fx app 进程级启动和关闭超时。`runtime.lifecycle.start_timeout` MUST 控制 `app.Start` 的最长等待时间，`runtime.lifecycle.stop_timeout` MUST 控制收到 `SIGINT` 或 `SIGTERM` 后 `app.Stop` 的最长等待时间。未声明这些字段时，系统 MUST 使用默认正数超时；显式配置非正数超时时，系统 MUST 拒绝启动并返回配置校验错误。

`user-service/cmd/serve.go` MUST NOT 定义 Fx app lifecycle timeout 默认常量；默认值 MUST 由共享配置层统一提供，serve 命令只消费已加载并校验过的配置值。

#### Scenario: 使用默认生命周期超时启动服务

- **WHEN** 协作者使用未声明 `runtime.lifecycle.start_timeout` 和 `runtime.lifecycle.stop_timeout` 的配置文件执行 `aegiscore-user-services serve`
- **THEN** 系统 MUST 使用默认 Fx app 启动和关闭超时
- **AND** 默认 `runtime.lifecycle.stop_timeout` MUST 大于或等于默认 `server.http.shutdown_timeout` 和默认 `server.grpc.shutdown_timeout`

#### Scenario: 使用配置化生命周期超时启动服务

- **WHEN** 配置文件声明 `runtime.lifecycle.start_timeout: 60s` 和 `runtime.lifecycle.stop_timeout: 120s`
- **THEN** `aegiscore-user-services serve` MUST 使用 `60s` 作为 Fx app 启动 context timeout
- **AND** 收到 `SIGINT` 或 `SIGTERM` 后 MUST 使用 `120s` 作为 Fx app 停止 context timeout
- **AND** 停止阶段 MUST 保持未被信号取消的上游 context value 传递语义

#### Scenario: 拒绝无效生命周期超时

- **WHEN** 配置文件声明非正数 `runtime.lifecycle.start_timeout` 或 `runtime.lifecycle.stop_timeout`
- **THEN** 系统 MUST 拒绝加载配置并返回可定位的配置校验错误

#### Scenario: 总关闭预算覆盖协议关闭预算

- **WHEN** 配置文件声明 `runtime.lifecycle.stop_timeout` 小于 `server.http.shutdown_timeout` 或 `server.grpc.shutdown_timeout`
- **THEN** 系统 MUST 拒绝加载配置并返回可定位的配置校验错误

#### Scenario: 不改变协议和业务行为

- **WHEN** 仅新增或调整 `runtime.lifecycle.start_timeout` 与 `runtime.lifecycle.stop_timeout`
- **THEN** 系统 MUST NOT 改变 HTTP API、OpenAPI、数据库 schema、RBAC、认证会话、metrics 指标契约或 HTTP/gRPC server shutdown timeout 的语义

#### Scenario: CLI 层不保留默认常量

- **WHEN** 系统实现 runtime lifecycle timeout 配置
- **THEN** `user-service/cmd/serve.go` MUST NOT 保留 `fxAppStartTimeout` 或 `fxAppStopTimeout` 默认常量
- **AND** serve 命令 MUST 使用配置 loader 返回的 lifecycle timeout 构造 `app.Start` 和 `app.Stop` context
