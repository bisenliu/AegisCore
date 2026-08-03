## ADDED Requirements

### Requirement: RBAC policy outbox 可靠投递

系统 MUST 以 PostgreSQL 中已提交的 RBAC policy outbox event 作为跨副本 revision 通知的可靠恢复事实，并由显式 dispatcher 对到期 event 执行 claim、Redis publish、成功 ack 和失败退避。dispatcher MUST 提供至少一次投递并在进程崩溃、Redis 暂时不可用或 publish 失败后自动恢复；Redis MUST 只作为数据库 revision 通知的可重放加速层，不得成为 event、revision 或投递状态的权威来源。

#### Scenario: 到期事件被 claim 并成功投递

- **WHEN** pending 或 failed event 的 `next_attempt_at` 已到期且 dispatcher 正在运行
- **THEN** dispatcher MUST 按 revision 升序批量 claim event、发布同一数据库 revision 的 Redis 通知并条件标记 delivered
- **AND** delivered event MUST 记录完成时间、清除 claim 与最近错误，后续扫描 MUST NOT 再将其作为可投递事件返回

#### Scenario: Redis 故障后自动恢复

- **WHEN** Redis 不可用、version cache 更新失败或 Pub/Sub publish 失败
- **THEN** dispatcher MUST NOT 删除、吞掉或标记该 event 为 delivered
- **AND** 系统 MUST 记录失败 attempt、稳定错误摘要和下一次尝试时间，并按配置退避继续重试
- **WHEN** Redis 恢复且 event 再次到期
- **THEN** dispatcher MUST 无需新的 RBAC mutation 或人工复制 event 即可重新发布并最终 ack

#### Scenario: 进程重启与过期 lease 恢复

- **WHEN** dispatcher 在 claim 后、publish 中或 publish 成功但 ack 前停止或崩溃
- **THEN** event MUST 保留 processing 状态和持久 lease，且不得因进程内状态丢失而消失
- **WHEN** claim lease 到期
- **THEN** 任一健康 dispatcher MUST 能重新 claim 并继续处理该 event
- **AND** publish 成功但 ack 前崩溃 MAY 产生重复通知，consumer 副作用 MUST 保持幂等

#### Scenario: 多 dispatcher 并发 claim

- **WHEN** 多个 user-service 副本并发扫描同一批 due event
- **THEN** PostgreSQL claim MUST 通过行级仲裁为每个 event 建立唯一有效 claim token 与 lease
- **AND** 同一 lease 期间最多一个 owner MUST 获得该 event，其他 dispatcher MUST 跳过已 claim 行而非执行非幂等副作用
- **AND** ack 或失败更新 MUST 同时匹配 event、processing 状态和 claim token，过期 owner MUST NOT 覆盖新 owner 的处理结果

#### Scenario: 失败退避与保留

- **WHEN** 第 N 次实际 publish 尝试失败
- **THEN** attempt count MUST 增加一次，下一次尝试 MUST 使用不超过配置最大值的有界指数退避
- **AND** failed event MUST 持续保留且没有因达到固定 attempt 次数而进入不可恢复终态
- **AND** 无效 event 数据 MUST 作为可诊断失败保留并退避，MUST NOT 被静默 ack 或删除

### Requirement: RBAC revision 通知消息与幂等消费

Redis policy refresh 消息 MUST 使用显式版本化 envelope 携带稳定 event identity、数据库 `policy_revision`、change kind、reason 及相关对象 ID。publisher 和 watcher MUST 接受消息的重复与乱序，Redis revision cache 与本地 revision tracker MUST 只按 max 推进；系统 MUST NOT 保留旧 `INCR` counter 或旧消息 payload fallback。

#### Scenario: 发布完整 revision 通知

- **WHEN** dispatcher 发布 `policy_changed` 或 `user_role_changed` event
- **THEN** payload MUST 包含 `schema_version`、`event_id`、`idempotency_key`、`policy_revision`、`kind`、`reason` 和 publisher instance ID
- **AND** payload MUST 携带 event 中存在的 `user_id`、`role_id`、`permission_id`，缺失的可选 ID MUST 保持为空
- **AND** publisher MUST 以原子 max 语义缓存数据库 revision，较小或重复 revision MUST NOT 使 Redis 值倒退

#### Scenario: 重复与乱序通知保持幂等

- **WHEN** watcher 重复收到同一 event，或先收到较大 revision 后收到较小 revision
- **THEN** `policy_changed` MUST 安全地从当前 PostgreSQL 权威投影执行全量 reload，`user_role_changed` MUST 安全地失效消息指定用户的角色缓存
- **AND** watcher MUST NOT 仅因消息 revision 不大于本地已知最大值而跳过该消息要求的缓存失效或 reload 副作用
- **AND** 完成副作用后本地 tracker MUST 只按 max 推进，MUST NOT 回退已知 revision

#### Scenario: 非法或旧协议消息被拒绝

- **WHEN** payload 缺少必需字段、包含未知 schema version/kind 或非法 UUID
- **THEN** watcher MUST 拒绝执行该消息并记录不含完整 payload 或敏感数据的诊断错误
- **AND** watcher MUST NOT 尝试按旧消息形状解析，也 MUST NOT 回退到 Redis counter 语义

#### Scenario: Redis 不是可靠或权威存储

- **WHEN** Redis revision cache 更新成功但 Pub/Sub publish 失败，或 Pub/Sub 消息丢失
- **THEN** outbox event MUST 保持未完成并可重试，watcher 的周期补偿 MAY 使用 Redis 已知最大 revision 加速发现变化
- **AND** PostgreSQL revision、outbox event 与 RBAC 关系投影 MUST 继续是恢复和授权数据的权威来源
- **AND** 系统 MUST NOT 要求 Redis publish 与 PostgreSQL mutation transaction 原子化
