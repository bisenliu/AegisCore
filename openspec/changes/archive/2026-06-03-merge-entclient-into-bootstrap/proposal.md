## Why

`user-services/internal/entclient` 目前只包含用户服务 Ent client 的 Fx provider，职责与 `user-services/internal/bootstrap` 中的 Redis/PostgreSQL 运行时依赖装配高度重叠。将该 provider 合入 `bootstrap` 可以减少内部包数量，使 `shared-infrastructure` 的服务侧运行时 wiring 更集中、更容易维护。

## What Changes

- 将 `user-services/internal/entclient/provider.go` 中的 Ent client provider 类型和构造逻辑迁移到 `user-services/internal/bootstrap/`。
- 更新用户服务 Fx module，直接使用 `bootstrap` 包内的 Ent client provider，不再导入 `internal/entclient`。
- 删除空的 `user-services/internal/entclient/` 包目录。
- 保持具名 `user_db` 与 `common_db` Ent client、PostgreSQL 连接池复用和 Fx 停止时关闭 Ent clients 的运行时行为不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 调整用户服务 Ent client provider 的代码组织契约，从独立 `internal/entclient` 包迁移到 `internal/bootstrap`，同时保持现有运行时依赖行为不变。

## Impact

- 受影响代码：`user-services/internal/bootstrap/`、`user-services/internal/entclient/` 以及引用 Ent client provider 的 Fx 装配。
- API 兼容性：不改变 HTTP API、响应信封、错误码或认证行为。
- 配置兼容性：不改变 YAML key、`AEGISCORE_` 环境变量、`postgres.user_db`、`postgres.common_db` 或 Redis/PostgreSQL 命名实例契约。
- 数据模型与迁移：不修改 Ent schema、生成代码或 Atlas migration。
