## Context

RBAC policy sync 由 permission feature 的 outbox dispatcher、Redis watcher、Casbin enforcer reload engine 和 `common/runtime/redispubsub` subscriber primitive 协同完成。当前主规格已经描述 PostgreSQL revision 权威事实、Redis 加速通知、outbox 至少一次投递和 watcher 自恢复，但 dispatcher 结构化 partial success、显式 lifecycle root context、watcher 多维 final state、panic observability 与 race/stress 门禁分散在多个待实施或待归档 change 中，且部分旧实现契约互不兼容。

本 change 只沉淀统一规格，不修改实现。受影响的长期边界是 `openspec/specs/rbac-access-control/spec.md`；实现归属仍分别位于 `user-service/internal/features/permission/application/`、`user-service/internal/features/permission/infrastructure/redis/`、`user-service/internal/features/permission/infrastructure/casbin/` 和 `common/runtime/redispubsub/`。`common` 只拥有无 RBAC 业务语义的 subscriber lifecycle primitive，policy revision、reconcile、dispatcher status 和 enforcer applied revision 仍由 permission feature 拥有。

## Goals / Non-Goals

**Goals:**

- 为 dispatcher batch 建立结构化结果、阶段化错误和 partial success 的唯一规格语义。
- 为 dispatcher、watcher、subscriber 与 enforcer reload flight 建立同一服务 lifecycle root context 约束。
- 明确 watcher 在断连、重订阅、正常停止、Stop 超时、reconcile cancellation 和异常退出下的 lifecycle、subscription、reconcile final state。
- 明确 dispatcher panic recovery 的日志字段、状态迁移和幂等停止语义。
- 给出可重复、无真实外部依赖的 race/stress 验证要求与推荐命令。

**Non-Goals:**

- 不修改 Go 业务代码、测试代码、Makefile、部署资产或观测面板。
- 不改变 HTTP API、OpenAPI、数据库 schema、migration、Redis 消息 envelope 或 RBAC 产品能力。
- 不新增 capability，也不把 RBAC 业务状态下沉到 `common`。
- 不保留旧无参 `Start()`、background root、error-only `DispatchOnce` 或单 waiter context 控制共享 reload flight 的兼容分支。
- 不执行 OpenSpec archive。

## Decisions

1. 以 `rbac-access-control` 中新增的统一 requirements 表达跨 change 基线。

   新 requirements 直接描述长期可验证行为，不复制一次性重构步骤，也不新增 capability。选择新增 requirements 而不是部分修改已有 requirement，是为了避免 archive 时以不完整 `MODIFIED` block 覆盖主规格现有场景；后续相关 change 归档时必须与本基线一致，不得重新引入旧兼容语义。备选方案是分别保留多个 change 的局部 delta；该方案无法解决语义冲突，也继续要求维护者跨目录推断完整链路。

2. lifecycle root context 是后台同步链路唯一所有者。

   permission lifecycle 在 `OnStart(ctx)` 接收服务 root context，dispatcher、watcher 和 subscriber 只能从该 context 派生运行 context；enforcer 共享 reload flight 同样受 engine root context 约束。调用方 `Stop(ctx)` 的 context 只限制等待退出的期限，单个 reload waiter context 只取消该 waiter。备选方案是使用 `context.Background()` 或首个 waiter context 作为 root；前者会泄漏 shutdown，后者会让无关 waiter 互相取消。

3. dispatcher batch 将工作结果与失败阶段分离。

   单次 dispatch 必须暴露 claimed、delivered、acknowledged、retried、failed 和 status refresh 等结构化结果；错误必须能区分 claim、publish、ack、failure record、claim lost、status refresh 与 cancellation。单条事件失败不阻断同 batch 后续未取消事件，已经成功的 publish/ack 不因最终返回 error 而被抹除。备选方案是继续返回合并 error；该方案无法表达 partial success，并会误导重试和测试。

4. watcher 状态拆分为 lifecycle、subscription 与 reconcile 三个维度。

   Redis 断连只把 subscription 置为 reconnecting，watcher lifecycle 继续 running，PostgreSQL revision reconcile 继续执行；重订阅确认后 subscription 恢复 connected 并清除当前订阅错误。正常 root cancellation 导致的 reconcile cancellation 不计为业务失败。`Stop` 超时只表示调用方等待超时，不能提前伪造 stopped；后台 loop 真正退出后才进入 stopped final state。备选方案是单一 running/error 布尔状态；该方案无法区分可恢复断连、停止中和异常退出。

5. dispatcher panic 使用 `unexpected_exit` fail-closed 终态。

   recovery 必须记录 `error_category=unexpected_exit`、`recover()` 捕获的 recovered value 和 stack trace，并原子完成 running=false、运行指标归零、done 关闭与最近错误分类更新。panic 后不自动重启，后续 `Stop(ctx)` 保持幂等。备选方案是仅记录 error 或吞掉 panic 后继续 loop；前者缺少根因证据，后者可能在损坏状态下继续投递。

6. race/stress 测试验证规格语义而非 goroutine 调度细节。

   测试使用 deterministic fake、channel、barrier 或等价同步 primitive，覆盖 watcher 断连重订阅与 Stop 竞态、dispatcher partial success 与 panic finalization、enforcer 多 waiter coalescing、root/waiter cancellation 和 force refresh。推荐命令限定 permission sync 相关包并使用 `-race -count` 重复运行，避免依赖真实 Redis 或 PostgreSQL。

## Risks / Trade-offs

- [Risk] 新统一 requirement 与待归档 change 的局部 delta 存在文字重复或冲突。→ Mitigation：以后续归档结果必须满足本 change 的 MUST 语义为准；归档前对照统一基线删除旧兼容表述。
- [Risk] 对 final state 的要求过度绑定当前字段名。→ Mitigation：规格只固定稳定状态语义；除 `error_category=unexpected_exit`、`recovered` 和 stack trace 等可观测契约外，允许等价结构化字段。
- [Risk] race/stress 命令增加本地验证耗时。→ Mitigation：限定到 permission application、Redis watcher 和 Casbin enforcer 包，使用可控同步点和有限重复次数。
- [Risk] `common/runtime/redispubsub` 与 permission watcher 的责任再次混淆。→ Mitigation：规格明确 common 只负责订阅 primitive 的连接与取消，RBAC revision、reconcile 和业务状态始终留在 permission feature。

## Migration Plan

本 change 无代码、数据、配置、API 或部署迁移。完成 artifacts 后运行 OpenSpec 校验和 `make user-service-architecture-lint`；未来实施或归档 dispatcher、watcher、subscriber、enforcer 相关 change 时，以本 change 的统一 requirements 作为一致性检查基线。

如需回滚，只移除 `openspec/changes/document-rbac-policy-sync-semantics/`，不会影响运行时或持久化数据。按本任务约束不执行 archive。

## Open Questions

无。
