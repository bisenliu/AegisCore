## 1. App Boundary Types

- [x] 1.1 在 `user/app` 定义用户创建、查询和列表 use case 的 transport-neutral 应用结果类型。
- [x] 1.2 在 `auth/app` 定义 token、改密和登出的 transport-neutral 应用结果类型。
- [x] 1.3 在 `auth/domain` 定义认证 use case 需要暴露给 transport 的错误分类，并避免这些错误携带 HTTP status 或响应码。

## 2. User Feature Refactor

- [x] 2.1 修改 `user/app.UserService` 接口，使 `CreateUser`、`GetUserByID` 和 `ListUsers` 返回应用结果而不是 `userapi.*Response` 或 `response.PaginatedData`。
- [x] 2.2 修改 `user/app/service.go`，让 service 返回领域模型或应用结果，并将用户已存在、用户不存在和内部错误保持为领域错误或普通 Go error。
- [x] 2.3 将用户 HTTP DTO 映射和分页响应包装迁移到 `user/transport/http`，继续使用 `response.NewPagination` 和 `response.NewPaginatedData` 保持响应结构。
- [x] 2.4 在 `user/transport/http` 增加错误映射，将 `userdomain.ErrUserAlreadyExists`、`userdomain.ErrUserNotFound` 和普通错误映射为既有 HTTP 响应。
- [x] 2.5 更新用户 controller 和 controller tests，验证创建、查询、列表、冲突、not found、分页空数组和 internal error 响应保持兼容。

## 3. Auth Feature Refactor

- [x] 3.1 修改 `auth/app.AuthService` 接口，使登录、刷新、改密、登出和退出全部设备返回 `auth/app` 应用结果。
- [x] 3.2 修改 `auth/app/tokens.go`，让 token 签发组件返回 `TokenResult`，并移除对 `authapi` 和 `common/contract/response` 的依赖。
- [x] 3.3 修改 `auth/app/service.go`、`credentials.go` 和会话相关组件，返回应用结果和领域/app 错误分类，不构造 `response.*Error` 或 `response.FromError`。
- [x] 3.4 将认证 HTTP DTO 映射迁移到 `auth/transport/http`，保持 token、改密和登出响应 JSON 字段兼容。
- [x] 3.5 在 `auth/transport/http` 增加错误映射，将无效凭据、缺失会话、token 无效、not found 和普通错误映射为既有 HTTP 响应。
- [x] 3.6 更新认证 controller 和 controller tests，验证登录成功、强制改密登录、刷新、改密、登出、无效凭据、token 无效和 internal error 响应保持兼容。

## 4. Tests And Boundary Verification

- [x] 4.1 更新 user/auth app 层单元测试，使其断言应用结果和领域/app 错误分类，而不是 HTTP DTO 或 `response.ApplicationError`。
- [x] 4.2 运行 `gofmt` 格式化修改的 Go 文件。
- [x] 4.3 在 `common/` 运行 `go test ./...`。
- [x] 4.4 在 `user-services/` 运行 `go test ./...`。
- [x] 4.5 使用 `rg` 检查 `user-services/internal/features/{user,auth}/app` 不再导入 feature `api` 或 `common/contract/response`。
