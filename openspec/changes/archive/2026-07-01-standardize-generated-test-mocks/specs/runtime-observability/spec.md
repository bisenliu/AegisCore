## ADDED Requirements

### Requirement: Metrics no-op 生成规范

系统 MUST 通过业务中立生成能力或生成规范为 feature-local metrics interface 生成 no-op 实现。业务 metrics interface MUST 保留在所属 feature 边界内，`common/runtime/observability/metrics` MUST NOT 承载 user-service 的 auth、permission、role、user 或其他服务业务指标方法。

#### Scenario: 生成 feature metrics no-op
- **WHEN** feature 定义业务 `Metrics` interface 且需要默认空实现
- **THEN** 系统 MUST 通过统一生成流程生成 no-op 实现文件
- **AND** feature MAY 保留 `NopMetrics()` 作为本 feature 的空实现入口
- **AND** 生成文件 MUST 与对应 feature-local `Metrics` interface 编译匹配

#### Scenario: common 保持业务中立
- **WHEN** `common/runtime/observability/metrics` 提供 metrics no-op 生成能力或规范
- **THEN** 该能力 MUST 只处理 Go interface 签名和空方法生成等业务中立逻辑
- **AND** 该能力 MUST NOT 定义 auth 登录、refresh、RBAC policy reload、route diff 或任何 user-service 业务指标方法

#### Scenario: 指标运行时语义不变
- **WHEN** 手写 metrics no-op 实现迁移为生成文件
- **THEN** Prometheus metric family、label key、label value、低基数约束和 tracing/logging 语义 MUST 保持不变
- **AND** metrics 配置禁用时的 no-op 行为 MUST 继续不产生运行时副作用
