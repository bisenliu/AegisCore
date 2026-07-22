## Why

当前 RBAC baseline 的系统内置用户、角色和权限 ID 需要成为可审计、可长期维护且不会因算法或生成流程变化而漂移的稳定契约。通过手写固化保留 UUID，可以让 seed、bootstrap、测试和后续权限模块扩展共享同一套明确的编号规则，避免 UUIDv5、`go:generate` 或预分配未来模块带来的隐式依赖和维护成本。

## What Changes

- 在 `user-service/internal/shared/rbacbaseline/ids.go` 中统一手写维护系统保留 ID 常量，作为系统用户、系统角色和系统权限 ID 的唯一来源。
- 采用 `00000000-0000-0000-0000-TTMMSSSSSSSS` 格式，其中 `TT` 表示实体类型，`MM` 表示模块编号，`SSSSSSSS` 表示同类型同模块递增序号。
- 固化当前已存在的系统用户、超级管理员角色和 RBAC baseline 权限 ID，权限模块只为当前真实进入 `DefaultPermissions()` 的模块分配连续编号。
- 明确禁止 UUIDv5、`go:generate`、预分配不存在的权限模块编号，以及在 seed、bootstrap、HTTP runtime 中动态生成系统 ID。
- 更新测试，移除 UUIDv5 校验，改为校验保留格式、类型/模块编码、sequence 非零、全局唯一性和 baseline 引用登记。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 修改 RBAC baseline 系统保留 ID 的格式、来源、分配和校验规则。

## Impact

- 影响代码：`user-service/internal/shared/rbacbaseline/ids.go`、`user-service/internal/shared/rbacbaseline/permissions.go`、RBAC seed 和 bootstrap 中引用系统 ID 的相关代码，以及 `ids_test.go` 等测试。
- 影响安全与授权：RBAC baseline 权限、超级管理员角色和内置用户 ID 将成为长期稳定的授权数据契约，已发布或已删除 ID 不得修改或复用。
- 不影响 HTTP API 路径、请求/响应结构、OpenAPI 文档、数据库 schema 或部署资产。
- 不新增第三方依赖，不引入代码生成流程。
