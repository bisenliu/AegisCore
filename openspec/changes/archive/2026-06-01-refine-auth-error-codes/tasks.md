## 1. Response Contract

- [x] 1.1 在 `common/response/error.go` 中新增 `CodeTokenInvalid = 20001` 与 `CodeTokenExpired = 20002`，并保留 `CodeUnauthenticated = 20000` 表示缺失认证信息或通用未认证。
- [x] 1.2 为新增错误码添加 `TokenInvalidError`、`TokenExpiredError` 构造函数，确保 HTTP status 均为 `401 Unauthorized`。
- [x] 1.3 在 `common/response/response.go` 中添加对应 Gin helper，保持失败信封结构与现有 `Fail` 行为一致。

## 2. Auth Middleware Mapping

- [x] 2.1 更新 `common/middleware/auth.go`：缺失 `Authorization` header 继续返回 `CodeUnauthenticated`，header 格式错误、空 bearer token 和 JWT 解析/签名/claims 错误返回 `CodeTokenInvalid`。
- [x] 2.2 在 token 解析失败时使用 `errors.Is(err, jwt.ErrTokenExpired)` 或共享 JWT 分类结果，将过期 token 映射为 `CodeTokenExpired`。
- [x] 2.3 保持所有认证失败响应 message 为 `unauthenticatedMessage`，并保留当前 error 级别日志和底层错误日志上下文。

## 3. Tests

- [x] 3.1 更新 `common/response/response_test.go`，覆盖新增错误构造函数和 helper 的 code、HTTP status 与 message。
- [x] 3.2 更新 `common/middleware/auth_test.go`，分别验证缺失 header 返回 `20000`、格式/空/非法 token 返回 `20001`、过期 token 返回 `20002`、有效 token 继续放行。
- [x] 3.3 更新 `user-services/internal/bootstrap/http_test.go` 中认证失败断言，避免把所有 401 固定为 `CodeUnauthenticated`。

## 4. Documentation And Verification

- [x] 4.1 检查 `openspec/specs/api-response-contract/spec.md` 的归档目标是否需要包含新增 token 认证失败要求，并确认 `common/middleware/auth.go` 的统一文案和日志级别无需单独主规格更新。
- [x] 4.2 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.3 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，确认共享响应契约和用户服务路由测试通过。
