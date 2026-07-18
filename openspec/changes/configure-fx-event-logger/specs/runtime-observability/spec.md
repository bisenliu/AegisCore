## ADDED Requirements

### Requirement: Fx 初始化事件使用统一结构化日志

系统 MUST 将 user-service 正式 Fx App 的 Fx event logger 接入当前 App 注入的结构化 `*zap.Logger`，并使用与业务日志一致的 encoder、字段和输出目标记录 Fx 构图、Invoke、constructor、decorator、stub、rollback、lifecycle 和 module trace 事件。

#### Scenario: 正式 App 构造时启用 Fx event logger

- **WHEN** user-service 通过 `AppOptions` 或 `NewApp` 构建正式 Fx App
- **THEN** Fx event logger MUST 由已注入的 `*zap.Logger` 构造
- **AND** Fx 自身事件 MUST 使用命名 logger 输出到统一结构化日志链路

#### Scenario: Fx event 日志级别

- **WHEN** Fx 记录常规构图、执行前后、module trace 或 lifecycle 事件
- **THEN** 系统 MUST 使用 debug 级别记录这些事件
- **WHEN** Fx 记录构造、Invoke、rollback 或 lifecycle 失败事件
- **THEN** 系统 MUST 使用 error 级别记录失败事件

#### Scenario: Fx event logger 保持快速非阻塞

- **WHEN** Fx 调用 event logger 记录初始化或 lifecycle 事件
- **THEN** event logger MUST 只执行本地 logger adapter 逻辑
- **AND** event logger MUST NOT 在 `LogEvent` 路径执行网络 I/O、远程导出、阻塞式重试或业务副作用

#### Scenario: logger 生命周期语义保持不变

- **WHEN** Fx App 停止并释放共享 logger
- **THEN** 系统 MUST 继续由 logger provider 的 Stop hook 同步当前 App logger
- **AND** Fx event logger MUST NOT 替换进程级默认 logger 或引入额外同步生命周期

### Requirement: Fx constructor 阶段 tracing provider 可用

系统 MUST 在依赖 tracing 的 Redis、Gin、Ent 等 user-service provider constructor 执行前，向 Fx 依赖图提供非 nil 且底层 `TracerProvider()` 可用的 tracing provider，并在 Fx 停止或启动 rollback 时关闭该 provider。

#### Scenario: constructor 阶段消费 tracing provider

- **WHEN** user-service Fx graph 构造 Redis、Gin 或 Ent provider
- **THEN** tracing provider MUST 已经返回非 nil `TracerProvider()`
- **AND** 依赖 tracing 的 constructor MUST NOT 因 tracing provider 尚未进入 `OnStart` 而失败

#### Scenario: 后续启动 hook 失败时释放 tracing provider

- **WHEN** tracing provider 已经构造成功且后续 Fx lifecycle hook 启动失败
- **THEN** Fx rollback MUST 调用 tracing provider shutdown
- **AND** shutdown 后 tracing provider 的底层 `TracerProvider()` MUST 被清空
