## 1. 中间件日志等级调整

- [x] 1.1 修改 `common/http/middleware/auth.go`，将缺少 `Authorization` header 的认证失败日志从 Error 降为 Info。
- [x] 1.2 修改 Bearer 格式错误、空 token 和其他 Authorization header 解析失败路径，使其使用 Warn 并保持响应 code/status 不变。
- [x] 1.3 修改 JWT access token 校验失败路径，使无效 token、过期 token、subject 错误和必要 claim 缺失使用 Warn；显式保留 `auth.ErrMissingSecret` 为 Error。
- [x] 1.4 保持 token version mismatch 为 Warn，保持非 mismatch 的 token version validator 依赖异常为 Error。

## 2. 测试覆盖

- [x] 2.1 更新 `common/http/middleware/auth_test.go`，使用可观测 Zap logger 断言缺 header、格式错误、空 token、无效 token 和过期 token 不产生 Error 日志。
- [x] 2.2 添加或更新测试，断言格式错误、空 token、无效 token、过期 token 和 token version mismatch 产生 Warn 日志。
- [x] 2.3 添加或更新测试，断言 JWT secret 缺失和 token version validator 依赖异常仍产生 Error 日志。
- [x] 2.4 确认测试继续覆盖 HTTP 401/500 响应 code/message/status 与 handler abort 行为不变。

## 3. 验证

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `common/` 模块运行 `go test ./...`。
- [x] 3.3 如实现影响跨模块构建，在 workspace 根目录或 `user-services/` 运行相关测试确认用户服务集成未回归。
