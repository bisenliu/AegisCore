## 1. 配置契约清理

- [x] 1.1 从 `common/config.AuthConfig` 删除 `Whitelist` 字段，并确认共享配置对象不再暴露认证白名单集合。
- [x] 1.2 从 `user-services/configs/config.yaml` 删除 `auth.whitelist` 及所有子项，保留 JWT、token version cache TTL 和 refresh token rotation 配置。
- [x] 1.3 更新 `common/config/loader_test.go` 中认证配置加载、缺失配置和环境变量覆盖测试，移除所有白名单断言和样例配置。

## 2. 认证中间件重构

- [x] 2.1 从 `common/middleware/auth.go` 删除白名单路径判断和 `isWhitelistedPath` helper，使认证中间件只处理实际挂载路由。
- [x] 2.2 更新认证中间件测试，删除白名单放行用例，保留并验证缺失 header、格式错误、空 token、非法 token、过期 token、token version 不匹配和认证上下文传播行为。
- [x] 2.3 确认认证失败日志仍使用调用方传入的 Zap logger、保留 `trace-id`，且不记录 token 原文。

## 3. 路由分组与中间件挂载

- [x] 3.1 调整 `user-services/internal/bootstrap.NewGinEngine`，全局仅挂载 trace-id、recovery、request logging 和 CORS，不再全局挂载认证中间件。
- [x] 3.2 扩展 `router.RouteParams` 和 bootstrap 路由注册参数，将认证中间件所需的 logger、JWT service、AuthConfig 和 SessionStore 传入路由层。
- [x] 3.3 在 `user-services/internal/router/router.go` 建立公开路由分组，注册 `GET /healthz`、Swagger、`POST /api/v1/auth/login`、`POST /api/v1/auth/refresh` 和 `POST /api/v1/auth/change-password`。
- [x] 3.4 在 `/api/v1` 下建立已认证路由分组并局部挂载认证中间件，注册 `POST /api/v1/auth/logout`、`POST /api/v1/auth/logout-all`、`GET /api/v1/users`、`POST /api/v1/users` 和 `GET /api/v1/users/:user_id`。
- [x] 3.5 在路由代码中以清晰命名或注释预留 Casbin 授权子分组挂载位置，确保未来授权中间件顺序为认证之后、业务 handler 之前。

## 4. 路由行为测试

- [x] 4.1 增加或更新路由/启动测试，验证 `/healthz`、Swagger、登录、刷新和改密入口未携带普通 Access Token 时不会被认证中间件拒绝。
- [x] 4.2 增加或更新路由/启动测试，验证 `/api/v1/users`、`/api/v1/users/:user_id`、`/api/v1/auth/logout` 和 `/api/v1/auth/logout-all` 未携带有效 Bearer token 时在进入 controller 前返回 HTTP 401。
- [x] 4.3 验证公开路由和受保护路由仍经过 trace-id、recovery、request logging 和 CORS 基础中间件。

## 5. 验证与收尾

- [x] 5.1 运行 `gofmt` 格式化变更的 Go 文件。
- [x] 5.2 在 `common/` 执行 `go test ./...`，确认共享配置和中间件测试通过。
- [x] 5.3 在 `user-services/` 执行 `go test ./...`，确认路由、bootstrap、controller 和服务测试通过。
- [x] 5.4 运行 OpenSpec 校验或状态检查，确认 proposal、design、specs 和 tasks 均完成且变更可进入 apply 阶段。
