## Why

`UserController.GetByID` 的方法名没有表达当前路由使用的是外部 UUID `user_id`，容易与内部自增 `id int64` 概念混淆。随着用户查询、后台管理或内部数据访问能力扩展，保留该命名会增加维护者误用内部 ID 与外部用户 ID 的风险。

## What Changes

- 将用户资料查询 controller handler 从 `GetByID` 重命名为表达外部用户 ID 语义的名称，优先使用 `GetByUserID`。
- 同步更新用户路由注册和相关 Go 引用，保持 controller/service/repository 分层职责不变。
- 同步更新相关 OpenSpec 主规格中对内部 handler 名称的引用，避免规格继续固定旧名称。
- 不改变 `GET /api/v1/users/:user_id` 路由、Swagger path 参数、响应信封、错误码、数据库 schema 或 repository 查询行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `project-naming-consistency`: 明确用户查询 controller 内部 handler 命名必须使用外部 `user_id` 语义，并在重命名时同步所有引用且保持外部契约不变。
- `user-profile-query`: 将查询用户规格中的内部 controller handler 引用从旧的 `GetByID` 语义调整为外部用户 ID 命名，同时保持查询 API 行为兼容。
- `http-service-runtime`: 将用户路由注册规格中的 handler 绑定从 `UserController.GetByID` 调整为 `UserController.GetByUserID`，保持路由 surface 和中间件行为不变。

## Impact

- 影响代码：`user-services/internal/controller/user_controller.go`、`user-services/internal/router/users.go`。
- 影响规格：`openspec/specs/project-naming-consistency/spec.md`、`openspec/specs/user-profile-query/spec.md`、`openspec/specs/http-service-runtime/spec.md`。
- API 兼容性：不改变 HTTP 路径、请求参数名、响应 JSON 字段、响应 envelope、业务错误码或认证要求。
- 数据兼容性：不改变 Ent schema、Atlas migration、数据库字段或 repository 查询条件。
