## Why

role feature 的基础设施 store constructor 当前通过 `fx.In` 和 `name:"primary_db"` 暴露 Fx/Dig metadata，使普通 Go 装配、单元测试和消费侧 port 边界被 DI 框架细节污染。现在需要把 role adapter 边界收敛为显式 Ent client 与窄 port 注入，避免 production package 继续依赖 Fx/Dig，并让架构检查能持续防止回归。

## What Changes

- **BREAKING**：移除 `RoleStore`、`RolePermissionStore`、`UserRoleStore` 基础设施 constructor 的 `fx.In` Params 和 `name:"primary_db"` tag，不保留旧 constructor 或兼容 wrapper。
- 将三个 store 的构造 API 改为直接接收显式 `*ent.Client`；`RolePermissionStore` 继续通过显式参数接收消费侧 `PermissionLookup` 窄 port。
- 调整 user-service 生产 Fx composition，使 composition 层负责把具名 primary PostgreSQL Ent client 适配给无 DI metadata 的 role infrastructure constructor。
- 更新 role feature 相关测试，使用普通 Go 参数直接装配 store，并覆盖新 constructor 签名。
- 补充架构检查，禁止 `user-service/internal/features/role` 的 domain、application、infrastructure 和 transport 生产包导入 Fx/Dig；允许 role feature 的 `fx.go`、`fx_test.go` 继续作为 composition 边界存在。
- 不删除 role feature 的 Fx module，不改变角色、角色权限、用户角色 HTTP API、RBAC baseline、Ent schema、migration、watcher、Casbin initial load 或 Redis policy sync 生命周期。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：将 role adapter 边界要求从可携带 DI metadata 的基础设施 constructor 收敛为显式 Ent client 与消费侧 port 注入，并要求架构检查阻止 role 生产包重新导入 Fx/Dig。

## Impact

- 受影响代码：`user-service/internal/features/role/` 的 infrastructure store constructor、feature Fx composition、相关测试与架构 lint 脚本或规则。
- 依赖边界：role infrastructure 生产代码不再导入 `go.uber.org/fx` 或 `go.uber.org/dig`，Fx/Dig metadata 只保留在 composition 边界。
- API 影响：Go constructor API 不兼容变更；HTTP API、OpenAPI 契约和外部业务行为不变。
- 数据与部署影响：不修改 Ent schema、Atlas migration、RBAC baseline、Redis key、Casbin policy、watcher 生命周期或部署资产。
- 验证影响：需要通过 role feature 测试、Fx/Dig import 搜索、OpenSpec 校验、架构 lint，以及暂存预期变更后的 `make lint` 和 `make verify`。
