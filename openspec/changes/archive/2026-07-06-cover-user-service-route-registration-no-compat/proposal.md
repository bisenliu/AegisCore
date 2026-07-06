## Why

`RegisterUserServiceHTTPRoutes` 同时挂载健康检查、OpenAPI、metrics、pprof 和受认证/RBAC 保护的 `/api/v1` 业务路由，但当前缺少聚合路由注册测试，无法防止旧路径兼容别名、旧 metrics/pprof 路径或认证绕过路径回流。

本次变更通过补齐路由注册测试，把当前路由图和当前中间件链固化为可验证契约，降低 runtime-observability、auth-session-management、rbac-access-control 与 user-identity-management 交叉维护时的回归风险。

## What Changes

- 为 `user-service/internal/router` 新增或补充路由注册测试，覆盖 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes`。
- 测试验证当前健康检查、OpenAPI、metrics、pprof、auth、permission、role 和 user 路由组装结果。
- 测试验证 Permission/Role controller 为 nil 时只跳过对应可选路由，不影响 auth 和 user 核心路由。
- 测试验证 metrics 配置错误会从 `RegisterUserServiceHTTPRoutes` 返回，pprof 开关只影响当前配置的 pprof 路径。
- 新增测试遵循 `delivery-operations` 和 `docs/TESTING.md` 的语义化断言规范，不引入旧路径兼容断言或机械 Fail helper。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`: 固化 user-service 聚合路由注册测试对健康检查、OpenAPI、metrics 和 pprof 当前路由归属的验证要求。
- `auth-session-management`: 固化认证公开路由和受保护会话路由在 `/api/v1/auth` 下注册，并进入当前认证中间件链的测试要求。
- `rbac-access-control`: 固化权限、角色、用户角色路由在认证后经过 RBAC 授权中间件注册，以及 Role/Permission controller nil 条件注册的测试要求。
- `user-identity-management`: 固化用户资料路由在 `/api/v1/users` 下注册并受当前认证/RBAC 中间件链保护的测试要求。
- `delivery-operations`: 固化本次新增 router 测试必须遵循语义化 `require`/必要 `assert` 断言规范和覆盖率验收命令的要求。

## Impact

- 代码：仅涉及 `user-service/internal/router/router.go` 同包测试；必要时增加轻量 mock controller 或 Gin route inspection 辅助。
- API：不修改生产路由行为，不新增 `/api`、`/v1` 或其他旧路径兼容别名。
- 安全：不修改 JWT 校验、token version 校验、Casbin 授权或认证绕过行为；测试只验证当前中间件链注册结果。
- 观测：不修改 OpenAPI 文档生成、metrics handler 输出格式或 pprof handler 行为；测试只验证注册与配置错误返回。
- 交付：需要运行 `go test -cover ./user-service/internal/router`、`go tool cover -func` 和 `openspec validate cover-user-service-route-registration-no-compat`。
