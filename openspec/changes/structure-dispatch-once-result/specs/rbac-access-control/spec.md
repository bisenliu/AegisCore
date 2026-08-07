## MODIFIED Requirements

### Requirement: RBAC policy outbox 可靠投递

系统 MUST 以 PostgreSQL 中已提交的 RBAC policy outbox event 作为跨副本 revision 通知的可靠恢复事实，并由显式 dispatcher 对到期 event 执行 claim、Redis publish、成功 ack 和失败退避。user-service MUST 私有拥有轮询、批量、claim lease 与退避配置，并通过 permission lifecycle 启停同一 dispatcher 实例。dispatcher MUST 提供至少一次投递并在进程崩溃或 Redis 故障后自动恢复；Redis MUST 只作为可重放加速层。dispatcher 单次 batch 执行 MUST 返回结构化结果或结构化错误，使调用方可判别 batch claim、单条 publish/ack/fail、claim lost、backlog/status 刷新和 context 取消等阶段；batch 中任一事件失败时，已成功事件 MAY 已经 publish 并 ack。

#### Scenario: 配置与生命周期

- **WHEN** dispatcher 配置缺失或含非正数 interval、batch size、claim timeout、backoff，或最大退避小于初始退避
- **THEN** user-service MUST 应用完整安全默认值，或在显式非法配置时拒绝启动并报告字段路径
- **WHEN** permission runtime 启动、停止或启动回滚
- **THEN** lifecycle MUST 显式启动或幂等停止 dispatcher，constructor MUST NOT 提前启动 goroutine
- **AND** dispatcher MUST 在 stop context 内等待 in-flight 工作退出，MUST NOT 关闭共享 Ent、PostgreSQL 或 Redis client

#### Scenario: 到期事件被 claim 并成功投递

- **WHEN** pending 或 failed event 的 `next_attempt_at` 已到期且 dispatcher 正在运行
- **THEN** dispatcher MUST 按 revision 升序批量 claim event、发布同一数据库 revision 的 Redis 通知并条件标记 delivered
- **AND** delivered event MUST 记录完成时间、清除 claim 与最近错误，后续扫描 MUST NOT 再将其作为可投递事件返回
- **AND** 单次 batch 结果 MUST 暴露 claim 数、成功投递数、ack 数和成功刷新后的 backlog/status 快照或等价信息

#### Scenario: Redis 故障后自动恢复

- **WHEN** Redis 不可用、version cache 更新失败或 Pub/Sub publish 失败
- **THEN** dispatcher MUST NOT 删除、吞掉或标记该 event 为 delivered
- **AND** 系统 MUST 记录失败 attempt、稳定错误摘要和下一次尝试时间，并按配置退避继续重试
- **AND** 单次 batch 结果 MUST 暴露 publish 失败事件和已安排 retry 的事件计数或等价结构化信息
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

#### Scenario: 单次 batch 部分成功语义

- **WHEN** dispatcher claim 到多个 due event 且其中任一事件 publish、ack 或 failure record 失败
- **THEN** dispatcher MUST 继续处理同 batch 后续未取消的 claim
- **AND** 返回结果 MUST 让调用方观察到已成功 delivered/ack 与 failed/retried 计数或等价结构化信息
- **AND** 返回错误 MUST 可判别失败阶段，MUST NOT 暗示整个 batch 未发生成功投递

#### Scenario: 单次 batch claim 与 status 失败语义

- **WHEN** batch claim 失败
- **THEN** dispatcher MUST 返回 claim 阶段失败且结果 MUST NOT 伪造已 claim 或已投递事件
- **WHEN** backlog/status 刷新失败
- **THEN** dispatcher MUST 返回独立的 backlog/status 刷新失败原因
- **AND** backlog/status 刷新失败 MUST NOT 伪装成事件 publish、ack 或 failure record 全部失败

#### Scenario: 单次 batch context 取消语义

- **WHEN** context 在 claim 后、当前事件未完成 publish 或后续事件处理前取消
- **THEN** dispatcher MUST 返回已累计的结构化结果和可通过 `context.Canceled` 或等价方式判别的错误
- **AND** dispatcher MUST NOT 主动 Ack 或 Fail 当前未完成 claim，后续恢复 MUST 继续依赖 claim lease 过期后的重投递
