## ADDED Requirements

### Requirement: workerpool 消费端日志字段安全

系统 MUST 要求 `workerpool.Task.Fields` 的生产者只传入低敏定位字段。任何消费端提交的后台任务字段 MUST NOT 包含密码、token、完整 Redis key、Redis key prefix、可拼装 Redis key 的材料、SQL、原始凭据或其他敏感值。workerpool MAY 在 error 和 panic 路径原样记录 `Task.Fields`，字段安全责任 MUST 由消费端在提交任务前满足。

#### Scenario: 任务失败日志只记录低敏字段
- **WHEN** 消费端提交 workerpool 任务且任务返回 error
- **THEN** workerpool MAY 记录任务池名称、任务名称、调用方提供的低敏 `Task.Fields` 和 error
- **AND** 消费端提供的 `Task.Fields` MUST NOT 包含完整 Redis key、Redis key prefix、可拼装 Redis key 的材料、token、密码、SQL 或原始凭据

#### Scenario: 任务 panic 日志只记录低敏字段
- **WHEN** 消费端提交 workerpool 任务且任务发生 panic
- **THEN** workerpool MAY 记录任务池名称、任务名称、调用方提供的低敏 `Task.Fields`、panic 和 stacktrace
- **AND** 消费端提供的 `Task.Fields` MUST NOT 包含完整 Redis key、Redis key prefix、可拼装 Redis key 的材料、token、密码、SQL 或原始凭据

#### Scenario: common 不承担业务字段脱敏
- **WHEN** feature 需要在后台任务失败或 panic 日志中关联单次业务操作
- **THEN** feature MUST 在自身边界生成低敏字段或不可逆 opaque 标识后再写入 `Task.Fields`
- **AND** `common/runtime/workerpool` MUST NOT 为 user-service auth、RBAC、refresh session、Redis key schema 或其他服务私有字段提供业务专用过滤、兼容字段或脱敏分支
