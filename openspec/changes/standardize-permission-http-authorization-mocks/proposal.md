## Why

permission HTTP 授权中间件测试当前使用手写 `fakeAuthorizer`，而同包 command/query controller 测试已使用 gomock 生成 mock。两种 mock 风格并存会增加测试维护成本，也降低对授权调用参数和未调用路径的精确断言能力。

## What Changes

- 扩展 `user-service/internal/features/permission/transport/http/mock_generate.go`，为 `authorization.Authorizer` 生成 feature-local gomock mock。
- 改造 `user-service/internal/features/permission/transport/http/authorization_test.go`，移除手写 `fakeAuthorizer`，改用 gomock 断言 user id、Gin full path 和 HTTP method。
- 覆盖并保持白名单、`OPTIONS`、缺失用户、授权拒绝、授权错误和 invalid subject 等场景，并显式验证白名单绕过时不会调用 authorizer。
- 保留授权中间件的真实 Gin 测试路径，不改变生产授权逻辑、Casbin engine、permission application authorization service 或 common HTTP middleware。
- 不新增中央 mock 仓库，也不保留旧 fake 兼容入口。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 明确 permission HTTP 授权中间件必须使用认证用户 ID、Gin route template 和 HTTP method 构造授权请求，并明确白名单、`OPTIONS` 和缺失用户等边界行为；本 change 不改变既有运行时语义，仅用 gomock 测试锁定这些行为。

## Impact

- 影响代码：`user-service/internal/features/permission/transport/http/mock_generate.go`、`user-service/internal/features/permission/transport/http/authorization_test.go` 和生成的 feature-local mock 文件。
- 影响验证：需要执行 `make user-service-generate` 检查 mockgen 无 drift，执行 `cd user-service && go test ./internal/features/permission/transport/http` 验证测试通过，执行 `make user-service-architecture-lint` 验证架构边界。
- 不影响 HTTP API、OpenAPI、数据库 schema、migration、部署资产、生产 RBAC 授权语义或共享契约。
