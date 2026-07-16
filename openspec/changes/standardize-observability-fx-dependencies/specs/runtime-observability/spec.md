## ADDED Requirements

### Requirement: 共享 observability Fx provider 组合语义

共享 metrics 与 tracing Fx provider MUST 能通过普通强类型依赖完成装配，并保持现有配置来源、service/environment 标识、disabled/no-op、错误传播和 shutdown 行为不变。metrics Fx provider MUST 从共享 runtime config 构造 metrics `Options` 并拒绝 nil config；tracing Fx provider MUST 从共享 runtime config 构造 provider，传播构造错误，并通过 `fx.Lifecycle` 注册 `OnStop: provider.Shutdown`。`NewFxProvider` MUST 保留从共享 runtime config 投影 service name、environment、metrics/tracing config 和 lifecycle 的 composition 职责，MUST NOT 退化为 `NewProvider` 的无语义别名。

#### Scenario: metrics Fx provider 通过共享配置装配
- **WHEN** Fx graph 调用共享 metrics `NewFxProvider` 且提供有效的 `*config.Config`
- **THEN** provider MUST 从共享 runtime config 投影 service name、environment 和 metrics 配置
- **AND** provider MUST 返回符合配置的 metrics `*Provider`
- **AND** metrics 配置禁用时 MUST 返回非 nil disabled provider

#### Scenario: metrics Fx provider 拒绝 nil config
- **WHEN** 共享 metrics `NewFxProvider` 收到 nil `*config.Config`
- **THEN** provider MUST 返回明确错误
- **AND** 系统 MUST NOT 用默认配置静默构造 metrics provider

#### Scenario: tracing Fx provider 保留 lifecycle shutdown
- **WHEN** Fx graph 调用共享 tracing `NewFxProvider` 且提供有效 `fx.Lifecycle` 与 `*config.Config`
- **THEN** provider MUST 从共享 runtime config 投影 service name、environment 和 tracing 配置
- **AND** provider 构造错误 MUST 传播给 Fx graph
- **AND** provider MUST 注册 `OnStop: provider.Shutdown` hook
- **AND** tracing 配置禁用时 MUST 返回非 nil provider 并使用 no-op 或 `NeverSample` 语义

#### Scenario: tracing provider 仍允许依赖 Fx lifecycle
- **WHEN** 共享 tracing Fx provider 需要在 Fx app 停止时关闭 exporter 或 provider 资源
- **THEN** provider MAY 直接依赖 `fx.Lifecycle`
- **AND** 系统 MUST NOT 将 shutdown lifecycle 移到 user-service 调用方
- **AND** 系统 MUST NOT 依赖全局 tracer shutdown 或新增 package-level mutable state

### Requirement: user-service Ent observability 依赖必需性

user-service 正式 Ent provider MUST 消费非 optional 的共享 metrics 与 tracing provider。`providers.Module` MUST 始终注册共享 metrics/tracing Fx provider；缺失任一 provider 时，正式 Fx graph MUST 构图失败。metrics 或 tracing 被配置禁用时，系统 MUST 通过非 nil disabled/no-op provider 表达禁用语义，MUST NOT 依赖 nil metrics/tracing 或 optional tag 作为正式降级路径。

#### Scenario: 正式 Ent provider 消费非 optional observability provider
- **WHEN** user-service `providers.Module` 构建正式 Ent client graph
- **THEN** `NamedEntClientParams` MUST 要求非 optional 的 `*commonmetrics.Provider`
- **AND** `NamedEntClientParams` MUST 要求非 optional 的 `*commontracing.Provider`
- **AND** 缺失任一 provider 时 Fx graph 校验 MUST 失败

#### Scenario: disabled observability 通过非 nil provider 表达
- **WHEN** metrics 或 tracing 配置为 disabled
- **THEN** 共享 Fx provider MUST 仍向 Ent provider 注入非 nil provider
- **AND** Ent provider MUST 通过该 provider 的 disabled/no-op 语义工作
- **AND** 正式 `providers.Module` MUST NOT 通过 nil provider 或 optional tag 表达禁用状态

#### Scenario: Ent nil fallback 限定为直接构造防御
- **WHEN** Ent provider 的纯函数或直接构造测试绕过正式 `providers.Module`
- **THEN** 实现 MAY 保留 nil metrics/tracing fallback 作为防御或测试 seam
- **AND** 该 fallback MUST NOT 成为正式 Fx graph 的降级机制
