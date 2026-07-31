## Why

用户角色缓存当前存在写后失效期间被 inflight 旧回源结果重新写入的风险，可能导致 Add、Remove 或 Replace 用户角色绑定成功后，后续授权继续命中失效前的角色集合。该问题影响 RBAC 授权最终状态的可预测性，需要在缓存失效、并发回源和重放写操作之间建立明确的顺序门禁。

## What Changes

- 为用户角色缓存项引入 per-user generation 或等价 revision 门禁，使缓存 load 开始时捕获版本，写入前再次校验版本。
- 调整 `InvalidateUserRole`，确保指定用户的 generation 先提升，再删除对应缓存项。
- 调整 `InvalidateAllUserRoles`，确保全量 generation 先提升，再删除所有用户角色缓存项，并抑制所有失效前启动的旧 load 写回。
- 调整 `RolesForUser` 回源流程，保证失效后才完成的旧回源结果不会写入缓存；cache disabled 模式保持直接回源和 fail-closed 语义。
- 补充用户角色 cache miss 与写并发、失效后旧 load 回填、全量失效、cache disabled 模式和 race 场景测试。
- 不改变 Casbin policy 全量 reload 的 revision 门禁，不引入 outbox dispatcher 或 watcher revision lag 机制。
- 不把 RBAC 业务 revision 语义下沉到 `common/runtime/localcache` 公共 API；如需包装，优先放在 permission infrastructure 边界内。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确用户角色缓存写后失效、inflight load 抑制、全量失效和 fail-closed 行为要求。

## Impact

- 影响 `user-service/internal/features/permission/` 中用户角色 resolver 或 cache wrapper 的缓存失效和回源写入顺序。
- 影响 role/user-role 绑定写操作成功后的缓存失效路径，以及并发授权读取用户角色集合时的最终一致性语义。
- 需要更新 `openspec/changes/harden-user-role-cache-invalidation-ordering/specs/rbac-access-control/spec.md`。
- 需要补充 permission 或 role 相关并发测试与 race 测试；不改变 HTTP API、OpenAPI、数据库 schema、部署资产或 `common/runtime/localcache` 公共契约。
