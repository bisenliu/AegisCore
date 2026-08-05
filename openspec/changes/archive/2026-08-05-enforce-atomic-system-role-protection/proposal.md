## Why

当前系统角色保护分散在 application 层的事务外预检查中，既未覆盖 `Description`，也未覆盖角色权限 add、replace、remove，且 PostgreSQL 写边界没有原子强制 `IsSystem=false`。具备角色管理权限的调用方因此可以通过正式 API 篡改系统角色元数据和权限基线，并在并发 seed 场景中利用检查与提交之间的窗口覆盖受信写入，生产授权基线无法保证。

## What Changes

- **BREAKING**：所有公开角色 metadata、状态和角色权限写请求只要目标为系统角色，就统一返回 `ErrSystemRoleProtected` 对应的 `409 Conflict`，不再接受 description-only、幂等或其他形式的系统角色普通写入。
- 将系统角色不变量收敛到普通 PostgreSQL 写端口：在同一事务内锁定目标角色、检查 `IsSystem`，并在任何角色、绑定、policy revision 或 outbox 写入前拒绝系统角色。
- 删除 `RoleMutation`、`ProtectSystemMutation` 和 role command 中的事务外系统角色保护分支；普通写路径不保留旧预检查、feature flag、双路径或回退逻辑。
- 保留 `SeedRoleStore` 与 `SeedRolePermissionStore` 作为唯一受信系统角色写端口，普通 HTTP use case 不得调用这些端口。
- 补充真实 PostgreSQL 集成测试、command/HTTP 测试和 seed 并发测试，证明拒绝路径不会改变角色、绑定、revision、outbox，也不会发送 policy change 通知。
- 更新角色写接口的 OpenAPI 错误注解和生成物，明确系统角色保护使用 `409 Conflict`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：明确系统角色受保护字段、权限绑定、错误语义、事务原子性、受信 seed 边界以及拒绝写入时必须保持不变的数据库事实。

## Impact

- 代码：`user-service/internal/features/role/domain/`、`application/command/`、`application/ports.go`、`infrastructure/postgres/`、`transport/http/` 及相关 mocks 和测试。
- API：现有角色 metadata、状态和权限绑定路由不增删；系统角色目标从部分成功或幂等成功收紧为稳定 `409 Conflict`，不提供兼容开关。
- OpenAPI：更新错误响应注解并重新生成 `user-service/docs/openapi.go`、`openapi.json`、`openapi.yaml`。
- 数据库：不修改 Ent schema 或 Atlas migration；使用现有 PostgreSQL 行锁和事务能力。
- 安全与授权：阻止系统角色 metadata 和权限基线通过公开 API 漂移；拒绝路径不推进 policy revision，不创建 outbox event，也不触发 policy reload 通知。
- 边界：不修改 `common/`、`internal/shared/rbacbaseline`、Casbin wildcard 语义、Redis policy sync、部署或观测资产。
