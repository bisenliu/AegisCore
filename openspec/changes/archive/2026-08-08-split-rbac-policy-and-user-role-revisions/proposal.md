## Why

当前 RBAC 同步把角色权限策略版本和用户角色缓存失效版本混用为同一个 `rbac_policy_revision`，导致纯用户角色绑定变更也会推进 Casbin policy target revision，并触发全量角色权限快照加载。Outbox 至少一次投递和 watcher 串行逐条处理会进一步放大重复或连续通知，使写入吞吐随策略目录规模、副本数和重复投递次数反向下降。

## What Changes

- **BREAKING** 拆分 RBAC policy revision 与 user-role revision：`policy_revision` 只表示会改变 Casbin 静态授权规则的事实，用户角色绑定变更改用独立 user-role revision 或等价提交水位。
- **BREAKING** 纯用户角色绑定变更不得调用 Casbin policy loader、不得推进 engine applied revision，也不得通过 policy reload 表示缓存失效完成。
- 调整本实例 coordinator：policy change 执行 revision-aware policy reload；user-role change 只同步失效指定用户角色缓存，缺少精确用户时执行全量用户角色缓存失效。
- 调整 watcher 消费模型：合并同批待处理通知，policy change 仅对最高未应用 policy revision 重建一次；重复、相等或乱序通知保留必要缓存失效副作用但跳过已应用 policy reload。
- 调整 outbox envelope 和持久化：明确区分 `policy_changed` 与 `user_role_changed` 的 revision 字段、幂等键和消费语义，不保留旧 envelope 或旧单 revision 分支。
- 更新测试和观测语义：验证 100 条重复或连续通知下 policy loader 调用次数有常数上界，纯 user-role 变更不查询规则全集，policy reload lag 继续只表示 PostgreSQL latest policy revision 与 Casbin actual applied revision 的差值。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 修改 RBAC policy revision、user-role cache invalidation、outbox envelope、watcher 幂等消费与副本收敛的稳定行为。
- `runtime-observability`: 修改 RBAC policy reload lag、watcher 日志和指标的 revision 解释，确保 user-role revision 不进入 Casbin policy lag 语义。

## Impact

- 代码：影响 `user-service/internal/features/permission/application`、`user-service/internal/features/permission/infrastructure/redis`、`user-service/internal/features/permission/infrastructure/casbin`、`user-service/internal/features/permission/infrastructure/postgres`、`user-service/internal/features/role/application`、`user-service/internal/features/role/infrastructure/postgres` 及相关测试。
- 数据库：需要调整 Ent schema 和 Atlas migration，新增或改造 user-role revision 持久化结构，并停止用 `rbac_policy_revision` 表记录纯用户角色绑定变更。
- Redis/Outbox：`policy_changed` 与 `user_role_changed` envelope 字段和幂等键语义变化；不保留兼容解析旧 payload 的分支。
- 观测：RBAC reload、watcher lag、dispatcher kind 计数、日志字段和 dashboard/runbook 需同步区分 policy revision 与 user-role revision。
- API/OpenAPI：HTTP 路径和响应契约不变；角色/用户角色写接口的同步副作用和内部 revision 语义变化。
- 运维：发布需包含数据库 migration，并按既有顺序先执行 migration，再执行 RBAC seed，最后滚动 user-service 副本。
