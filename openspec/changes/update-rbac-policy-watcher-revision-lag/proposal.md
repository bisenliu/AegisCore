## Why

当前 RBAC watcher 的本地 Casbin 投影已绑定数据库 policy revision，但 Pub/Sub 处理、周期补偿和 lag 仍以 Redis `CurrentVersion()` 作为远端权威值。Redis counter 缺失、落后、重建或消息丢失时，这会使数据库已有更新而 watcher 不 reload，并产生 lag 为 `0` 但授权投影仍旧的假收敛。

## What Changes

- 将 Redis policy refresh message 降级为唤醒 hint；收到消息后必须读取或校验数据库 latest policy revision，再以该 revision 驱动 reload。
- 将周期性 `CheckVersion` 改为读取数据库 latest policy revision 或等价可靠 revision source，不再用 Redis counter 判断投影是否收敛。
- 将 RBAC policy reload lag 重新定义为 `max(database_latest_policy_revision - local_applied_policy_revision, 0)`。
- 更新 watcher mismatch、reload success/failure、revision store unavailable 等 metrics reason 与结构化日志字段，使诊断语义区分消息 hint、数据库目标和本地 applied projection。
- 更新 dashboard、alert、runbook 和测试 fixture，覆盖 Pub/Sub 丢失、Redis 旧消息、Redis 故障恢复、数据库 revision 超前以及本地 reload 失败后恢复。
- **BREAKING**：移除 Redis latest/local version 差值作为稳定 lag 语义，不保留旧 Redis version lag 指标或兼容 PromQL。
- 不新增 policy revision/outbox schema，不实现 outbox dispatcher，不改造 Casbin engine 内部 revision apply gate，也不改造用户角色 cache generation。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 将 watcher 的通知处理、周期补偿和副本收敛权威来源改为数据库 latest policy revision，并禁止 Redis 状态导致旧投影假收敛。
- `runtime-observability`: 将 policy reload lag、metrics reason、日志、dashboard、alert 和 runbook 统一到 database latest 与 local applied projection revision 语义。

## Impact

- Go 代码：影响 `user-service/internal/features/permission/` 的 watcher、数据库 revision query port/adapter、composition、metrics、日志和相关测试；RBAC revision 业务语义继续留在 permission feature，不下沉到 `common/` 或 `internal/shared/`。
- PostgreSQL：只读取现有 policy revision 表中的 latest revision；不新增或修改 Ent schema、Atlas migration 或 outbox 数据模型。
- Redis：Pub/Sub 和已有 revision payload 仅作为可丢失、可重复、可乱序的唤醒 hint；Redis counter 不再是补偿或 lag 的权威来源，Redis key/channel schema 不变。
- Casbin 与安全：依赖前置 revision-aware projection change 提供的 applied revision 和防倒退 apply gate；本 change 不修改 engine 内部交换协议，但确保数据库 revision 超前时 watcher 最终触发 reload。
- 可观测性：影响 permission feature metrics、Prometheus rules、Grafana dashboard 源与 Compose provisioning 副本、runbook 和 metrics/dashboard 测试 fixture。
- API、OpenAPI 与部署：HTTP API 和 OpenAPI 不变，无数据库 migration 或新外部依赖；部署观测资产需要与应用代码同步发布。
