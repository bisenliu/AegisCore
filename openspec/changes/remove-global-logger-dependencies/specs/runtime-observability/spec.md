## ADDED Requirements

### Requirement: 正式 App logger 生命周期与显式来源

user-service 正式 App MUST 使用 Fx 装配出的服务级 `*zap.Logger` 作为运行时日志依赖来源，并由 logger provider 在 App Stop 阶段同步该正式 logger。正式 App MUST NOT 通过安装、恢复或持有进程级默认 logger 来表达 logger lifecycle；request lifecycle 中的日志关联 MUST 继续通过明确写入 request context 的 logger 和 request ID context 传播。

#### Scenario: App Stop 同步正式 logger
- **WHEN** user-service Fx App 停止并执行 logger provider 的 `OnStop` hook
- **THEN** 系统 MUST 对服务级正式 logger 执行既有 `Sync` 责任
- **AND** stdout/stderr 不支持 fsync 的平台错误 MUST 继续按既有规则忽略

#### Scenario: 正式 App 不安装默认 logger
- **WHEN** user-service 正式 App 构造或启动 logger provider
- **THEN** provider MUST NOT 调用 `logger.SetDefault` 或等价逻辑安装进程级默认 logger
- **AND** App Stop MUST NOT 恢复旧默认 logger 或持有默认 logger restore 状态

#### Scenario: 并行 App logger 隔离
- **WHEN** 同一进程并行或连续构造多个 user-service 测试 App
- **THEN** 每个 App 的 feature、middleware 和 provider 日志 MUST 来源于自身注入的服务级 logger 或 request context logger
- **AND** 一个 App 构造的 logger MUST NOT 覆盖另一个 App 通过默认 logger fallback 观察到的实例

#### Scenario: request context 日志关联保持不变
- **WHEN** HTTP 请求经过 request ID 和 tracing 相关 middleware 并进入业务处理
- **THEN** 请求生命周期内通过明确 context logger 记录的日志 MUST 继续包含有效的 `request_id`、`trace_id` 和 `span_id`
- **AND** 本变更 MUST NOT 修改 `X-Request-ID`、W3C `traceparent` 或 `tracestate` 的外部传播契约

#### Scenario: 观测输出契约保持不变
- **WHEN** logger 依赖来源从进程默认迁移为显式注入或 request context
- **THEN** 日志 message、level、logger name、`component`、`service`、`env`、`request_id`、`trace_id`、`span_id` 和敏感信息过滤语义 MUST 保持不变
- **AND** 系统 MUST NOT 修改 metrics、tracing、OpenAPI、pprof、Prometheus alert 或 Grafana dashboard 契约
