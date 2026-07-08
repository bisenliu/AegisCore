## 1. OpenSpec 与范围确认

- [X] 1.1 确认 `refactor-password-change-login-result` 的 proposal、design、`auth-session-management`、`shared-platform-primitives` 和 `runtime-observability` delta 已生成且可被 OpenSpec 识别。
- [X] 1.2 更新 change artifacts，确认最终方案为强制改密登录返回 HTTP `200 OK`、`success=false`、`CodePasswordChangeRequired=20006`、受限 token data，且登录响应 data 不再返回状态枚举。

## 2. 共享错误契约

- [X] 2.1 修改 `common/contract/errors`，新增或保留 `CodePasswordChangeRequired=20006` 和相关注释。
- [X] 2.2 更新 common 错误码测试，覆盖 `CodePasswordChangeRequired` 的稳定数值。
- [X] 2.3 确认不新增 `ReasonPasswordChangeRequired`、`PasswordChangeRequiredError` 或等价通用 error factory。

## 3. Auth application 登录结果模型

- [X] 3.1 修改 auth token result 与 issuer，使 `TokenResult` 只表达 token 载荷，移除 `PasswordChangeRequired` 或等价 token 业务状态字段。
- [X] 3.2 修改登录 use case 接口和实现，使用 `LoginResult.PasswordChangeRequired` 表达强制改密分支。
- [X] 3.3 删除 `LoginStatus`、`LoginStatusAuthenticated` 和 `LoginStatusPasswordChangeRequired` 字符串枚举。
- [X] 3.4 更新登录 use case、token issuer 和相关测试，覆盖普通登录、强制改密登录、token 签发失败和一次性 password change session 创建失败路径。

## 4. Auth HTTP 响应与 OpenAPI

- [X] 4.1 修改 auth HTTP DTO 和 mapper，删除登录响应 `status` 字段，并让登录、强制改密和 refresh 共用 `TokenResponse`。
- [X] 4.2 新增或保留 `toPasswordChangeRequiredEnvelope` 专用 mapper，返回 `success=false`、`CodePasswordChangeRequired`、`messages.PasswordChangeRequired` 和受限 token data。
- [X] 4.3 修改 auth HTTP controller，使普通登录返回 `success=true/CodeOK`，强制改密登录返回 `success=false/CodePasswordChangeRequired`，失败路径继续使用 `response.Fail(c, err)`。
- [X] 4.4 更新 auth HTTP controller 测试，断言强制改密登录响应不包含状态枚举、不包含 refresh token，并携带 `CodePasswordChangeRequired`。
- [X] 4.5 更新 E2E HTTP flow 断言，强制改密登录期望 `success=false` 和 `CodePasswordChangeRequired`。
- [X] 4.6 更新 auth controller OpenAPI 注解，并运行 `make user-service-openapi-generate` 生成 `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml`。

## 5. 验证与收尾

- [X] 5.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [X] 5.2 运行相关 grep，确认登录响应不再暴露 `LoginStatus`、`LoginResponse` 或响应 `status` 枚举，并确认 `CodePasswordChangeRequired` 只在预期位置出现。
- [X] 5.3 运行 `go test ./common/contract/errors`。
- [X] 5.4 运行 `go test ./user-service/internal/features/auth/application/command`。
- [X] 5.5 运行 `go test ./user-service/internal/features/auth/transport/http`。
- [X] 5.6 运行 `go test ./user-service/internal/providers`。
- [X] 5.7 运行 `go test ./user-service/tests/e2e -run '^$'` 编译 E2E flow。
- [X] 5.8 运行 `make user-service-architecture-lint` 验证 OpenSpec 与架构边界；当前因 `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml` 存在本次预期未提交生成物 diff 被 drift 检查拦截。
- [X] 5.9 运行 `openspec validate refactor-password-change-login-result --strict` 并确认通过。
