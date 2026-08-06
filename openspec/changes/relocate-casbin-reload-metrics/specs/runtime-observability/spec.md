## ADDED Requirements

### Requirement: 通用 metrics provider 与 feature metrics 所有权分离

系统 MUST 保持通用 metrics Provider、Prometheus registry、metrics endpoint、HTTP metrics middleware、runtime collector 和 component status collector 的跨服务边界，同时 MUST 让带业务语义的 feature metrics 由 owning feature 或服务级 adapter 拥有。permission/RBAC metrics MAY 复用通用 Provider 注册 collector，但其 recorder interface、指标定义和空实现 MUST 留在 permission 边界。

#### Scenario: 通用 provider 继续支撑 feature collector 注册

- **WHEN** metrics 启用且 permission feature 注册 Casbin reload collector
- **THEN** collector MUST 通过通用 metrics Provider 注册到同一 Prometheus registry
- **AND** `/metrics` endpoint、service/environment label 约束和 provider enabled/disabled 语义 MUST 保持不变

#### Scenario: disabled metrics 不暴露 collector

- **WHEN** metrics 禁用
- **THEN** 系统 MUST NOT 暴露 metrics endpoint 或注册 Casbin reload collector
- **AND** permission feature MUST 继续获得非 nil feature-local no-op recorder，运行时行为和依赖图 MUST 保持稳定

#### Scenario: component status collector 不承载 feature 指标

- **WHEN** user-service 通过 component status collector 暴露运行时组件状态
- **THEN** collector MUST 继续只表达业务中立的 running 和 last error 状态
- **AND** Casbin reload 计数、last success gauge、RBAC watcher 专用指标和 outbox 指标 MUST 由 permission feature metrics 拥有

#### Scenario: 架构门禁防止业务指标回流 common

- **WHEN** 运行 `make user-service-architecture-lint`
- **THEN** 检查 MUST 在 `common/runtime/observability/metrics` 出现 Casbin、permission、role、RBAC、user-service 或 `aegiscore_casbin` 业务 metrics 语义时失败
- **AND** 该门禁 MUST 不禁止 common 保留通用 Provider、通用 label、HTTP metrics 和 component status collector
