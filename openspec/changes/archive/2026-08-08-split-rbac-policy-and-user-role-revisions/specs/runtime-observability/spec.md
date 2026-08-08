## MODIFIED Requirements

### Requirement: RBAC 同步可观测性、健康与投影 lag

系统 MUST 为 outbox dispatcher 与 policy watcher 暴露低基数 metrics、结构化日志和只读 status，并接入 health/readiness。Policy reload lag MUST 只表示 PostgreSQL latest policy revision 与 Casbin engine actual applied revision 的非负差值；user-role revision、Redis counter、Pub/Sub payload 或 reload attempt MUST NOT 充当 policy lag 权威值。健康探测 MUST 只读，MUST NOT 修改 outbox event。

#### Scenario: backlog、lag 与投递指标

- **WHEN** dispatcher claim、publish、ack、失败、重试或采集 outbox 状态
- **THEN** feature metrics MUST 记录固定 result/reason/kind 枚举下的处理计数、due backlog、最老未完成 event age 和 loop 运行状态
- **AND** metrics label MUST NOT 包含 event/revision/user/role/permission ID、idempotency key、原始错误、SQL、Redis key、payload 或其他高基数字段
- **AND** `kind` MUST 使用固定枚举区分 policy 事件与 user-role 事件，MUST NOT 通过 label 暴露具体 revision 或用户标识

#### Scenario: dispatcher 结构化日志

- **WHEN** event 被 claim、成功投递、失败退避、lease 冲突或循环状态变化
- **THEN** 日志 MUST 使用英文 message 和稳定 `snake_case` 字段，并 MAY 记录 policy revision、user-role revision、attempt、kind、reason 和稳定错误类别
- **AND** 日志 MUST NOT 记录完整 event payload、SQL、Redis key、连接 secret 或将原始底层错误暴露到公共健康响应
- **AND** policy 事件字段 MUST 使用 `policy_revision`，user-role 事件字段 MUST 使用 `user_role_revision`，MUST NOT 用 `policy_revision` 表示用户角色绑定提交水位

#### Scenario: 只读 health 与 readiness

- **WHEN** dispatcher 正在运行且可读取 outbox 状态
- **THEN** status MUST 报告最近成功时间、最近错误类别、due count 和最老未完成 event age，探测 MUST NOT 改变任何 event
- **WHEN** dispatcher 未启动、循环意外退出或 outbox 状态查询失败
- **THEN** readiness MUST 失败并返回稳定且不含敏感信息的定位结果
- **AND** 单次 publish 失败或处于退避中的 backlog MUST 保持可见且不得终止 dispatcher 循环

#### Scenario: metrics 禁用时保持行为

- **WHEN** 全局 metrics provider 禁用
- **THEN** dispatcher MUST 继续 claim、发布、重试和更新只读 status，并通过非 nil no-op feature metrics 满足正式依赖图
- **AND** 系统 MUST NOT 因 collector 未注册而改变 event 投递、health 或 readiness 状态机

#### Scenario: 数据库 latest policy 超前时暴露非零 lag

- **WHEN** watcher 成功读取的 database latest policy revision 大于 local applied projection revision
- **THEN** `aegiscore_user_service_rbac_policy_reload_lag` MUST 记录两者的正差值
- **AND** watcher MUST 记录 database revision mismatch 事件，metrics label MUST 只使用固定低基数 source、result 和 reason allowlist
- **AND** dashboard、alert 和 runbook MUST 将该值解释为数据库 policy 授权事实与本地实际 Casbin 投影之间的差值

#### Scenario: user-role revision 禁止影响 policy lag

- **WHEN** 只有用户角色绑定变化且 latest user-role revision 高于 local applied policy revision
- **THEN** `aegiscore_user_service_rbac_policy_reload_lag` MUST NOT 因 user-role revision 变为非零
- **AND** watcher MUST NOT 将 user-role revision mismatch 记录为 policy reload lag，MUST NOT 触发 policy reload failure 或 policy readiness 失败
- **AND** user-role 通知 MAY 通过固定 kind/reason 的 dispatcher 或 watcher 计数、缓存失效计数和结构化日志体现

#### Scenario: lag 为零禁止假收敛

- **WHEN** watcher 基于一次成功数据库 policy revision 读取记录 lag 为 `0`
- **THEN** local applied projection revision MUST 大于或等于该次读取的 database latest policy revision
- **AND** Redis counter 缺失、落后、重建、等于 local 值、user-role revision 推进或 Pub/Sub 消息处理成功 MUST NOT 单独使 policy lag 变为 `0`
- **WHEN** local applied revision 高于本次读取的 database latest policy revision
- **THEN** lag MUST 按非负规则记录为 `0` 且 MUST NOT 降低 local applied revision

#### Scenario: 查询或 reload 失败不清零 lag

- **WHEN** database latest policy revision 读取失败
- **THEN** 系统 MUST 记录固定 `revision_store_unavailable` 或等价 reason，并保留上一 lag 观测值，MUST NOT 用 Redis、hint revision 或 user-role revision 更新 lag
- **WHEN** database latest policy revision 读取成功但 reload 失败或实际 applied revision 仍低于目标
- **THEN** 系统 MUST 记录固定 `reload_failed` reason 并保留基于 database latest policy revision 与 actual applied 计算的非零 lag
- **AND** 只有后续成功数据库 policy 校准证明 actual applied revision 不低于 database latest policy revision 时，系统才 MUST 把 lag 记录为 `0`

#### Scenario: watcher 指标 reason 与日志字段

- **WHEN** watcher 记录周期检查、Pub/Sub 唤醒、revision mismatch、reload success 或 reload failure
- **THEN** metrics MUST 使用稳定低基数 source/result/reason 区分 `revision_store_unavailable`、`revision_mismatch`、`reload_failed` 与成功
- **AND** metrics reason MUST NOT 继续以 Redis version store 不可用表达数据库 revision 查询失败，也 MUST NOT 包含 revision 数值、用户、角色、权限、Redis key 或原始错误文本
- **AND** 结构化日志 MUST 使用 `database_latest_policy_revision`、`local_applied_policy_revision`、`target_policy_revision`、`hint_policy_revision`、`user_role_revision`、`source` 和稳定 reason 中的适用字段
- **AND** 日志 MUST NOT 使用含混的 `remote_policy_revision` 或 `remote_version` 字段把 Redis 消息、user-role revision 或 counter 描述为数据库 policy 权威事实，也 MUST NOT 记录 policy 内容、SQL、Redis key 或 secret

#### Scenario: dashboard、alert 与 fixture 同步

- **WHEN** Grafana dashboard 展示 RBAC policy reload lag 或 Prometheus alert 评估持续未收敛
- **THEN** 查询、panel 说明、alert annotation 和 runbook MUST 使用 database latest policy revision 与 local applied projection revision 语义
- **AND** alert MUST 继续覆盖超过既定最终收敛 SLO 的非零 policy lag，并将 policy revision store unavailable 与 policy reload failure 作为可定位关联信号
- **AND** dashboard 源、Compose provisioning 副本、Prometheus rules、metrics load 测试和相关 fixture MUST 在同一 change 中更新
- **AND** 生成或检查命令 MUST 在旧 Redis/local version 文案、混合 revision 文案、PromQL 或 dashboard drift 存在时失败
