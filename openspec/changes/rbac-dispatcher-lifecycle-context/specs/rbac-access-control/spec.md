## MODIFIED Requirements

### Requirement: RBAC policy outbox 可靠投递

系统 MUST 以 PostgreSQL 中已提交的 RBAC policy outbox event 作为跨副本 revision 通知的可靠恢复事实，并由显式 dispatcher 对到期 event 执行 claim、Redis publish、成功 ack 和失败退避。user-service MUST 私有拥有轮询、批量、claim lease 与退避配置，并通过 permission lifecycle 启停同一 dispatcher 实例。dispatcher MUST 提供至少一次投递并在进程崩溃或 Redis 故障后自动恢复；Redis MUST 只作为可重放加速层。dispatcher 启动 MUST 接收显式 lifecycle context，并以该 context 派生后台运行 context；Start 路径 MUST NOT 使用 `context.Background()` 作为运行根上下文，也 MUST NOT 保留无参 `Start()` 兼容接口或 adapter。

#### Scenario: 配置与生命周期

- **WHEN** dispatcher 配置缺失或含非正数 interval、batch size、claim timeout、backoff，或最大退避小于初始退避
- **THEN** user-service MUST 应用完整安全默认值，或在显式非法配置时拒绝启动并报告字段路径
- **WHEN** permission runtime 启动、停止或启动回滚
- **THEN** lifecycle MUST 在 `OnStart(ctx)` 中显式调用 `Dispatcher.Start(ctx)`，或幂等调用 `Stop(ctx)` 停止 dispatcher，constructor MUST NOT 提前启动 goroutine
- **AND** dispatcher `Start(ctx)` MUST 使用传入 ctx 派生后台运行 context，并保存 cancel 供 `Stop(ctx)` 触发退出
- **AND** dispatcher MUST 在 stop context 内等待 in-flight 工作退出，`Stop(ctx)` 的 ctx MUST 只控制等待退出的期限，MUST NOT 替代 Start 建立的运行根 context
- **AND** dispatcher MUST NOT 关闭共享 Ent、PostgreSQL 或 Redis client

#### Scenario: dispatcher start stop 幂等性

- **WHEN** 同一 dispatcher 已经运行且调用方再次调用 `Start(ctx)`
- **THEN** dispatcher MUST 保持单一后台 loop 或 ticker，并 MUST NOT 覆盖正在运行实例的 cancel 或启动第二个 worker
- **WHEN** dispatcher 未运行、正在停止或已经停止后调用 `Stop(ctx)`
- **THEN** dispatcher MUST 稳定返回，并在调用方期限内等待已存在的后台 loop 退出或确认无需等待

#### Scenario: dispatcher 运行上下文观测

- **WHEN** dispatcher 启动、进入后台轮询、记录 running 状态或报告 loop 停止
- **THEN** 后台轮询、结构化日志和 `DispatcherRunningObserved(true/false)` MUST 使用由 `Start(ctx)` 传入 lifecycle context 派生的运行 context 或其 logger-aware 派生 context
- **AND** dispatcher Start 路径 MUST NOT 调用 `context.WithCancel(context.Background())` 建立运行根 context

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
