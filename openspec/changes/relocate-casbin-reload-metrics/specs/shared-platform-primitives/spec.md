## ADDED Requirements

### Requirement: Runtime metrics 保持业务中立

`common/runtime/observability/metrics` MUST 只提供跨服务通用的 metrics Provider、registry 注册、稳定 label 常量、HTTP metrics 支撑和业务中立 collector。该包 MUST NOT 定义、注册或导出 Casbin policy reload、permission、role、RBAC、user-service 或其他单一服务业务指标的接口、空实现、constructor、collector 或指标名称。

#### Scenario: common metrics 不拥有 Casbin reload 指标

- **WHEN** 系统编译或审查 `common/runtime/observability/metrics`
- **THEN** 该包 MUST NOT 包含 `ReloadMetrics`、`NopReloadMetrics`、`NewCasbinPolicyReloadMetrics` 或 `aegiscore_casbin_*` 指标定义
- **AND** 该包 MUST NOT 通过业务命名的 no-op recorder 为 user-service permission/RBAC 运行时提供依赖

#### Scenario: 通用 component status collector 保留

- **WHEN** 服务需要导出业务中立运行时组件的 running 和 last error 状态
- **THEN** 系统 MUST 继续复用 `common/runtime/observability/metrics` 的 component status collector
- **AND** collector MUST 只使用稳定 resource label 与只读状态接口，不得内置 Casbin、permission、role 或 user-service 组件名

#### Scenario: 业务指标归属调用方

- **WHEN** 消费服务或 feature 需要记录带业务语义的 metrics
- **THEN** 业务指标接口、空实现、指标名称和 collector MUST 由 owning service、feature 或其 observability adapter 拥有
- **AND** owning 代码 MAY 复用通用 metrics Provider 注册 collector，但 MUST NOT 把业务 recorder 下沉到 `common`
