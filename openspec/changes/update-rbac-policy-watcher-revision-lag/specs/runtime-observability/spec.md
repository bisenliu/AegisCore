## ADDED Requirements

### Requirement: RBAC policy reload lag 反映数据库投影差值

系统 MUST以`max(database_latest_policy_revision - local_applied_policy_revision, 0)`作为RBAC policy reload lag的唯一稳定语义。database latest MUST来自watcher最近一次成功读取的可靠数据库revision source，local applied MUST来自Casbin engine当前实际成功应用的projection revision；Redis counter、Pub/Sub payload、消息接收进度或reload attempt MUST NOT作为lag任一侧的权威值。旧Redis/local version差值 MUST NOT作为稳定指标、dashboard、alert、runbook或兼容PromQL保留。

#### Scenario: 数据库latest超前时暴露非零lag

- **WHEN** watcher成功读取的database latest policy revision大于local applied projection revision
- **THEN** `aegiscore_user_service_rbac_policy_reload_lag` MUST记录两者的正差值
- **AND** watcher MUST记录database revision mismatch事件，metrics label MUST只使用固定低基数source、result和reason allowlist
- **AND** dashboard、alert和runbook MUST将该值解释为数据库授权事实与本地实际投影之间的差值

#### Scenario: lag为零禁止假收敛

- **WHEN** watcher基于一次成功数据库revision读取记录lag为`0`
- **THEN** local applied projection revision MUST不小于该次读取的database latest policy revision
- **AND** Redis counter缺失、落后、重建、等于local值或Pub/Sub消息处理成功 MUST NOT单独使lag变为`0`
- **WHEN** local applied revision高于本次读取的database latest revision
- **THEN** lag MUST按非负规则记录为`0`且 MUST NOT降低local applied revision

#### Scenario: 查询或reload失败不清零lag

- **WHEN** database latest revision读取失败
- **THEN** 系统 MUST记录固定`revision_store_unavailable`或等价reason，并保留上一lag观测值，MUST NOT用Redis或hint revision更新lag
- **WHEN** database latest读取成功但reload失败或实际applied revision仍低于目标
- **THEN** 系统 MUST记录固定`reload_failed`reason并保留基于database latest与actual applied计算的非零lag
- **AND** 只有后续成功数据库校准证明actual applied revision不低于database latest时，系统才 MUST把lag记录为`0`

#### Scenario: watcher指标reason与日志字段

- **WHEN** watcher记录周期检查、Pub/Sub唤醒、revision mismatch、reload success或reload failure
- **THEN** metrics MUST使用稳定低基数source/result/reason区分`revision_store_unavailable`、`revision_mismatch`、`reload_failed`与成功
- **AND** metrics reason MUST NOT继续以Redis version store不可用表达数据库revision查询失败，也 MUST NOT包含revision数值、用户、角色、权限、Redis key或原始错误文本
- **AND** 结构化日志 MUST使用`database_latest_policy_revision`、`local_applied_policy_revision`、`target_revision`、`hint_revision`、`source`和稳定reason中的适用字段
- **AND** 日志 MUST NOT使用含混的`remote_policy_revision`或`remote_version`字段把Redis消息或counter描述为数据库权威事实，也 MUST NOT记录policy内容、SQL、Redis key或secret

#### Scenario: dashboard、alert与fixture同步

- **WHEN** Grafana dashboard展示RBAC policy reload lag或Prometheus alert评估持续未收敛
- **THEN** 查询、panel说明、alert annotation和runbook MUST使用database latest与local applied projection revision语义
- **AND** alert MUST继续覆盖超过既定最终收敛SLO的非零lag，并将revision store unavailable与reload failure作为可定位关联信号
- **AND** dashboard源、Compose provisioning副本、Prometheus rules、metrics load测试和相关fixture MUST在同一change中更新
- **AND** 生成或检查命令 MUST在旧Redis/local version文案、PromQL或dashboard drift存在时失败
