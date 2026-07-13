## Why

当前公开 RBAC 管理 API 允许普通管理员在创建角色或权限时提交 `system=true`，该标记会被写入持久化数据并触发系统保护规则，导致非 seed 来源的数据被伪造成系统保护数据。

该问题破坏 RBAC seed 与系统保护边界，使系统保护语义可被公开 API 反向制造，需要以不保留兼容方案的方式收紧公开契约和写入路径。

## What Changes

- **BREAKING**：公开创建角色 API 移除 `system` 请求字段，普通 HTTP API 创建的角色一律为非系统角色。
- **BREAKING**：公开创建权限 API 移除 `system` 请求字段，普通 HTTP API 创建的权限一律为非系统权限。
- 普通 role/permission command 和 store create input 不再携带可由调用方控制的系统标记。
- 仅 RBAC seed port 可写入或更新系统角色、系统权限，并由 seed store 固定写入系统标记。
- 不新增 provenance 字段，不变更角色或权限数据库 schema；系统数据来源边界由 seed-only 写入路径和测试约束保证。
- 更新 OpenAPI 生成物、测试与主规格 delta，确保公开契约和安全边界一致。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `rbac-access-control`：收紧系统角色、系统权限的写入来源，规定公开 API 不得制造系统数据，只有 RBAC seed port 可写系统数据。

## Impact

- API：`POST /api/v1/roles` 和 `POST /api/v1/permissions` 请求体删除 `system` 字段，旧客户端继续传入该字段不再获得系统数据写入能力；若 JSON 绑定启用未知字段拒绝，应返回请求错误。
- OpenAPI：需要重新生成 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`。
- 数据库：不新增字段，不生成 Atlas migration；普通 create 和 seed upsert 只调整 `is_system` 写入边界。
- 业务代码：影响 role/permission HTTP request、input preparer、application command、application ports、PostgreSQL store 和 seed service 输入。
- 测试：需要反转当前允许 `system=true` 创建的 controller、command 和 store 测试，新增普通 API 固定非系统与 seed 固定系统覆盖。
- 安全：普通 RBAC 管理员不再能制造受系统保护的数据，系统保护边界恢复为 seed 来源数据专属。
