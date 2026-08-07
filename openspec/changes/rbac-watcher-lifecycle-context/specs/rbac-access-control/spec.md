## MODIFIED Requirements

### Requirement: RBAC watcher 自恢复生命周期与权威校准状态

RBAC watcher MUST 在单一显式生命周期内持续监督 Redis policy refresh 订阅与 PostgreSQL policy revision 权威校准。订阅故障 MUST NOT 终止数据库补偿；瞬时错误恢复后 MUST 更新当前状态且不得因历史错误保持永久失败。watcher MUST 只通过 permission application 拥有的结构化只读 status port 暴露状态，MUST NOT 保留 `Running()`/`LastError()` 旧接口、旧状态 adapter 或兼容分支。watcher 启动 MUST 接收显式 lifecycle context，并以该 context 派生后台运行 context；Start 路径 MUST NOT 使用 `context.Background()` 作为运行根上下文，也 MUST NOT 保留无参 `Start()` 兼容接口或 adapter。

#### Scenario: Watcher 生命周期与状态恢复

- **WHEN** permission runtime 启动、停止或启动回滚
- **THEN** lifecycle MUST 在 `OnStart(ctx)` 中显式调用 `Watcher.Start(ctx)`，或幂等调用 `Stop(ctx)` 停止 watcher，constructor MUST NOT 提前启动 goroutine
- **AND** watcher `Start(ctx)` MUST 使用传入 ctx 派生后台运行 context，并保存 cancel 供 `Stop(ctx)` 触发退出
- **AND** watcher MUST 在 stop context 内等待 in-flight 消息处理或 revision check 退出，`Stop(ctx)` 的 ctx MUST 只控制等待退出的期限，MUST NOT 替代 Start 建立的运行根 context
- **AND** watcher MUST NOT 关闭共享 Redis client
- **WHEN** 同一 watcher 已经运行且调用方再次调用 `Start(ctx)`
- **THEN** watcher MUST 保持单一后台 loop 或 ticker，并 MUST NOT 覆盖正在运行实例的 cancel 或启动第二个 worker
- **WHEN** Redis subscription 断开、重连或关闭消息 channel
- **THEN** watcher 根生命周期 MUST 保持 running，MUST NOT 要求人工操作、进程重启或新的 RBAC mutation 才能恢复

#### Scenario: Watcher 运行上下文观测

- **WHEN** watcher 启动、处理 Pub/Sub payload、执行周期 revision check、查询 latest revision、执行 projection reload 或记录同步结果
- **THEN** 后台消息处理、周期校准、结构化日志和 watcher metrics MUST 使用由 `Start(ctx)` 传入 lifecycle context 派生的运行 context 或其 logger-aware 派生 context
- **AND** watcher Start 路径 MUST NOT 调用 `context.WithCancel(context.Background())` 建立运行根 context

### Requirement: RBAC 架构装配与资源生命周期

role 和 permission feature MUST 保持 domain、application、transport 和 infrastructure 分层。permission application MUST 只保留权限查询、授权、policy loading/sync 和 seed/角色绑定所需最小端口，不得保留公开权限 command 或 route diff 生产装配。domain/application MUST 框架无关并拥有消费侧最小 port；Fx、Gin、Ent、Redis、SQL、HTTP response 和 named resource metadata MUST 留在对应边界。RBAC watcher 和 policy 投影主动资源 MUST 显式启动、停止和回滚；无后台执行的 user-role localcache MUST NOT 拥有启停或关闭生命周期。permission composition MUST 以单一 runtime 聚合对象表达稳定组件集合。

#### Scenario: RBAC 主动资源生命周期

- **WHEN** user-service 启停 permission/RBAC runtime
- **THEN** permission lifecycle MUST 先初始化 fail-closed policy，再通过显式 lifecycle context 启动 watcher，随后启动 dispatcher；停止时 MUST 先停 dispatcher，再停 watcher
- **AND** watcher 与 dispatcher 的 stop context MUST 只控制本次等待期限，不得替代各自 Start 建立的运行根 context
- **AND** watcher 或 dispatcher 启动失败 MUST 触发已启动主动资源的幂等回滚停止
