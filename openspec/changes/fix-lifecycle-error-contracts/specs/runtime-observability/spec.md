## MODIFIED Requirements

### Requirement: Tracing 与依赖观测

系统 MUST 通过最小 OTLP 配置提供 OpenTelemetry tracing，并为 Redis 命令、Ent 查询和 HTTP 请求传播上下文；禁用 tracing 时 MUST 保持非 nil no-op 语义。启用 tracing 时，Fx provider MUST 在 lifecycle `OnStart(ctx)` 中初始化 exporter 和 SDK provider，并在 `OnStop(ctx)` 中关闭，MUST NOT 在 `fx.New` constructor 阶段连接 exporter。

#### Scenario: tracing 启停

- **WHEN** tracing 关闭
- **THEN** provider MUST 使用 no-op 或 `NeverSample` 语义且不连接 exporter
- **WHEN** tracing 开启且 Fx app 执行 `OnStart(ctx)`
- **THEN** provider MUST 使用服务名、环境和 OTLP endpoint 初始化 exporter 与 SDK provider
- **AND** exporter 初始化 MUST 使用 lifecycle 启动 context，受 Fx 启动预算、取消和超时控制
- **AND** lifecycle 停止时 provider MUST 使用停止 context 关闭 SDK provider 和 exporter 资源

#### Scenario: tracing exporter 构造失败

- **WHEN** tracing 开启且 OTLP exporter 构造失败
- **THEN** `OnStart(ctx)` MUST 返回包含 `create OTLP tracing exporter` 语义的错误
- **AND** 返回错误 MUST 通过标准错误 wrapping 保留底层 gRPC、TLS、endpoint 或 context cause
- **AND** 系统 MUST NOT 将底层 cause 替换为无 cause 的新错误

#### Scenario: constructor 阶段不连接 exporter

- **WHEN** Fx graph 在 `fx.New` constructor 阶段构造 tracing provider
- **THEN** provider 对象 MUST 可注入给依赖方
- **AND** provider MUST NOT 连接 OTLP exporter 或执行可能阻塞的 exporter 初始化
- **AND** 依赖方 MUST NOT 要求 `TracerProvider()` 在 `OnStart` 前已经连接真实 exporter

#### Scenario: Redis 命令 span

- **WHEN** user-service 执行 Redis 命令
- **THEN** 系统 MUST 创建低风险属性的 span 并传播服务 tracing provider
- **AND** span MUST NOT 记录完整 key、参数、token、密码或连接 secret

#### Scenario: Redis metrics 探测取消

- **WHEN** metrics HTTP scrape context 被取消
- **THEN** Redis PING MUST 尽快终止
- **WHEN** collector 经标准 `Collect` 直接调用
- **THEN** MUST 使用 background context 与 collector timeout，不得声称感知 HTTP 取消
- **AND** 最小探测间隔、快照复用及 `aegiscore_redis_*` 指标契约 MUST 保持不变

#### Scenario: Ent 查询观测

- **WHEN** Ent 执行查询
- **THEN** 系统 MUST 产生 span，并记录低基数 latency 与 error metrics
- **AND** 观测 MUST NOT 修改 SQL、事务、schema、查询返回值或错误语义
