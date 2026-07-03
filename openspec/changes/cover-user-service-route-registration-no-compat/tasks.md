## 1. 路由注册测试

- [x] 1.1 在 `user-service/internal/router` 同包测试中新增聚合路由注册测试辅助，使用 Gin route inspection 收集 method 和 path，并用轻量请求结果验证认证/RBAC 边界。
- [x] 1.2 覆盖 `RegisterUserServiceHTTPRoutes` 注册健康检查、OpenAPI、metrics、pprof 和 `/api/v1` feature 核心路由的结果。
- [x] 1.3 覆盖 `registerV1Routes` 中 auth、permission、role 和 user 路由的当前路径与认证/RBAC 中间件边界。
- [x] 1.4 覆盖 PermissionController 和 RoleController 为 nil 时的条件注册结果，确保对应可选路由缺失且 auth/user 核心路由保留。
- [x] 1.5 覆盖 metrics 配置错误返回和 pprof 开关注册结果，不新增或接受旧路径兼容别名。

## 2. 断言规范

- [x] 2.1 检查新增测试使用 `require` 的语义化断言，必要时只在互相独立 route 条目上使用 `assert` 收集失败。
- [x] 2.2 确认新增测试没有机械使用 `Fail`、`Failf`、`FailNow`、`FailNowf`，且没有在存在更具体语义化断言时使用 `True` 或 `False` 包装布尔表达式。

## 3. 验证

- [x] 3.1 运行 `go test -coverprofile=/tmp/cover-user-service-router.out ./user-service/internal/router`。
- [x] 3.2 运行 `go tool cover -func=/tmp/cover-user-service-router.out`，确认 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 均有覆盖。
- [x] 3.3 运行 `openspec validate cover-user-service-route-registration-no-compat`。
- [x] 3.4 运行 `make user-service-architecture-lint` 检查 OpenSpec 文档语言与架构边界。
- [x] 3.5 将本次预期变更暂存后运行 `make lint`。
- [x] 3.6 将本次预期变更暂存后运行 `make verify`，并确认生成物 drift 或非预期 diff 不存在。
