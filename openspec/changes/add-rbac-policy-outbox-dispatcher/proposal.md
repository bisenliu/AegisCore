## Why

RBAC mutation 已能将数据库 `policy_revision` 与 pending outbox event 原子持久化，但当前没有后台组件恢复投递这些事件；进程在提交后崩溃、Redis 短暂不可用或 publish 失败时，副本通知仍可能长期停滞。需要实现基于 PostgreSQL 持久事实的可靠 dispatcher，使 Redis 退化为可丢失、可重放的通知加速层，并让已提交 mutation 无需新写即可恢复传播。

## What Changes

- 新增 RBAC policy outbox dispatcher application service 与 PostgreSQL store，按 revision 扫描到期的 pending/failed event，并通过带 lease 的并发安全 claim、publish、ack 和失败记录协议完成可靠投递。
- 为失败事件记录 attempt、稳定错误摘要和下一次重试时间，使用可配置退避；进程重启或 claim 超时后可重新处理未完成事件，成功 ack 保持幂等。
- 调整 Redis policy refresh 消息，使其携带数据库 `policy_revision`、change kind、reason 及可选 `user_id`、`role_id`、`permission_id`，且不再保留旧 `INCR` counter 或旧消息协议 fallback。
- 在 user-service lifecycle 中显式启动和停止 dispatcher，并增加 poll interval、batch size、claim timeout 与 retry backoff 配置和校验。
- 增加 dispatcher 的低基数 metrics、结构化日志和只读 health/readiness 状态，覆盖积压、投递结果、重试和运行状态。
- 增加 Redis 故障恢复、publish 失败、进程重启、重复投递、claim 超时和多 dispatcher 并发竞争测试。
- **BREAKING**：Redis Pub/Sub consumer 只接受携带数据库 revision 与变更元数据的新消息，不兼容旧 counter 或旧消息形状。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：新增持久 outbox 的 claim、发布、ack、失败重试和崩溃恢复契约，并将 Redis 明确定义为数据库 revision 通知的非权威加速层。
- `runtime-observability`：新增 dispatcher 配置、生命周期、低基数 metrics、日志以及 health/readiness 只读状态要求。

## Impact

- Go 代码：影响 `user-service/internal/features/permission/` 的 application、PostgreSQL/Redis infrastructure 与 composition/lifecycle 接线，并消费前置 change 在 role persistence 中建立的 RBAC outbox 表；不得将 RBAC 事件模型下沉到 `common/` 或通用 eventbus。
- 数据库：不改变 mutation 与 outbox 的事务写入模型；可能需要 Atlas migration 补充 claim lease 字段或扫描索引，具体以现有 schema 与 dispatcher 协议差距为准。
- Redis/Casbin：Redis 只传播数据库 revision 通知，短暂不可用不丢失 PostgreSQL 中的待投递事实；本 change 不实现 revision-aware Casbin reload，也不改变 Casbin policy 数据模型。
- 配置与运行时：新增 user-service 私有 dispatcher 配置和 Fx lifecycle；dispatcher 停止、异常或积压状态进入 RBAC 只读 health contract 与 readiness 判断。
- 可观测性：新增低基数 Prometheus 指标和结构化日志；如增加 dashboard、alert 或 runbook 查询，必须同步部署观测资产并运行 drift 校验。
- API/OpenAPI：HTTP 业务路由、请求和响应结构不变，不产生 OpenAPI 契约变更。
- 兼容性：不保留旧 Redis `INCR` counter 或旧 Pub/Sub 消息 fallback；滚动发布必须协调 dispatcher、publisher 与 watcher 的协议切换。
