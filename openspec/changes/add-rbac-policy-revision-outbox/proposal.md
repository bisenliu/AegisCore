## Why

当前在线 RBAC 写入在 PostgreSQL mutation 提交后，仍依赖独立的本地 reload、Redis version 更新和 Pub/Sub 发布完成同步；数据库已提交而后续步骤失败时，变更通知可能丢失，且 Redis counter 被赋予了不适合作为业务权威版本的职责。需要以 PostgreSQL 中单调递增的 `policy_revision` 和事务 outbox 建立可恢复的提交事实，使业务 mutation、revision 分配和待投递事件原子落库。

## What Changes

- 新增 RBAC policy revision 与 outbox 的 Ent schema 和 Atlas SQL migration，持久化单调 revision、事件类型、原因、对象 ID、状态、重试信息、幂等键及时间字段。
- 调整角色、角色权限和用户角色在线写侧的 PostgreSQL 事务边界，使业务 mutation、revision 记录和 outbox event 在同一事务中提交；任一步骤或 commit 失败均回滚全部写入。
- 以数据库 revision 作为 policy version 的唯一权威来源；Redis 后续只能消费数据库 revision 进行加速通知，不再分配或权威保存版本。
- 调整在线写成功语义：API mutation 的可靠恢复由已提交 outbox 保证，不再以本地 reload 或 Redis publish 成功作为数据库 mutation 可恢复性的前提。
- 增加业务写失败、revision 写失败、outbox 写失败和 commit 失败的事务回滚测试。
- 明确本 change 不实现 outbox dispatcher、revision-aware Casbin reload、watcher/Redis Pub/Sub 协议改造或 lag 指标。
- **BREAKING**：移除旧 Redis counter 作为权威 policy version 的契约，不提供兼容分支。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：修改在线 RBAC 写后同步、持久化 policy revision、事务 outbox 恢复和 Redis 权威性要求。

## Impact

- Go 代码：影响 `user-service/internal/features/role/` 的角色、角色权限和用户角色写侧 application/infrastructure，以及 `user-service/internal/features/permission/` 中被写侧依赖的 policy sync 契约与 composition 接线；不向 `common/`、`internal/shared/` 或 `internal/integration/` 下沉 RBAC 业务模型。
- 数据库：新增 Ent schema、Ent 生成代码和 Atlas migration；migration 必须在新写侧代码发布前受控执行。
- API：HTTP 路由和请求/响应结构不变，但成功与同步失败语义调整为数据库事务提交后具备 outbox 恢复能力，不再依赖 Redis publish 成功保证可靠性。
- Redis/Casbin：Redis 不再是 policy version 权威来源；本 change 不实现 dispatcher、消息协议、watcher 或 Casbin reload 消费侧改造。
- OpenAPI、部署清单和观测资产：无契约或资产变更；安全边界增强为 PostgreSQL 原子提交与 fail-closed 事务错误处理。
- 规格与验证：更新 `rbac-access-control` delta，运行 Ent/Atlas 生成与迁移校验、相关单元和事务测试、architecture lint、lint 与 verify。
