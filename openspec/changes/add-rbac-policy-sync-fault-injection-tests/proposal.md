## Why

RBAC 写后同步覆盖数据库 revision、同步通知、Casbin projection 与用户角色 cache，任一环节出现延迟、乱序、重放或 Redis 故障都可能导致授权漂移或假收敛。需要用故障注入验收套件固化 P0 问题复现条件，防止后续重构在同步可靠性上回退。

## What Changes

- 新增 RBAC policy sync 故障注入测试套件，覆盖数据库 revision、同步通知、dispatcher、watcher、Casbin projection 与用户角色 cache 的端到端收敛行为。
- 新增或完善 integration/package-level harness，用可控通道、barrier、eventually 条件与 deadline 注入 loader 阻塞、Redis publish 失败、dispatcher 重试、watcher 消息乱序和 cache loader 延迟。
- 增加数据库提交后 Redis 故障并恢复、无需新写所有副本 lag 归零的验收场景。
- 增加两次 reload 逆序完成时最终策略仍对应最新 revision 的验收场景。
- 增加 Add/Remove/Replace outbox 重放不丢通知、不产生非幂等破坏的验收场景。
- 增加 100 并发 RBAC 写及最终 applied revision 收敛的验收场景。
- 更新 `docs/TESTING.md` 或相关测试说明，记录每个故障注入场景对应风险、运行方式和预期收敛条件。
- 不实现业务修复逻辑、不引入新的同步协议或 schema、不改变生产 runtime 行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 增加 RBAC 写后同步故障注入验收场景，明确在 Redis 故障恢复、reload 乱序、outbox 重放与高并发写入下的最终授权投影收敛要求。

## Impact

- 影响测试代码：`user-service/internal/features/permission/`、`user-service/internal/features/role/` 或对应 integration 测试目录中与 policy sync、Casbin projection、用户角色 cache 相关的测试与 harness。
- 影响文档：`docs/TESTING.md` 或现有测试说明需要新增故障注入场景、风险映射、依赖开关和运行命令。
- 影响规格：`openspec/changes/add-rbac-policy-sync-fault-injection-tests/specs/rbac-access-control/spec.md` 记录新增验收场景。
- 不影响 HTTP API、OpenAPI、数据库 schema、migration、生产配置、部署资产或生产 runtime 行为。
