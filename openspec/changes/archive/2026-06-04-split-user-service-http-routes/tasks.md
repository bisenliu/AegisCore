## 1. Router Structure

- [x] 1.1 将 `user-services/internal/router/router.go` 中的总入口重命名为 `RegisterUserServiceHTTPRoutes`，保留 `RouteParams` 作为用户服务 HTTP 路由注册参数。
- [x] 1.2 新增 `user-services/internal/router/system.go`，迁移 `HealthResponse`、`registerSystemRoutes` 和 `healthz`，保持 `/healthz` 响应不变。
- [x] 1.3 新增 `user-services/internal/router/auth.go`，拆出 `registerPublicAuthRoutes` 和 `registerProtectedAuthRoutes`，保持 auth 路径、HTTP 方法和 handler 绑定不变。
- [x] 1.4 新增 `user-services/internal/router/users.go`，拆出 `registerUserRoutes`，保持 users 路径、HTTP 方法和 handler 绑定不变。

## 2. Route Composition

- [x] 2.1 在 `router.go` 中实现 `registerV1Routes`，集中创建 `/api/v1`、公共认证分组、认证中间件分组和预留授权分组。
- [x] 2.2 确认认证中间件仍通过 `commonmw.AuthWithTokenVersionValidator(params.Log, params.JWT, params.AuthConfig, params.TokenVersionValidator)` 注册，且只作用于受保护路由。
- [x] 2.3 保持 `RegisterSwagger(engine, params.Environment)` 的调用位置和 Swagger 启用逻辑不变。
- [x] 2.4 更新 `user-services/internal/bootstrap/routes.go`，让 Fx 路由注册入口调用 `router.RegisterUserServiceHTTPRoutes`。

## 3. Verification

- [x] 3.1 运行 `gofmt` 格式化所有修改和新增的 Go 文件。
- [x] 3.2 使用代码搜索确认 `router.RegisterRoutes` 不再有残留调用。
- [x] 3.3 在 `user-services` 模块运行 `go test ./...`，验证路由、Swagger、bootstrap 和认证边界相关测试通过。
- [x] 3.4 如共享中间件相关行为受影响，在 `common` 模块运行 `go test ./...` 验证共享 HTTP middleware 测试通过。
