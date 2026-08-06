## ADDED Requirements

### Requirement: Casbin reload metrics 归属 permission feature

Casbin policy reload recorder MUST 由 user-service permission/RBAC 边界拥有。permission feature MUST 定义 Engine 消费的最小 reload recorder interface、disabled metrics 时使用的非 nil no-op 实现，以及使用通用 metrics Provider 注册 Prometheus collector 的实现。该 recorder MUST NOT 位于 `common/runtime/observability/metrics`、`user-service/internal/shared` 或 role feature。

#### Scenario: Engine 使用 permission-owned recorder

- **WHEN** permission composition 构造 Casbin Engine
- **THEN** Engine MUST 接收 permission-owned reload recorder interface
- **AND** Engine MUST NOT import `common/runtime/observability/metrics` 以获得 Casbin reload 业务接口
- **AND** Engine 的授权、reload、health 和 initialization 投影 MUST 继续指向同一个 engine 实例

#### Scenario: 指标名称和语义保持不变

- **WHEN** policy reload 成功或失败
- **THEN** recorder MUST 分别增加 `aegiscore_casbin_policy_reloads_total{status="success"}` 或 `aegiscore_casbin_policy_reloads_total{status="failure"}`
- **AND** recorder MUST 使用 `aegiscore_casbin_policy_reload_last_success` 以 `1` 表示最近 reload 成功，以 `0` 表示最近 reload 失败
- **AND** 指标 MUST NOT 增加用户、角色、权限、revision、Redis key、SQL、原始错误或其他高基数 label

#### Scenario: metrics 禁用时使用安全空实现

- **WHEN** 全局 metrics provider 缺失或禁用
- **THEN** permission feature MUST 为 Casbin Engine 注入非 nil no-op reload recorder
- **AND** no-op recorder MUST 保持 reload、watcher、initializer 和授权 fail-closed 行为不变，不得注册 collector 或引入 nil 分支

#### Scenario: 不改变 policy reload 行为

- **WHEN** Casbin policy 初始加载、显式 reload、Pub/Sub 触发 reload 或周期补偿 reload 执行
- **THEN** 本 change MUST NOT 改变 revision-aware loading、enforcer swap、防倒退、最近错误、readiness/startup 或 watcher 收敛语义
- **AND** 观测失败 MUST NOT 改变 reload 成功或失败的业务结果
