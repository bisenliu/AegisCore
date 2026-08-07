## ADDED Requirements

### Requirement: RBAC policy sync 并发与状态测试门禁

系统 MUST 为 RBAC watcher 与 Casbin enforcer 的并发同步、补偿、关闭和 cancellation 语义提供 race/stress 测试门禁。测试 MUST 使用 deterministic fake、可控 channel 或同步 primitive 构造竞态，MUST 能在 `go test -race` 下稳定运行，并且 MUST NOT 依赖真实外部 Redis 或 PostgreSQL 服务。

#### Scenario: watcher 并发通知与周期补偿收敛

- **WHEN** watcher 并发接收多条重复、乱序或较小的 Pub/Sub policy hint，并同时触发周期性 PostgreSQL revision check
- **THEN** 测试 MUST 断言 watcher 只把数据库可见 revision 作为 reload target，并最终通过 revision-aware port 追赶到不低于最高权威 revision 的投影
- **AND** 测试 MUST 断言重复或旧 hint 不会导致 Casbin applied revision 倒退
- **AND** 定向 user-role cache invalidation 的副作用 MUST 按协议执行，但不得独立推进 Casbin applied revision

#### Scenario: watcher 重订阅与状态语义

- **WHEN** 已确认的 Redis Pub/Sub 订阅断连、message channel 关闭或 Receive 返回可恢复错误
- **THEN** 测试 MUST 断言 watcher 根生命周期仍保持 running，subscription state 进入 reconnecting 或等价的重订阅状态
- **AND** 周期性 PostgreSQL revision check MUST 在订阅退避期间继续运行
- **WHEN** 重订阅确认成功
- **THEN** 测试 MUST 断言 subscription state 恢复 connected，并清除当前 subscription 错误

#### Scenario: watcher Stop 竞态与取消语义

- **WHEN** watcher `Stop(ctx)` 与阻塞 revision source、阻塞 reload engine、订阅确认、Receive、退避 timer 或 payload delivery 并发发生
- **THEN** 测试 MUST 断言 Stop 在调用方 context 期限内取消内部 root context 并等待 watcher goroutine 退出，除非测试显式覆盖 Stop 超时语义
- **AND** Stop 完成后 watcher lifecycle MUST 为 stopped，subscription state MUST 为 stopped 或等价关闭状态
- **AND** 正常停止导致的 reconcile cancellation MUST NOT 记录为业务 failure、最近失败时间或当前 reconcile 错误
- **AND** Stop 超时 MUST 返回 context 错误，并保持后续重复 Stop 调用安全

#### Scenario: enforcer 多 waiter 与 reload coalescing

- **WHEN** 多个 goroutine 并发调用 `ReloadToRevision`、`RefreshToRevision` 或等价 revision-aware reload 入口，且 target revision 重复、乱序或递增
- **THEN** 测试 MUST 断言实际 reload 工作被串行化或 coalesce，最终 applied revision 不低于所有未取消等待方请求的最高 target
- **AND** 每个未取消等待方 MUST 只在 engine 实际 applied revision 不低于其 target 后返回成功
- **AND** 单个等待方 context cancellation MUST NOT 取消其他等待方仍需要的共享 reload

#### Scenario: enforcer root cancel、leader cancel 与强制刷新

- **WHEN** engine root context 被取消、reload leader context 被取消或 loader/reload gate 被阻塞
- **THEN** 测试 MUST 断言未完成等待方返回取消错误或对应 reload 错误，engine 不提升 applied revision，不清除最近失败状态，也不使用旧投影放行请求
- **WHEN** force refresh 请求在普通 reload 已经开始读取数据库后加入同一 flight
- **THEN** 测试 MUST 断言 engine 在 force refresh 到达后重新读取一次 PostgreSQL 快照，并且不得把 force 请求到达前构造的候选视为该请求已完成
