## Why

当前在线 RBAC mutation 已能把业务数据、policy revision 与 outbox event 原子提交，但 revision 仍由 PostgreSQL identity sequence 分配。sequence 的数值顺序不等于并发事务的提交可见顺序：较大 revision 可以先提交并被副本加载，较小 revision 随后提交后又被 max revision 门禁忽略，导致 lag 显示为零但授权快照永久缺失变更。同时，业务事务提交后的本地 reload 失败仍会让 API 返回错误，使调用方无法依据响应判断 mutation 是否已经提交。

这些缺口会破坏现有 transactional outbox、revision-aware projection 和故障恢复协议的正确性，必须在上线前修复并以真实 PostgreSQL 并发测试验证。

## What Changes

- 为在线 RBAC mutation 增加事务内单行 revision counter，使 revision 分配顺序与事务提交可见顺序一致。
- 让每条 `policy_changed` 通知强制从 PostgreSQL 当前权威快照刷新 Casbin projection；强制刷新可以 coalesce，但不能因 target revision 已应用而跳过。
- 在线 mutation 一旦完成数据库提交，API 按提交成功返回；本地 projection 同步失败记录错误并保持 fail-closed，由 outbox 和 watcher 自动恢复，不再把已提交 mutation 表达为失败。
- 绑定写操作在提交事务内构造响应集合，避免提交后读取失败再次造成 API 结果歧义。
- 增加真实 PostgreSQL 提交逆序、100 并发 mutation、Redis 恢复、outbox 重放和多 projection 收敛测试。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 强化 policy revision 提交顺序、旧或重复全局通知刷新、写 API 提交结果和并发故障验收契约。

## Impact

- Go 代码：影响 `user-service/internal/features/role/` 的事务写侧和 `user-service/internal/features/permission/` 的 reload/watcher 协议。
- 数据库：新增 RBAC revision counter Ent schema、生成代码和 Atlas SQL migration；现有 revision/outbox 数据保持不变。
- HTTP API：路由、请求、响应 JSON 和成功状态码不变；行为调整为数据库已提交后不再因本地同步失败返回错误。
- OpenAPI：接口形态不变，生成物预计无变化，仍需运行生成检查。
- 安全：本地 reload 失败时继续 fail-closed；Redis 仍只是通知加速层。
- 不影响 `common/`、`internal/shared/`、`internal/integration/`、部署清单和观测资产。
