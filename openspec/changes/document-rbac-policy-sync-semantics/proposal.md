## Why

RBAC policy sync 的 dispatcher、watcher、subscriber 与 enforcer reload flight 正在通过多个不兼容 change 分别收敛，但主规格尚未统一描述它们共享的生命周期、部分成功、终态和异常可观测语义。若缺少统一基线，后续归档与维护只能依赖代码细节推断后台同步链路，容易产生互相冲突的实现和测试。

## What Changes

- **BREAKING**：统一规定 RBAC policy sync 后台链路必须由服务 lifecycle root context 驱动，不再允许旧 `Start()`、`context.Background()` root 或由单个等待方 context 独占共享 reload flight 生命周期的兼容语义。
- **BREAKING**：统一规定 dispatcher 单次 batch 返回结构化结果并保留 partial success；单条 publish、ack、fail 或 claim lost 不得阻断同 batch 后续事件，error-only `DispatchOnce` 不再作为可选兼容行为。
- 明确 watcher 在 Redis 断连、重订阅、正常停止、`Stop` 超时和 reconcile cancellation 下的 lifecycle、subscription 与 reconcile final state。
- 明确 dispatcher 后台循环 panic recovery 必须记录 recovered value 与 stack，并将 dispatcher 转入 `unexpected_exit`，不得伪装为正常停止或普通 dispatch failure。
- 明确 subscriber 与 watcher 的断连恢复责任边界，以及 enforcer reload flight 的 root cancellation、waiter cancellation、coalescing 和 applied revision 约束。
- 增加 RBAC policy sync 的 race/stress 规格门禁与推荐测试命令，要求使用 deterministic fake 和可控同步 primitive 验证并发、关闭与 cancellation 语义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：补充 RBAC policy sync 的 dispatcher batch、lifecycle root context、watcher/subscriber final state、enforcer reload flight、panic observability 和 race/stress 验证要求。

## Impact

- 本 change 仅创建 OpenSpec proposal、design、spec delta 和 tasks，为现有 RBAC policy sync 相关 change 的后续归档提供统一规格基线。
- 规格涉及的实现边界包括 `user-service/internal/features/permission/application/`、`user-service/internal/features/permission/infrastructure/redis/`、`user-service/internal/features/permission/infrastructure/casbin/`、permission feature lifecycle wiring，以及 `common/runtime/redispubsub/` 的通用 subscriber primitive。
- 不修改 Go 业务代码、测试代码、公开 HTTP API、OpenAPI、数据库 schema、migration、Redis 消息 envelope、部署资产或产品功能。
- 不新增 capability；所有 requirement delta 归入现有 `rbac-access-control`。
