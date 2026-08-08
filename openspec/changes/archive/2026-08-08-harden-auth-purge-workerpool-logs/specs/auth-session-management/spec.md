## ADDED Requirements

### Requirement: 后台会话清理日志不暴露 Redis key material

系统 MUST 在认证会话全量撤销后的后台 Redis purge 任务日志中避免暴露完整 Redis key、Redis key prefix、Redis namespace、用户 UUID hash tag、session ID 或可拼装 refresh session key 的材料。后台任务执行所需的 `purgeKey` 和 `sessionPrefix` MAY 作为闭包内部数据使用，但 MUST NOT 进入 `workerpool.Task.Fields` 或后台任务 error/panic 日志字段。系统 MUST NOT 保留旧的 `purge_key`、`session_prefix` 或等价兼容日志字段。

#### Scenario: purge 任务返回 error 时日志不含 key material
- **WHEN** 退出全部会话或强制改密触发 detached refresh session 后台 purge，且后台 Redis purge 任务返回 error
- **THEN** workerpool 失败日志 MUST 只包含稳定任务名、低敏批量大小、cut time 和可选不可逆 opaque 标识
- **AND** 日志字段名和值 MUST NOT 包含 `purge_key`、`session_prefix`、Redis namespace、`auth:session`、`auth:user:sessions`、`{user_uuid}`、session ID 或可拼装 refresh session key 的材料

#### Scenario: purge 任务 panic 时日志不含 key material
- **WHEN** detached refresh session 后台 purge 任务在 workerpool 执行边界发生 panic
- **THEN** workerpool panic 日志 MUST 保留 panic 和 stacktrace 观测能力
- **AND** 日志字段名和值 MUST NOT 包含 `purge_key`、`session_prefix`、Redis namespace、`auth:session`、`auth:user:sessions`、`{user_uuid}`、session ID 或可拼装 refresh session key 的材料

#### Scenario: purge 执行语义保持不变
- **WHEN** 后台 purge 任务被提交并成功执行
- **THEN** 系统 MUST 继续使用 detached purge key 读取待清理 session 索引，并使用同一用户 session prefix 构造待删除 refresh session key
- **AND** 日志字段收敛 MUST NOT 改变退出全部会话、强制改密、token version 递增、refresh session 撤销或 Redis key 存储格式
