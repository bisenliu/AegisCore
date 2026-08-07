## Context

RBAC watcher 负责消费 Redis Pub/Sub policy refresh hint，并通过 PostgreSQL latest policy revision 做周期权威校准。当前 watcher `Start()` 直接从 `context.Background()` 派生运行 context，无法承接 Fx lifecycle `OnStart(ctx)` 中已经建立的上下文值、logger-aware context 或启动链路约束。

Outbox dispatcher 已采用 `Start(ctx context.Context)` 的显式生命周期契约。watcher 应收敛到相同模型：启动调用方提供 lifecycle context，watcher 从该 context 派生自身长期运行 context；停止调用方提供 stop context，只控制本次等待期限。

## Decisions

- `policyWatcherRunner` 改为 `Start(ctx context.Context) error`，并更新 Fx `OnStart(ctx)` 直接传入同一个 lifecycle ctx。
- `Watcher.Start(ctx)` 拒绝 nil context，避免隐式回退到 `context.Background()` 形成第二套生命周期语义。
- watcher 运行 context 使用 `context.WithCancel(context.WithoutCancel(ctx))` 派生，继承 Start context values，同时避免 Fx startup deadline 或取消信号在 OnStart 返回后意外终止长期后台循环。
- logger 注入发生在运行 context 上，`run`、`CheckVersion`、`HandlePayload`、`latestRevision`、`ObserveTargetRevision` 和 metrics 均继续使用同一条运行上下文链路。
- `Stop(ctx)` 继续接受 nil context 并回退到 `context.Background()`，因为该 ctx 只表达等待期限，不是 watcher 运行根 context。

## Non-Goals

- 不修改 `common/runtime/redispubsub.Subscriber.Start()` 签名或内部 context 模型。
- 不修改 Casbin enforcer reload flight 的上下文模型。
- 不修改 watcher 的消息串行处理、revision check 周期、缓存失效或 reload 决策语义。
- 不保留无参 watcher Start 兼容接口或 adapter。
