## ADDED Requirements

### Requirement: RBAC policy sync 统一生命周期上下文

RBAC policy sync 的 dispatcher、watcher、subscriber 和 enforcer reload engine MUST 由 permission runtime 接收的显式服务 lifecycle root context 统一约束。后台运行 context MUST 从该 root context 派生；启动路径 MUST NOT 使用 `context.Background()` 建立独立 root，MUST NOT 保留无参 `Start()` 或等价兼容 adapter。`Stop(ctx)` 的 ctx MUST 只限制等待退出的期限，单个 reload waiter 的 ctx MUST 只取消该 waiter，MUST NOT 替代或取消仍被其他参与者需要的共享运行 root 或 reload flight。

#### Scenario: permission lifecycle 启动与停止后台同步链路

- **WHEN** permission runtime 启动、停止或执行启动失败回滚
- **THEN** lifecycle MUST 使用同一服务 lifecycle root context 显式启动 watcher、subscriber 和 dispatcher，并使 enforcer reload engine 受该 root cancellation 约束
- **AND** constructor MUST NOT 提前启动 goroutine，启动失败 MUST 幂等停止已启动资源
- **AND** 停止顺序 MUST 阻止新的 dispatch 或 reconcile 工作进入，再取消运行 root 并等待 in-flight 工作退出
- **AND** 任一组件 MUST NOT 关闭共享 PostgreSQL、Ent 或 Redis client

#### Scenario: lifecycle root cancellation 终止共享 reload flight

- **WHEN** 服务 lifecycle root context 被取消且 enforcer 仍有进行中的 reload flight 或等待方
- **THEN** reload engine MUST 取消未完成工作并使等待方返回 cancellation 或对应 reload error
- **AND** engine MUST NOT 提升 applied revision、清除最近失败状态或把未完成候选投影发布为成功结果
- **WHEN** 仅一个 reload waiter 的 context 被取消且其他 waiter 仍需要同一 flight
- **THEN** engine MUST 只结束该 waiter 的等待，共享 flight MUST 继续服务其他未取消 waiter

### Requirement: RBAC dispatcher batch partial success 与异常终态

RBAC policy outbox dispatcher 单次 batch MUST 返回结构化结果或等价结构化状态，使调用方能够区分 claim、publish、ack、failure record、claim lost、backlog/status refresh 和 context cancellation。dispatcher MUST 保留 partial success：单条事件失败 MUST NOT 阻断同 batch 后续未取消事件，已成功 publish 或 ack 的结果 MUST NOT 因最终返回 error 而被抹除。旧 error-only `DispatchOnce` 语义 MUST NOT 作为兼容行为保留。后台 loop panic MUST fail-closed 进入 `unexpected_exit`，并留下完整恢复证据。

#### Scenario: dispatcher batch 部分成功

- **WHEN** dispatcher claim 多个 due event，且其中部分事件发生 publish、ack、failure record 或 claim lost 错误
- **THEN** dispatcher MUST 继续处理同 batch 后续未取消 claim
- **AND** 结构化结果 MUST 暴露 claimed、delivered、acknowledged、retried、failed 和 status refresh 成功与否或等价信息
- **AND** 返回错误 MUST 可判别每个失败阶段，MUST NOT 暗示整个 batch 未发生成功投递
- **AND** 已成功 ack 的事件 MUST 保持 delivered，失败或失去 claim 的事件 MUST 按既有 lease recovery 与退避语义恢复

#### Scenario: claim、status refresh 与 cancellation 相互独立

- **WHEN** batch claim 失败
- **THEN** 结果 MUST NOT 伪造已 claim、已投递或已 ack 事件，并 MUST 标识 claim 阶段错误
- **WHEN** backlog/status refresh 失败但 batch 内已有事件成功投递
- **THEN** 结果 MUST 保留已成功事件计数，并独立标识 status refresh 失败
- **WHEN** context 在某个 claim 完成前被取消
- **THEN** dispatcher MUST 停止开始新的工作，且 MUST NOT 主动 Ack 或 Fail 当前未完成 claim，后续恢复 MUST 继续依赖 claim lease

#### Scenario: dispatcher 后台 panic recovery 可观测性

- **WHEN** dispatcher 后台 run loop 发生 panic
- **THEN** recovery 日志 MUST 记录 `error_category=unexpected_exit`
- **AND** recovery 日志 MUST 记录来自 `recover()` 的 recovered value 和 stack trace
- **AND** dispatcher MUST 将最近错误分类更新为 `unexpected_exit`，停止 ticker，设置 running=false，上报对应运行指标并关闭当前 done signal
- **AND** dispatcher MUST NOT 自动重启后台 loop，后续 `Stop(ctx)` MUST 幂等稳定返回

### Requirement: RBAC watcher 断连恢复与 final state

RBAC watcher MUST 分别维护 lifecycle、subscription 与 reconcile 状态，并以 PostgreSQL policy revision 作为最终权威事实。Redis subscription 断连、message channel 关闭或可恢复 Receive error MUST 触发重订阅而不是终止 watcher root lifecycle；订阅退避期间周期 reconcile MUST 继续运行。正常停止、Stop 等待超时、reconcile cancellation 与异常退出 MUST 形成可判别 final state，历史瞬时错误恢复后 MUST NOT 使 watcher 永久保持失败。

#### Scenario: Redis 断连与重订阅

- **WHEN** 已确认的 Redis subscription 断连、message channel 关闭或 Receive 返回可恢复错误
- **THEN** watcher lifecycle MUST 保持 running，subscription MUST 进入 reconnecting 或等价状态
- **AND** PostgreSQL revision reconcile MUST 在重订阅退避期间继续运行，Redis hint MUST NOT 取代数据库权威 revision
- **WHEN** subscriber 完成新的订阅确认
- **THEN** subscription MUST 恢复 connected，并清除当前 subscription error 与对应失败时间

#### Scenario: watcher 正常停止与 reconcile cancellation

- **WHEN** watcher root context 因正常 lifecycle shutdown 被取消，且 revision query、reload、订阅确认、Receive、退避 timer 或 payload handling 正在执行
- **THEN** watcher MUST 停止接收新工作并等待 in-flight 工作退出
- **AND** 由该正常停止直接导致的 reconcile cancellation MUST NOT 记录为业务 failure、最近失败时间或当前 reconcile error
- **AND** 后台 loop 真正退出后 lifecycle 与 subscription MUST 进入 stopped 或等价关闭终态

#### Scenario: watcher Stop 超时

- **WHEN** `Stop(ctx)` 的等待期限先于 watcher 后台 loop 退出到期
- **THEN** Stop MUST 返回调用方 context error，并保持内部 root cancellation 已发出
- **AND** watcher MUST NOT 在后台 loop 实际退出前伪造 stopped final state
- **AND** 后续重复 Stop MUST 保持安全，并可在后台退出后观察到 stopped final state

#### Scenario: subscriber 与 watcher 责任边界

- **WHEN** Redis 订阅需要建立、确认、取消、断连检测或重新建立
- **THEN** `common/runtime/redispubsub` subscriber MAY 提供无业务语义 lifecycle primitive
- **AND** policy revision envelope、数据库权威校准、reconcile 状态、user-role cache invalidation 和 watcher final state MUST 由 permission feature 拥有

### Requirement: RBAC policy sync race 与 stress 验证门禁

系统 MUST 为 dispatcher、watcher、subscriber 和 enforcer reload engine 的并发、关闭、异常与 cancellation 语义提供 race/stress 验证。测试 MUST 使用 deterministic fake、可控 channel、barrier 或等价同步 primitive，MUST NOT 依赖真实 Redis 或 PostgreSQL，并 MUST 断言规格状态而非偶然 goroutine 调度顺序。

#### Scenario: dispatcher、watcher 与 subscriber 并发验证

- **WHEN** 测试并发触发 dispatcher partial success、panic finalization、watcher 断连重订阅、reconcile、Stop 和 subscriber cancellation
- **THEN** 测试 MUST 在 `go test -race` 下稳定通过且不得报告 data race、goroutine leak 或重复关闭 panic
- **AND** 测试 MUST 覆盖 running、reconnecting、connected、stopped、unexpected_exit 和 Stop timeout 等适用状态迁移

#### Scenario: enforcer 多 waiter 与 force refresh 验证

- **WHEN** 多个 goroutine 以重复、乱序或递增 target revision 并发请求普通 reload 或 force refresh
- **THEN** 实际 reload MUST 串行化或 coalesce，未取消 waiter MUST 只在 applied revision 不低于各自 target 后成功
- **AND** 单个 waiter cancellation MUST NOT 取消其他 waiter 所需 flight
- **AND** force refresh 在普通 reload 已开始读取数据库后加入时，engine MUST 为 force 请求重新读取一次 PostgreSQL 快照

#### Scenario: 推荐验证命令

- **WHEN** 维护者验证 RBAC policy sync 的统一并发语义
- **THEN** SHOULD 运行 `go test -race -count=20 ./user-service/internal/features/permission/application ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission/infrastructure/casbin`
- **AND** SHOULD 再运行相关包普通测试、`openspec validate document-rbac-policy-sync-semantics --strict` 和 `make user-service-architecture-lint`
