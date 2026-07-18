## ADDED Requirements

### Requirement: Fx DI 初始化边界保护
user-service composition root MUST 启用 Fx constructor、decorator 和 Invoke 范围内的 panic recovery，并 MUST 将其定位为 DI 初始化边界保护。可预期的资源、配置和依赖错误 MUST 优先通过 constructor 返回 `error` 暴露，MUST NOT 依赖 panic recovery 表达正常失败路径。

#### Scenario: constructor panic 转换为 Fx 错误
- **WHEN** Fx 在 user-service composition root 中执行 constructor、decorator 或 Invoke 时发生未预期 panic
- **THEN** App 构造或启动 MUST 通过 Fx error 暴露 panic 信息
- **AND** 进程 MUST NOT 因该 DI 初始化 panic 直接崩溃

#### Scenario: recovery 范围受限
- **WHEN** HTTP handler、worker task、后台 goroutine 或 lifecycle hook 运行期发生 panic
- **THEN** `fx.RecoverFromPanics()` MUST NOT 被视为这些运行期边界的恢复策略
- **AND** 对应边界 MUST 使用其自身已有或显式设计的 panic 处理机制

### Requirement: tracing provider constructor 可用性
Fx tracing provider MUST 在 constructor 阶段提供非 nil tracer provider，使 Redis、Gin、Ent 和其他依赖 tracing 的 constructor 可以使用同一 service runtime config 初始化 instrumentation。tracing provider MUST 继续在 Fx lifecycle stop 阶段关闭，并保持禁用 tracing 时的 no-op 或 `NeverSample` 语义。

#### Scenario: constructor 阶段使用 tracing provider
- **WHEN** user-service Fx graph 构造依赖 tracing 的 Redis、Gin 或 Ent provider
- **THEN** tracing provider 的 `TracerProvider()` MUST 返回非 nil 值
- **AND** 这些 provider MUST 使用服务级 tracing provider，不得静默回退到全局 provider

#### Scenario: tracing 构造失败
- **WHEN** tracing 配置缺失服务名、环境、非法采样率，或启用 tracing 但缺少 OTLP endpoint
- **THEN** Fx graph MUST 返回明确构造错误
- **AND** 系统 MUST NOT 延迟到 Redis、Gin、Ent 或 HTTP server 初始化时才暴露该配置错误

#### Scenario: Redis instrumentation 失败
- **WHEN** Redis tracing instrumentation 返回错误
- **THEN** Redis client constructor MUST 返回包含 `instrument redis tracing` 的错误并关闭已创建 client
- **AND** user-service cache Redis provider MUST 包装资源名并通过 Fx error path 传播该错误，MUST NOT panic
