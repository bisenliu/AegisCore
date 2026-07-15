## ADDED Requirements

### Requirement: 用户 feature 日志依赖显式化

user feature 的 application、HTTP 边界和关键 PostgreSQL infrastructure 在正式业务主路径记录日志时 MUST 使用 constructor 注入的 `*zap.Logger`、由注入 logger 派生的组件 logger，或 request context 中明确携带的 logger。正式主路径 MUST NOT 通过 package-level 默认 logger fallback 获取可变进程全局日志依赖。

#### Scenario: 用户 application 构造声明日志依赖
- **WHEN** 用户资料 command 或 query use case 需要记录应用日志
- **THEN** 其 constructor MUST 显式接收 `*zap.Logger` 或包含该 logger 的最小依赖结构
- **AND** 构造缺省处理 MUST 使用局部 nop logger 或 fail-fast 规则，不得调用 `logger.SetDefault` 或依赖构造函数隐式安装默认 logger

#### Scenario: 用户 request 日志使用 request context
- **WHEN** 用户 HTTP controller 或输入准备后的业务调用需要记录请求关联日志
- **THEN** 日志 MUST 使用当前 request context 中明确携带的 logger 或由注入 logger 结合 context 派生
- **AND** `request_id`、`trace_id` 和 `span_id` 关联 MUST 保持可用

#### Scenario: 用户 infrastructure 不依赖全局默认 logger
- **WHEN** 用户 PostgreSQL adapter 或其他关键 infrastructure 记录持久化错误、分页诊断或状态异常
- **THEN** logger MUST 从 adapter constructor 显式注入或由调用方 context 提供
- **AND** 生产文件 MUST NOT 通过 package-level `logger.Info`、`logger.Warn`、`logger.Error`、`logger.Debug` 或 `logger.NamedComponent(nil, ...)` 作为正式主路径日志来源

#### Scenario: 架构检查覆盖用户 feature
- **WHEN** 执行 `make user-service-architecture-lint`
- **THEN** 检查 MUST 能发现 user feature application 或关键 infrastructure 重新依赖 package-level 默认 logger 的生产代码
- **AND** 测试 fixture 使用局部 logger 或显式 context logger 时 MUST 不被误判为生产主路径违规
