## ADDED Requirements

### Requirement: Tracing provider 生命周期所有权

系统 MUST 为 `common/runtime/observability/tracing` provider 定义单一启动和关闭所有权。普通 constructor MUST 在返回前立即启动底层 SDK provider，并由调用方显式 `Shutdown`；Fx adapter MUST 在 constructor 阶段只返回未启动但可安全注入的 facade，并且只允许 Fx `OnStart` 创建 exporter、batch processor 和真实 SDK provider，Fx `OnStop` 或 rollback MUST 关闭同一 provider 并恢复 no-op。

#### Scenario: 普通 constructor 立即启动
- **WHEN** 调用方使用普通 constructor 创建启用 tracing 的 provider
- **THEN** constructor MUST 在调用方 context 预算内创建 OTLP exporter、batch processor 和 SDK provider
- **AND** 返回成功后调用方 MUST 通过 `Shutdown(ctx)` 关闭该 provider
- **AND** constructor 失败时 MUST 返回包含 `create OTLP tracing exporter` 或等价可定位上下文的错误，并通过标准 wrapping 保留底层 cause

#### Scenario: Fx constructor 延迟启动
- **WHEN** Fx graph 构造 tracing provider
- **THEN** constructor MUST 返回非 nil provider 和可安全注入的 dynamic tracer provider
- **AND** constructor 阶段 MUST NOT 创建 exporter、连接 OTLP endpoint、启动 batch processor 或执行可能阻塞的 tracing 初始化
- **WHEN** Fx lifecycle 执行 `OnStart(ctx)`
- **THEN** provider MUST 使用同一个实例创建真实 SDK provider，并在启动 context 预算内完成

#### Scenario: Fx rollback 关闭已启动 provider
- **WHEN** tracing `OnStart` 已成功但后续 lifecycle hook 启动失败
- **THEN** Fx rollback MUST 调用同一 provider 的 `Shutdown(ctx)`
- **AND** provider MUST 关闭 SDK provider、batch processor 和 exporter，并将后续 dynamic tracer 使用恢复为 no-op
- **AND** 系统 MUST NOT 悬挂已关闭 provider 或保留旧 exporter

#### Scenario: 禁用 tracing 不连接 OTLP
- **WHEN** tracing 配置为 disabled
- **THEN** provider MUST 保持非 nil 且可注入
- **AND** 启动路径 MUST NOT 要求 OTLP endpoint、创建 OTLP exporter 或连接网络
- **AND** span 创建 MUST 安全返回 no-op 或 never-sampled span，且不改变调用方业务语义

### Requirement: Dynamic tracer 启停安全

constructor 阶段获取的 dynamic tracer provider 和 tracer MUST 在 provider 启动前、启动后、Shutdown 后都可安全使用。启动前和 Shutdown 后 MUST 使用 no-op provider；启动后 MUST 委托当前真实 SDK provider；该切换 MUST NOT 安装 OpenTelemetry global provider，也 MUST NOT 要求 instrumentation 重新获取 tracer。

#### Scenario: 启动前 dynamic tracer 安全 no-op
- **WHEN** Redis、Gin、Ent 或其他 instrumentation 在 constructor 阶段保存 dynamic tracer 或 tracer provider
- **THEN** provider 尚未启动时 span 创建 MUST 返回安全 no-op span
- **AND** 调用方 MUST NOT 因 tracing 未启动而 panic、阻塞或连接 exporter

#### Scenario: 启动后 dynamic tracer 使用真实 provider
- **WHEN** 同一个 provider 在 lifecycle 中成功启动
- **THEN** constructor 阶段已保存的 dynamic tracer MUST 使用当前真实 SDK provider 创建 span
- **AND** 调用方 MUST NOT 重新安装 instrumentation 或重新获取 tracer provider 才能获得真实 span

#### Scenario: Shutdown 后恢复 no-op
- **WHEN** provider 已执行 `Shutdown(ctx)` 并关闭底层 SDK provider
- **THEN** 既有 dynamic tracer 后续 span 创建 MUST 回退到 no-op
- **AND** 系统 MUST NOT 使用已关闭的 SDK provider、batch processor 或 exporter

#### Scenario: 传播器保持稳定
- **WHEN** 调用方通过 provider 获取 `TextMapPropagator`
- **THEN** propagator MUST 支持 W3C trace context 与 baggage 的 inject/extract
- **AND** propagator 行为 MUST NOT 依赖 provider 是否已连接 OTLP exporter

### Requirement: Tracing lifecycle 重复调用语义

tracing provider MUST 对重复或非法 lifecycle 调用提供明确且被测试的结果。重复启动同一 provider MUST 失败并保持既有已启动 provider 不变；`Shutdown(ctx)` 对 nil provider、未启动 provider 和已关闭 provider MUST 幂等成功；非法启动输入 MUST 返回明确错误。

#### Scenario: 重复启动不得泄漏旧 provider
- **WHEN** 同一个 provider 已成功启动后再次执行启动逻辑
- **THEN** 第二次启动 MUST 返回可识别错误
- **AND** 系统 MUST 保持第一次启动的 SDK provider 仍为当前 provider
- **AND** 系统 MUST NOT 静默替换、丢失或泄漏旧 exporter、batch processor 或 SDK provider

#### Scenario: Shutdown 幂等
- **WHEN** provider 为 nil、从未启动或已经关闭
- **THEN** `Shutdown(ctx)` MUST 返回 nil
- **AND** 重复 Shutdown MUST NOT panic、阻塞或重复关闭同一 exporter

#### Scenario: 非法启动输入失败
- **WHEN** 启动逻辑收到 nil context、nil provider、缺失 resource 或启用 tracing 但缺失 exporter factory
- **THEN** 启动 MUST 返回明确错误
- **AND** provider MUST 保持未启动或保持原有已启动 provider 不变
