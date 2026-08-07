## Context

RBAC policy sync 的生产行为已经由 `rbac-access-control` 主规格定义：数据库 policy revision 是权威事实，Redis Pub/Sub 只做加速，watcher 负责订阅与周期校准，Casbin engine 负责 revision-aware reload 与 fail-closed 状态。当前风险集中在实现层：watcher 和 engine 都包含 goroutine、context cancellation、coalescing、waiter、fake Redis message channel 和状态快照，普通单元测试不足以发现数据竞争、Stop 超时、leader cancel 或重复通知导致的状态倒退。

本 change 只强化测试和最小可测试性，不改变 HTTP API、数据库 schema、OpenAPI、部署清单、Redis/PostgreSQL 协议或生产授权语义。代码归属保持在 permission feature 内：Redis watcher 测试位于 `user-service/internal/features/permission/infrastructure/redis`，Casbin enforcer 测试位于 `user-service/internal/features/permission/infrastructure/casbin`。不得把 RBAC 业务语义、revision envelope、测试 fake 或 watcher 状态适配下沉到 `common`、`internal/shared` 或 `integration`。

## Goals / Non-Goals

**Goals:**

- 用 deterministic fake、channel gate、context deadline 和 `sync` primitive 覆盖 watcher/enforcer 的并发 reload、Stop、重订阅和 cancellation 语义。
- 在 `go test -race` 下验证 permission sync 相关包，目标可被开发者和 CI 单独运行。
- 将 watcher 状态断言固化为测试：reconnecting 时 lifecycle 仍为 running，Stop 完成后 lifecycle stopped 且 subscription stopped，reconcile cancellation 不记为业务 failure。
- 验证 enforcer 多 waiter、leader cancel、engine root cancel、force refresh 加入当前 flight 和 applied revision 不倒退语义。

**Non-Goals:**

- 不修改公开业务接口、HTTP route、OpenAPI、Ent schema、Atlas migration、部署或观测资产。
- 不引入真实 Redis/PostgreSQL 依赖的非确定性压力测试。
- 不新增跨 feature、`common` 或 `shared` 层的生产抽象。
- 不改变 watcher、dispatcher、subscriber、enforcer、policy loader 或 cache resolver 的生产行为。

## Decisions

1. 测试优先使用包内 fake 和可控同步点，而不是 `sleep` 驱动的时序假设。

   备选方案是用真实 Redis/miniredis 和固定 sleep 做压力测试。该方案更接近协议层，但对本 change 的核心竞态控制力弱，容易慢且不稳定。包内 fake 可以精确阻塞 revision source、message receive、reload engine 和 policy loader，并让 race detector 覆盖真实共享状态。

2. watcher 测试围绕结构化 status port 和公开 lifecycle 方法断言，不引入旧 `Running()`/`LastError()` 兼容接口。

   备选方案是为测试新增私有状态读取分支。该方案会绕开规格要求的结构化状态来源，且容易形成只为测试服务的生产分支。应优先使用现有 status snapshot；若现有 fake 无法表达必要事件，仅增加测试文件内 helper 或最小内部 hook。

3. enforcer 测试通过 fake loader/reload gate 构造并发场景，并用最终 `applied revision`、waiter 返回值和 root cancellation 断言收敛。

   备选方案是检查内部 goroutine 数或具体 reload 次数。reload coalescing 允许实现自由度，过度绑定内部次数会让测试脆弱。测试应验证规范结果：等待方只在目标 revision 达成时成功，单个 waiter cancel 不取消共享 reload，root cancel 后未完成等待方返回取消错误，force refresh 必须触发到达后的新快照读取。

4. 增加服务私有 Makefile 目标或文档化命令时，必须带 `user-service` 前缀。

   根 `Makefile` 不新增 `seed-rbac` 这类无服务上下文目标；同理 race test 目标应命名为 `user-service-rbac-sync-race-test` 或等价带服务上下文的名称，并执行：

   ```bash
   go test -race ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission/infrastructure/casbin
   ```

## Risks / Trade-offs

- [Risk] 并发测试过度绑定内部实现，后续合理重构会频繁改测试。→ Mitigation：断言规格语义和外部可观察状态，避免断言具体 goroutine 调度或 reload 次数，除非该次数本身是规范要求。
- [Risk] race 测试运行时间增加。→ Mitigation：限定在两个 permission sync 包，使用可控同步点和短 deadline，不纳入真实外部服务。
- [Risk] 为测试添加 hook 可能污染生产边界。→ Mitigation：优先使用测试文件 fake；如必须调整生产 constructor 或接口，保持最小、包内、无业务语义，并确认不进入 `common` 或 `shared`。
- [Risk] Stop 超时测试可能留下 goroutine。→ Mitigation：每个阻塞 fake 必须提供 unblock/cleanup 路径，测试用 `t.Cleanup` 关闭 channel 并等待 goroutine 退出。

## Migration Plan

无需数据库、配置、API 或部署迁移。合入后开发者可单独运行新增 race 目标；如测试暴露现有实现竞态，应在本 change 范围内修复 permission sync 相关包，不改变生产契约。

回滚方式是移除本 change 增加的测试、最小测试辅助和 Makefile 目标；由于不改变持久化或外部接口，回滚不需要数据修复。

## Open Questions

无。
