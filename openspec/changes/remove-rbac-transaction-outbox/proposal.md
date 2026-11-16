## Why

AegisCore 是后端服务底座，不是可靠消息框架。RBAC 事务 outbox、claim lease 和 dispatcher 只为跨副本加速通知提供至少一次投递，却把投递状态机、配置、健康检查和指标带进 user-service。PostgreSQL revision 已经是授权事实，Redis Pub/Sub 也只是可丢失 hint，底座不需要再维护一套 outbox。

## What Changes

- **BREAKING** 移除 RBAC policy/user-role 事务 outbox、dispatcher、claim/ack/fail 状态机及其配置、健康检查和指标。
- 在线写入仍与对应 revision 同事务提交；提交后本实例继续立即 reload 或失效缓存，并 best-effort 发布 Redis Pub/Sub hint。
- watcher 继续用 policy revision 补偿 Casbin 投影；user-role 变更改为按数据库 revision 水位读取受影响用户并失效缓存。
- 删除 `rbac_policy_outbox_events` 及 revision 上的 outbox edge，并生成 Atlas migration。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 以数据库 revision 和 watcher 补偿替代事务 outbox 与可靠 dispatcher。
- `runtime-observability`: 移除 outbox dispatcher 健康检查和指标，保留 policy watcher 与投影 lag。

## Impact

影响 role/permission 写入与同步、Ent schema、Atlas migration、RBAC 配置、Nacos 示例配置、健康检查、Prometheus 指标和相关文档。不改变 HTTP API、认证契约或 `common/` 原语；`common/` 不得新增 outbox 框架。Redis 故障时副本通过数据库 revision 最终收敛，不再承诺 outbox 至少一次投递。
