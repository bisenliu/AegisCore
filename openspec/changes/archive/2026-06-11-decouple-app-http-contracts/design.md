## Context

当前 `user` 与 `auth` feature 已按 `api/app/domain/transport/http/infra` 分层组织，但 `app` 层仍直接返回 HTTP response DTO 和响应契约类型：

- `user/app.UserService` 返回 `userapi.UserResponse` 和 `response.PaginatedData[userapi.UserResponse]`。
- `auth/app.AuthService` 返回 `authapi.TokenResponse`、`authapi.ChangePasswordResponse` 和 `authapi.LogoutResponse`。
- `auth/app` 的 token、credential、session 编排组件以及 `user/app/service.go` 会直接构造 `response.*Error` 或 `response.FromError`。

这些依赖让 use case 的输出被 HTTP 表达绑定。目标是在不改变现有 HTTP API 外部行为的前提下，让 `app` 层只暴露领域模型、应用结果和应用错误分类，由 `transport/http` 完成 HTTP DTO、分页信封和错误响应映射。

## Goals / Non-Goals

**Goals:**

- `user-services/internal/features/{user,auth}/app` 不再导入 feature `api` 包或 `common/contract/response`。
- 用户资料 use case 返回用户领域模型或应用结果对象，列表 use case 返回应用分页结果而不是 HTTP 分页响应契约。
- 认证 use case 返回 `auth/app` 内部定义的 token、改密和登出应用结果，而不是 HTTP response DTO。
- HTTP response DTO 映射、`response.NewPaginatedData` 包装和 `response.*Error` 构造集中在 `transport/http`。
- 保持现有 HTTP 路由、JSON 字段、分页结构、业务错误码、HTTP status code、Swagger DTO 和认证行为兼容。

**Non-Goals:**

- 不调整数据库 schema、Ent 生成代码、Atlas migration 或 Redis key。
- 不改变 JWT claims、token subject、token version、refresh session 轮转策略或认证中间件行为。
- 不新增 CLI、RPC、事件或批处理入口，只为后续复用清理边界。
- 不新增第三方依赖，不迁移 `common/contract/response` 包。

## Decisions

### 1. App service 返回应用结果类型

在每个 feature 的 `app` 包内定义 use case 输出类型，例如：

- `user/app.UserResult`：包含公开业务所需的 `userdomain.User`。
- `user/app.ListUsersResult`：包含 `[]userdomain.User`、`Page`、`PageSize` 和 `Total`。
- `auth/app.TokenResult`：包含 access token、refresh token、token type、expires in 和 `PasswordChangeRequired`。
- `auth/app.ChangePasswordResult`、`auth/app.LogoutResult`：表达改密和登出用例结果。

这样 ports 面向应用语义，不泄漏 HTTP DTO。列表结果保留分页输入和总数，但不引用 `response.Pagination` 或 `response.PaginatedData`。

替代方案是让 service 直接返回领域模型和多个标量值，例如 `([]userdomain.User, int, error)`。该方案类型更少，但调用方更难读懂分页语义，也不利于未来扩展结果元数据，因此不采用。

### 2. HTTP DTO 和分页信封由 transport/http 映射

在 `user/transport/http` 和 `auth/transport/http` 增加或迁移 mapper：

- app result 转 `userapi.UserResponse`、`authapi.TokenResponse` 等 HTTP response DTO。
- `user/app.ListUsersResult` 转 `response.PaginatedData[userapi.UserResponse]`。
- controller 继续调用 `response.OK`、`response.Created` 或 `response.Fail` 输出统一信封。

这样 `api` 包仍然承载 HTTP request/response DTO 和 Swagger 文档模型，`transport/http` 成为 HTTP 表达边界。

替代方案是在 `api` 包中暴露 mapper。该方案会让 `api` 反向理解 app/domain，容易把 DTO 包变成业务转换层，因此不采用。

### 3. App 层返回领域错误或应用错误分类，transport 映射为 HTTP 应用错误

领域或持久化语义错误继续使用现有领域错误，例如 `userdomain.ErrUserNotFound`、`userdomain.ErrUserAlreadyExists`。认证流程中的稳定失败语义放在 `auth/domain`，例如无效凭据、缺失认证会话、token 无效等哨兵错误或轻量 typed error；`auth/app` 可以返回这些领域错误，但不拥有 HTTP status、响应码或响应消息。

`transport/http` 增加 feature-local 错误映射函数：

- 用户资料：`ErrUserAlreadyExists` 映射为 `response.ConflictError(messages.UserAlreadyExists)`；`ErrUserNotFound` 映射为 `response.NotFoundError(messages.UserNotFound)`；其他错误用 `response.FromError`。
- 认证会话：`authdomain.ErrInvalidCredentials` 映射为 `response.UnauthenticatedError(messages.InvalidCredentials)`；`authdomain.ErrMissingSession` 映射为 `response.UnauthenticatedError(messages.MissingSession)`；`authdomain.ErrTokenInvalid` 映射为 `response.TokenInvalidError(messages.MissingSession)`；其他错误用 `response.FromError`。

controller 调用 `response.Fail(c, toHTTPError(err))`，保持对外 status code、业务码和 message 兼容。

替代方案是保留 app 返回 `response.ApplicationError`，只移除 DTO。该方案仍会让 app 知道 HTTP status 和响应码，不能支持“第一步和第二步一起调整”的边界目标，因此不采用。

### 4. Token 签发组件也返回 app 结果

`auth/app/tokens.go` 当前的 `issuedTokenPair` 内嵌 `authapi.TokenResponse`，`IssuePasswordChangeToken` 也直接返回 HTTP DTO。需要改为：

- `issuedTokenPair.Response *TokenResult`
- `IssuePasswordChangeToken(...) (*TokenResult, error)`
- JWT 签发失败返回普通 wrapped error 或 app 内部错误分类，不构造 `response.FromError`

service 的登录、刷新和改密流程只组合 app result。最终 HTTP token DTO 在 auth controller mapper 中生成。

### 5. 测试按边界拆分断言

`app` 测试应断言应用结果字段、领域错误或应用错误分类，不再断言 HTTP DTO 类型或 `response.ApplicationError`。`transport/http` controller 测试继续断言 JSON 响应、status code、业务码和分页结构，以证明外部行为未变化。

## Risks / Trade-offs

- [Risk] 错误映射漏掉某个领域/app 错误，导致原本的 401/404/409 变成 500。→ 在 `transport/http` 增加错误映射单元测试，并保留 controller 行为测试覆盖登录失败、用户不存在、用户冲突和 token 无效路径。
- [Risk] service 签名调整会影响测试 stub 和 Fx 装配编译。→ 一次性更新 ports、service、controller stub 和 app tests，并运行 `go test ./...`。
- [Risk] 列表分页结果迁移可能改变空数组或 `total_pages` 行为。→ transport mapper 必须继续使用 `response.NewPaginatedData` 和 `response.NewPagination`。
- [Risk] 认证 token DTO 字段映射遗漏 `password_change_required` 或 refresh token 空值语义。→ controller 测试覆盖普通登录和强制改密登录响应。

## Migration Plan

1. 新增 app result 类型和 auth domain 错误分类。
2. 修改 user/auth app ports、service、token/credential/session 组件返回应用结果和领域/app 错误。
3. 将 HTTP DTO mapper、分页包装和错误映射移动到 `transport/http`。
4. 更新 app tests 与 controller tests，确保内部签名和外部 HTTP 行为同时被覆盖。
5. 运行 `gofmt`，并分别在 `common/` 与 `user-services/` 执行 `go test ./...`。

回滚策略：由于不涉及数据迁移或配置变更，可通过回滚代码恢复旧签名；外部 API 无需调用方迁移。

## Open Questions

无。实现时如发现认证错误分类需要区分 token expired 与 token invalid，应优先复用现有 `common/security/auth` 解析结果和既有 `api-response-contract` 业务码，不扩大本 change 范围。
