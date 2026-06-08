## 1. Service 归一化边界

- [x] 1.1 移除 `UserController.ListUsers` 中对 `validators.NormalizeListUsers(&req)` 的直接调用，使 controller 只负责 query 参数绑定、调用 service 和输出响应。
- [x] 1.2 在 `UserService.ListUsers` 开始处添加 `validators.NormalizeListUsers(&req)`，确保其发生在日志记录、repository input 构造和分页响应构造之前。
- [x] 1.3 确认 `UserService.ListUsers` 使用归一化后的 `Page`、`PageSize`、`Offset`、`Limit`、`Nickname` 和 `Username` 访问 repository 并构造分页 metadata。

## 2. Tests

- [x] 2.1 更新 `user_controller_test.go` 的列表测试，使其不再在 controller 边界断言 service 侧分页默认值或 `Offset`/`Limit` 派生结果。
- [x] 2.2 更新或新增 `user_service_test.go` 列表测试，验证零值或非法分页会归一化为默认 repository input 和响应分页 metadata。
- [x] 2.3 更新或新增 `user_service_test.go` 列表测试，验证显式分页和带空白字符的 `nickname`/`username` 过滤条件会在 repository 访问前归一化。
- [x] 2.4 保留非法 `status` query 在 controller/shared validation 边界的校验覆盖。

## 3. 验证

- [x] 3.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 中运行 `go test ./...`，并修复边界移动导致的失败。
- [x] 3.3 确认本变更不需要修改 Ent schema、Ent 生成代码、Atlas migration、配置、路由、响应 envelope 或 API 文档。

## 4. Auth 归一化边界

- [x] 4.1 移除 `AuthController.LoginUser` 中对 `validators.NormalizeLogin(&req)` 的直接调用，使 controller 只负责 JSON binding、调用 service 和输出响应。
- [x] 4.2 在 `AuthService.Login` 开始处调用 `validators.NormalizeLogin(&req)`，确保用户名和密码 trim 以及空凭证错误映射在 service 入口统一执行。
- [x] 4.3 移除 `AuthController.ChangePassword` 中对 `validators.NormalizeChangePassword(&req)` 的直接调用，保留从 `Authorization` header 读取 token 并写入 request DTO 的 HTTP 边界职责。
- [x] 4.4 在 `AuthService.ChangePassword` 开始处调用 `validators.NormalizeChangePassword(&req)`，确保 Bearer stripping、新密码 trim、缺失 token 和空密码错误映射在 service 入口统一执行。
- [x] 4.5 移除 `AuthController.RefreshToken` 中对 `validators.NormalizeRefresh(&req)` 的直接调用，使 controller 不再处理 refresh token 归一化和缺失 token 业务错误映射。
- [x] 4.6 在 `AuthService.Refresh` 开始处调用 `validators.NormalizeRefresh(&req)`，确保裸 token 与 Bearer token 兼容、空 token 拒绝逻辑对 HTTP 和非 HTTP 调用一致。

## 5. Auth 测试

- [x] 5.1 更新 `auth_controller_test.go`，将登录、改密和刷新请求的归一化断言从 controller 边界移除或改为仅断言绑定后的原始输入被传给 service。
- [x] 5.2 更新或新增 `auth_service_test.go` 登录测试，验证带空白字符的 username/password 在 service 入口被归一化后再调用凭证校验仓储。
- [x] 5.3 更新或新增 `auth_service_test.go` 登录测试，验证 trim 后为空的凭证由 service 映射为 `InvalidCredentials` 认证失败。
- [x] 5.4 更新或新增 `auth_service_test.go` 改密测试，验证 `Authorization` header 中的 Bearer token 和带空白字符的新密码由 service 入口归一化后再校验 token 和更新凭证。
- [x] 5.5 更新或新增 `auth_service_test.go` 改密测试，验证缺失 token、仅 Bearer token 和 trim 后为空的新密码仍返回现有错误语义。
- [x] 5.6 更新或新增 `auth_service_test.go` 刷新测试，验证 Bearer refresh token 在 service 入口被归一化，且缺失 token 或仅 Bearer token 仍返回现有 `MissingSession` 错误语义。
