## 1. Auth Middleware

- [x] 1.1 修改 `common/middleware.Auth` 签名，新增显式 `*zap.Logger` 参数并保持 JWT service 与 auth config 参数语义不变。
- [x] 1.2 将认证中间件内部白名单放行、认证头缺失、格式错误、空 token 和 token 校验失败日志改为使用传入 logger 与请求 context 输出。
- [x] 1.3 确认认证失败日志不记录 token 原文，并保持现有 HTTP 401、错误码、响应 message、abort 行为不变。

## 2. Call Sites

- [x] 2.1 修改 `user-services/internal/bootstrap/bootstrap.go` 的 Gin 中间件链，注册认证中间件时传入 Fx 注入的 `params.Log`。
- [x] 2.2 修改 `common/middleware/auth_test.go` 的 `Auth` 调用，传入测试 logger 或 no-op logger。
- [x] 2.3 全量搜索仓库内 `Auth(` 调用，确认没有遗漏旧签名调用处。

## 3. Verification

- [x] 3.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 3.2 在 `common/` 运行 `go test ./...`，确认共享中间件测试通过。
- [x] 3.3 在 `user-services/` 运行 `go test ./...`，确认用户服务组装编译与测试通过。
