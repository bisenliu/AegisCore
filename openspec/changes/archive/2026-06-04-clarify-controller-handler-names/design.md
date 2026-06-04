## Context

用户服务当前采用 Gin 路由、controller/service/repository 分层。用户 controller 中 `List`、`Create` 需要结合 `UserController` 接收者才能表达用户列表和用户创建语义；auth controller 中 `Login`、`Refresh`、`Logout`、`LogoutAll` 需要结合 `/auth` 路由上下文才能表达认证和会话语义。路由注册处目前可读，但测试、Swagger godoc 注释、日志和后续 handler 扩展会持续增加对方法名的直接引用。

本变更只调整 controller handler 的 Go 标识符和引用点，不改变 HTTP 路由、请求绑定、响应信封、认证中间件、service 接口或 repository 行为。

## Goals / Non-Goals

**Goals:**

- 让 controller handler 名称在脱离接收者或路由上下文时仍能表达具体业务动作。
- 保持 user controller 与 service 层 `ListUsers`、`CreateUser` 命名一致。
- 让 auth/session handler 名称区分登录、刷新 token、退出当前会话和退出全部会话。
- 同步路由注册、Swagger godoc 注释锚点和测试引用，避免留下旧名称。

**Non-Goals:**

- 不重命名 service 接口或业务方法。
- 不改变任何 HTTP method、path、认证要求、响应格式、错误码或 Swagger 对外 API 描述。
- 不调整请求 DTO、响应 DTO、Ent schema、Atlas migration、Redis key 或数据库访问逻辑。
- 不新增 common 共享能力或中间件。

## Decisions

- 将 `UserController.List` 重命名为 `UserController.ListUsers`，将 `UserController.Create` 重命名为 `UserController.CreateUser`。这样与 service 层现有 `ListUsers`、`CreateUser` 保持一致，检索时也能直接命中用户业务动作。备选方案是保留短 CRUD 名称；该方案改动更小但不能解决脱离接收者后的语义不足。
- 保留 `UserController.GetByID`。该名称已经表达按 ID 查询，且与现有路由参数 `user_id` 一致；强行改为 `GetUserByID` 会增加额外改动但收益较小。
- 将 `AuthController.Login` 重命名为 `LoginUser`，将 `Refresh` 重命名为 `RefreshToken`，将 `Logout` 重命名为 `LogoutCurrentSession`，将 `LogoutAll` 重命名为 `LogoutAllSessions`。这些名称分别表达用户凭据登录、刷新 token、退出当前会话和退出全部会话，比短动词更适合在路由、测试和 godoc 锚点中独立出现。备选方案是完全跟随 service 层 `Login`、`Refresh`、`Logout`、`LogoutAll`；该方案保持一致但继续依赖 `/auth` 路由上下文。
- 保留 `AuthController.ChangePassword`。该名称已经明确表达业务动作，且不是短 CRUD 或短会话动作。
- 路由注册只更新 handler 引用，`group.GET`、`group.POST` 的 path 字符串保持不变。controller 内部继续只负责 HTTP 绑定、请求清洗和响应写入，service/repository 分层不变。
- `http-service-runtime` 主规格中也存在 `UserController.Create` handler 绑定描述，因此需要在本变更中补充运行时路由注册命名约束，避免实现完成后规格仍引用旧 handler 名称。

## Risks / Trade-offs

- 旧方法名仍被测试或文档引用 -> 通过全文搜索 `UserController.List`、`UserController.Create`、`AuthController.Login`、`AuthController.Refresh`、`AuthController.Logout`、`AuthController.LogoutAll` 和对应 route handler 引用，确保无遗漏。
- Swagger 生成文档可能依赖 godoc 注释锚点 -> 同步修改注释标题为新 handler 名称，保持 `@Router`、参数和响应说明不变。
- 纯重命名不改变运行时行为，测试收益有限 -> 运行 `go test ./...` 覆盖路由注册、controller 请求绑定和认证会话相关测试，验证无行为回归。

## Migration Plan

该变更无运行时迁移步骤。部署时随普通 Go 代码发布即可，回滚方式是回退 handler 标识符重命名及引用点。由于 HTTP 契约、配置、数据结构和存储均不变，客户端无需调整。

## Open Questions

无。
