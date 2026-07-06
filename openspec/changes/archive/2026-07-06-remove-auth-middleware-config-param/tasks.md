## 1. Common Middleware API

- [x] 1.1 修改 `common/http/middleware.AuthWithTokenVersionValidator` 签名，删除 `config.AuthConfig` 参数并移除不再需要的 `common/runtime/config` import。
- [x] 1.2 更新 `common/http/middleware/auth_test.go` 中所有 middleware 构造调用，确保 JWT 配置仍只传入 `auth.NewJWTService`。
- [x] 1.3 运行 `go test ./common/http/middleware`，验证认证响应、日志和 token version 校验行为不变。

## 2. User-Service 路由装配

- [x] 2.1 修改 `user-service/internal/router/router.go`，删除 `RouteParams.AuthConfig` 字段并更新 `AuthWithTokenVersionValidator` 调用参数。
- [x] 2.2 修改 `user-service/internal/providers/routes.go`，停止向 `router.RouteParams` 填充 `AuthConfig`。
- [x] 2.3 更新 `user-service/internal/router/router_registration_test.go` 和相关 provider 路由测试 fixture，删除不再存在的 `RouteParams.AuthConfig` 字段赋值。
- [x] 2.4 运行 `go test ./user-service/internal/router ./user-service/internal/providers`，验证路由注册和 provider 装配不回归。

## 3. 全仓检查与验证

- [x] 3.1 使用 `rg 'AuthWithTokenVersionValidator\('` 检查所有调用点均已同步为新签名。
- [x] 3.2 使用 `rg 'params\.AuthConfig|AuthConfig:' user-service/internal/router user-service/internal/providers common/http/middleware` 检查无遗留无效透传。
- [x] 3.3 运行 `go test ./common/http/middleware ./user-service/internal/router ./user-service/internal/providers`，完成最小回归验证。
- [x] 3.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [x] 3.5 运行 `make lint`，确认 lint 通过后再标记完成。
- [x] 3.6 运行 `make verify`，确认完整验证通过且无未暂存预期 diff 后再标记完成。
