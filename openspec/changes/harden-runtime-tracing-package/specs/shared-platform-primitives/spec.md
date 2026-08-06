## ADDED Requirements

### Requirement: Tracing primitive 公开 API 与实现边界

`common/runtime/observability/tracing` MUST 作为业务中立的共享 runtime primitive 维护清晰的公开 API 和包内职责边界。公开入口 MUST 表达 tracing 能力、构造、传播和关闭职责；仅供 lifecycle adapter 使用的启动状态机 MUST 留在包内，除非存在经规格确认的跨服务调用需求。

#### Scenario: 公开入口表达真实运行时职责
- **WHEN** 调用方需要创建或消费 tracing provider
- **THEN** 公开 API MUST 优先使用 `Options`、`Provider`、普通 constructor、Fx tracing provider constructor、`Tracer`、`OTelTracerProvider`、`TextMapPropagator` 和 `Shutdown`
- **AND** 公开名称 MUST 表达 tracing 能力语义，MUST NOT 以缺少能力语义的通用 Fx 名称作为主要入口

#### Scenario: Start 暴露面收窄
- **WHEN** 启动状态机只被 tracing 包内 lifecycle 使用
- **THEN** 该启动函数 MUST 保持未导出或包内可见
- **AND** 包外调用方 MUST 使用普通 constructor 表达立即启动所有权，或使用 Fx adapter 表达延迟启动所有权
- **AND** 系统 MUST NOT 保留绕过所有权边界的兼容双生命周期路径

#### Scenario: 包内文件职责聚焦
- **WHEN** 维护 tracing package 实现
- **THEN** provider facade、resource 构造、OTLP exporter、dynamic tracer、lifecycle 状态机、Fx adapter、包文档和 examples MUST 分别放置在聚焦文件中
- **AND** Fx adapter MUST 只负责配置映射和 hook 注册，MUST NOT 承载 resource、exporter 或 dynamic tracer 细节
- **AND** tracing primitive MUST NOT 导入 user-service feature、router、bootstrap、服务私有配置包、部署资产或业务 DTO

#### Scenario: 文档和示例不连接真实 endpoint
- **WHEN** tracing package 提供 `doc.go` 和 executable examples
- **THEN** 示例 MUST 覆盖 disabled provider、span 创建、propagator inject/extract 和显式 `Shutdown`
- **AND** 示例 MUST NOT 连接真实 OTLP endpoint、修改 OpenTelemetry global provider 或依赖外部网络服务
