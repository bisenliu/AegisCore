## Why

当前 `Dispatcher.DispatchOnce` 只返回合并后的 `error`，调用方和测试无法稳定区分 batch claim、单条 publish/ack/fail、backlog/status 刷新等不同失败阶段。部分成功 batch 中已经成功 publish 并 ack 的事件会和后续失败共享同一个错误结果，容易误读为整个 batch 未投递。

## What Changes

- **BREAKING**：将 `DispatchOnce` 从 error-only 返回语义调整为结构化结果，明确 batch 内已 claim、已投递、已确认、已重试和失败事件计数。
- 将 claim batch、dispatch batch、refresh backlog/status、finalize result 从 `DispatchOnce` 主流程拆分为私有步骤。
- 让 claim 失败、单条 publish/ack/fail 失败、claim lost、backlog/status 刷新失败在结果或结构化错误中可判别。
- 保持单条失败不阻塞同 batch 后续事件投递；保持 `ctx` canceled 时不主动 Ack/Fail 当前未完成 claim，继续依赖 lease recovery。
- 更新 dispatcher 单元测试覆盖全成功、部分 publish 失败、ack 失败、claim 失败、backlog/status 失败和 `ctx` canceled 返回语义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：补充 RBAC policy outbox dispatcher 单次 dispatch 的结构化结果与部分成功语义。

## Impact

- 影响代码：`user-service/internal/features/permission/application/dispatcher.go`、`outbox.go` 及 dispatcher 单元测试。
- 不影响公开 HTTP API、OpenAPI、数据库 schema、migration、Redis 消息格式、outbox event payload 或 policy revision 发布协议。
- `Start(ctx)` 后台循环需要适配新的 `DispatchOnce` 返回值，但 lifecycle 行为不在本 change 中扩展。
