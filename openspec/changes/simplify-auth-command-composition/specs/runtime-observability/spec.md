## ADDED Requirements

### Requirement: Auth Metrics 正式依赖图必须完整

user-service 的正式 auth 模块 MUST 向五个 command use case 和 Redis SessionStore 提供唯一且明确的单值 `authapplication.Metrics` 依赖。该依赖 MUST 在 Fx/Dig graph 中作为必选输入边存在，MUST NOT 使用 `optional`、variadic、slice/group 或 nil 表达正式降级。metrics enabled 时 MUST 注入 Prometheus recorder；metrics disabled 时 MUST 由 auth metrics provider 注入 `authapplication.NopMetrics()`。缺失 `*commonmetrics.Provider` 或缺失 `authapplication.Metrics` 的正式 graph MUST 构图失败，MUST NOT 被解释为自动 no-op。

#### Scenario: metrics 启用时注入 Prometheus recorder

- **WHEN** user-service 以 metrics 启用配置构造正式 auth module
- **THEN** auth module MUST 向登录、刷新、改密、退出当前会话、退出全部会话 use case 和 Redis SessionStore 注入当前 Prometheus Metrics 实现
- **AND** auth 指标记录 MUST 使用既有 metric family、label key、label value 和低基数约束

#### Scenario: metrics 禁用时注入 NopMetrics

- **WHEN** user-service 以 metrics 禁用配置构造正式 auth module
- **THEN** auth module MUST 通过 `newAuthMetrics` 或等价 auth metrics provider 注入 `authapplication.NopMetrics()`
- **AND** 五个 command use case 和 Redis SessionStore MUST 在不接收 nil Metrics 的情况下完成构图
- **AND** 系统 MUST NOT 注册或更新 auth Prometheus 指标

#### Scenario: command use case Metrics edge 为必选单值

- **WHEN** 正式 auth module 构造登录、刷新、改密、退出当前会话或退出全部会话 use case
- **THEN** 每个 constructor MUST 声明一个明确的 `authapplication.Metrics` 输入边
- **AND** 该输入边 MUST NOT 使用 `optional`、variadic、slice/group annotation 或 nil 表达可缺失依赖
- **AND** 缺失该输入时 Fx graph MUST 构造失败

#### Scenario: Redis SessionStore Metrics edge 为必选单值

- **WHEN** 正式 auth module 构造 Redis refresh session store
- **THEN** `authredis.SessionStoreParams.Metrics` MUST 是必选输入
- **AND** 该输入 MUST NOT 使用 `optional` tag 或 nil 表达正式降级
- **AND** 不观察指标的直接 SessionStore 测试 MUST 显式传入 `authapplication.NopMetrics()`

#### Scenario: no-op 与 graph 缺边语义区分

- **WHEN** metrics 配置显式禁用
- **THEN** auth metrics provider MUST 返回 no-op 实现并允许正式 graph 成功构造
- **AND** 当 `*commonmetrics.Provider` 或 auth metrics provider 未注册时，正式 graph MUST fail-fast
- **AND** use case 或 SessionStore 内部的 nil 防御 MUST NOT 被用作正式 graph 的降级契约

#### Scenario: 指标契约保持不变

- **WHEN** auth Metrics 的正式依赖接线被收紧
- **THEN** 既有 metric family、指标名称、label key、label value 和低基数约束 MUST 保持不变
- **AND** 系统 MUST NOT 新增 metrics backend、dashboard、alert、配置字段或部署资产
