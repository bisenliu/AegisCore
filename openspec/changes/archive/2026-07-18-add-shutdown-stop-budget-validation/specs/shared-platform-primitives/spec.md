## ADDED Requirements

### Requirement: Runtime lifecycle 停止预算校验

系统 MUST 在 `common/runtime/config` 中校验 `runtime.lifecycle.stop_timeout` 覆盖共享 runtime 已知的串行停止路径最低预算，并 MUST 在预算不足时于配置加载阶段返回包含最低所需时长的错误。该校验 MUST 保持业务中立，不得在 `common` 中引入 auth、RBAC、user-service 资源名或 feature 语义。

#### Scenario: 停止总预算不足

- **WHEN** 配置中的 `runtime.lifecycle.stop_timeout` 小于 HTTP shutdown timeout、worker drain allowance、tracing flush allowance 和 shutdown safety margin 的组合最低预算
- **THEN** 配置校验 MUST 失败
- **AND** 错误信息 MUST 指出 `runtime.lifecycle.stop_timeout` 以及最低所需预算

#### Scenario: 停止总预算满足组合下限

- **WHEN** 配置中的 `runtime.lifecycle.stop_timeout` 大于或等于组合最低预算
- **THEN** 共享 runtime 配置校验 MUST 继续通过该 lifecycle 校验
- **AND** 既有 HTTP、gRPC、metrics、tracing、logger、resource 和 local cache 校验语义 MUST 保持不变

#### Scenario: workerpool 停止预算保持业务中立

- **WHEN** 调用方需要把 feature worker drain 纳入 App stop 总预算
- **THEN** `common/runtime/workerpool` MUST 只提供显式 `Stop(ctx)` drain 语义和错误传播
- **AND** auth purge、refresh session、Redis key 或其他业务停止策略 MUST 由 owning feature 或服务组合层表达
