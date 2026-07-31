## ADDED Requirements

### Requirement: RBAC outbox dispatcher 运行时配置与生命周期

user-service MUST 私有拥有 RBAC outbox dispatcher 的轮询、批量、claim lease 和重试退避配置，并通过 permission runtime 与 Fx lifecycle 显式启动和停止同一 dispatcher 实例。constructor MUST 只构造对象，application/domain MUST 不依赖 Fx、Ent 或 Redis concrete type，dispatcher MUST 不关闭共享 PostgreSQL 或 Redis client。

#### Scenario: 配置默认值与校验

- **WHEN** dispatcher 配置缺失
- **THEN** user-service MUST 提供完整且安全的 `poll_interval`、`batch_size`、`claim_timeout`、`retry_backoff.initial` 和 `retry_backoff.max` 默认值
- **WHEN** interval、batch size、claim timeout 或 backoff 不是正值，或 max backoff 小于 initial backoff
- **THEN** composition MUST 返回明确配置错误并拒绝启动，MUST NOT 静默修正为零值或禁用可靠投递

#### Scenario: dispatcher 显式启停

- **WHEN** permission/RBAC lifecycle 启动
- **THEN** hook MUST 在所需共享资源可用后显式启动 dispatcher，constructor MUST NOT 提前启动 goroutine、扫描数据库或发布 Redis 消息
- **WHEN** lifecycle 停止、启动回滚或 stop context 到期
- **THEN** dispatcher MUST 取消轮询并在调用方期限内等待 in-flight 工作退出，重复 Start/Stop MUST 安全且幂等
- **AND** dispatcher MUST NOT 关闭共享 Ent、PostgreSQL 或 Redis resource

#### Scenario: 后台错误不中断可恢复循环

- **WHEN** 单轮扫描、claim、publish、ack 或失败记录发生可重试错误
- **THEN** dispatcher MUST 更新只读状态并继续后续轮询，MUST NOT panic、静默退出或使 event 从 PostgreSQL 消失
- **WHEN** 循环发生不可恢复的意外退出
- **THEN** status MUST 将 running 置为 false 并保留稳定错误类别供 readiness 读取

### Requirement: RBAC outbox dispatcher 可观测性与健康状态

系统 MUST 为 dispatcher 暴露低基数 metrics、结构化日志和 permission feature 只读 status，并将其接入 user-service health/readiness。健康探测 MUST 只读取状态或执行只读 outbox 查询，不得 claim、publish、ack、修改 retry 时间或依赖 infrastructure concrete implementation。

#### Scenario: backlog、lag 与投递指标

- **WHEN** dispatcher claim、publish、ack、失败、重试或采集 outbox 状态
- **THEN** feature metrics MUST 记录固定 result/reason/kind 枚举下的处理计数、due backlog、最老未完成 event age 和 loop 运行状态
- **AND** metrics label MUST NOT 包含 event/revision/user/role/permission ID、idempotency key、原始错误、SQL、Redis key、payload 或其他高基数字段

#### Scenario: dispatcher 结构化日志

- **WHEN** event 被 claim、成功投递、失败退避、lease 冲突或循环状态变化
- **THEN** 日志 MUST 使用英文 message 和稳定 `snake_case` 字段，并 MAY 记录 policy revision、attempt、kind、reason 和稳定错误类别
- **AND** 日志 MUST NOT 记录完整 event payload、SQL、Redis key、连接 secret 或将原始底层错误暴露到公共健康响应

#### Scenario: 只读 health 与 readiness

- **WHEN** dispatcher 正在运行且可读取 outbox 状态
- **THEN** status MUST 可报告最近成功时间、最近错误类别、due count 和最老未完成 event age，探测 MUST 不改变任何 event
- **WHEN** dispatcher 未启动、循环意外退出或 outbox 状态查询失败
- **THEN** readiness MUST 失败并返回稳定且不含敏感信息的定位结果
- **AND** 单次 publish 失败或处于退避中的 backlog MUST 保持可见且不得终止 dispatcher 循环

#### Scenario: metrics 禁用时保持行为

- **WHEN** 全局 metrics provider 禁用
- **THEN** dispatcher MUST 继续 claim、发布、重试和更新只读 status，并通过非 nil no-op feature metrics 满足正式依赖图
- **AND** 系统 MUST NOT 因 collector 未注册而改变 event 投递、health 或 readiness 状态机
