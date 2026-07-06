## Why

RBAC 写路径当前在更新权限或角色后为了返回完整实体执行额外 refetch，并在角色权限、用户角色批量绑定路径中逐条插入，导致在线管理接口和 seed/同步场景产生不必要的数据库往返。当前 Ent 版本已具备返回实体的单实体更新和批量创建基础能力，但现有命令与 HTTP 返回契约要求完整响应体，阻碍了写路径收敛为更少的数据库交互。

## What Changes

- **BREAKING**：权限和角色写侧更新、启用、停用 HTTP 接口不再返回完整实体响应体，成功后改为无响应体的成功状态，由调用方按需使用查询接口读取最新实体。
- 权限与角色 store 的普通更新和启停路径不再在 `Update().Save(ctx)` 成功后执行 `GetBy...` refetch。
- 角色权限和用户角色的替换、seed ensure、seed sync 批量新增路径改为基于 Ent `CreateBulk` 写入，减少单条 INSERT 循环。
- Ent 生成特性启用 `sql/upsert`，使 bulk create 能在需要幂等的 seed/ensure 路径中保留唯一冲突忽略语义。
- 保留系统角色、系统权限身份保护、权限可用性校验、事务回滚、policy reload 和用户角色缓存失效语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：调整 RBAC 权限、角色写接口返回契约，并要求批量绑定写路径保持事务性、幂等性和授权语义不变。
- `delivery-operations`：扩展 user-service Ent 生成约定，要求生成物支持 RBAC bulk insert 的唯一冲突忽略能力。

## Impact

- 影响 Go 代码：`user-service/internal/features/permission/application`、`user-service/internal/features/permission/transport/http`、`user-service/internal/features/permission/infrastructure/postgres`、`user-service/internal/features/role/application`、`user-service/internal/features/role/transport/http`、`user-service/internal/features/role/infrastructure/postgres`。
- 影响 HTTP API：权限与角色更新、启用、停用接口成功响应从 `200 + entity body` 调整为无实体响应；OpenAPI 注解和生成物需要同步。
- 影响 Ent 生成：`user-service/ent/generate.go` 需要启用 `sql/upsert` 并重新生成 `user-service/ent/` 生成代码。
- 不影响数据库 schema：不新增表、字段、索引或 Atlas migration。
- 不影响部署清单和观测资产：无需修改 Docker、Compose、Kubernetes、Helm、Prometheus 或 Grafana 资产。
- 安全影响：RBAC 授权、系统角色/权限保护、policy sync、用户角色缓存失效和超级管理员语义必须保持不变。
