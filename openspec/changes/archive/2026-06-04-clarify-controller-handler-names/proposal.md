## Why

`UserController.List`、`UserController.Create` 以及 auth controller 中的 `Login`、`Refresh`、`Logout`、`LogoutAll` 依赖接收者和路由上下文才能表达完整语义。随着 controller、路由、测试、Swagger 注释和日志继续扩展，短 handler 名称会降低定位和检索清晰度。

## What Changes

- 将用户资料 controller handler 重命名为更明确的业务动作名称，例如 `ListUsers`、`CreateUser`，并同步路由注册和测试引用。
- 将 auth/session controller 中同类短 handler 名称重命名为更明确的认证或会话动作名称，例如 `LoginUser`、`RefreshToken`、`LogoutCurrentSession`、`LogoutAllSessions`。
- 保留已足够明确的 `ChangePassword` 和 `GetByID`，除非实现阶段发现与项目命名规则存在更一致的最小调整。
- 同步 Swagger godoc 注释锚点和测试名称中直接引用旧 handler 名称的位置。
- 不改变 HTTP 路径、方法、认证要求、请求/响应结构、错误码、配置、数据库 schema 或 service/repository 行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-query`: 明确用户列表 handler 的实现命名约定，不改变 `GET /api/v1/users` 外部行为。
- `user-profile-create`: 明确用户创建 handler 的实现命名约定，不改变 `POST /api/v1/users` 外部行为。
- `user-session-control`: 明确登录、刷新、退出当前设备和退出全部设备 handler 的实现命名约定，不改变 `/api/v1/auth/*` 外部行为。
- `http-service-runtime`: 明确路由注册中的 controller handler 绑定名称，不改变 HTTP route surface 或中间件顺序。

## Impact

- 主要影响 `user-services/internal/controller/user_controller.go`、`user-services/internal/controller/auth_controller.go`、`user-services/internal/router/users.go`、`user-services/internal/router/auth.go` 以及相关 controller/router 测试。
- Swagger/OpenAPI 对外路径和响应契约保持兼容；仅更新 godoc 注释函数锚点以匹配新的 handler 名称。
- 不引入新依赖，不修改 Ent schema，不需要 Atlas migration。
