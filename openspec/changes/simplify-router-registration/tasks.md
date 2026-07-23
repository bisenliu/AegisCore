## 1. 路由装配重构

- [x] 1.1 调整 `user-service/internal/router` 的 route composition 参数，显式接收 auth、permission、role 和 user controller 以及现有安全依赖。
- [x] 1.2 在 `/api/v1` 下集中挂载 auth public、auth authenticated、permission、role 和 user transport route 函数，保持三层 middleware 顺序不变。
- [x] 1.3 保留缺失 token version validator、RBAC authorizer 或必需 controller 时的注册失败行为，避免服务以缺失业务路由状态启动。

## 2. Fx 装配清理

- [x] 2.1 更新 `user-service/internal/providers/routes.go`，由固定 controller 输入构造 `router.RouteParams`，不再消费 `group:"*_routes"`。
- [x] 2.2 从 auth、permission、role 和 user feature `fx.go` 中移除 route registrar provider、`fx.As(new(router.*RouteRegistrar))` 和 route group tag。
- [x] 2.3 删除或停用仅用于固定路由转发的 feature-local `route_registrar.go`，并清理不再使用的 router registrar interface。

## 3. 测试和架构约束

- [x] 3.1 更新 `user-service/internal/router/router_registration_test.go`，校验集中注册后的 runtime route、auth route、authorized business route 和 RBAC baseline 一致性。
- [x] 3.2 更新 providers/Fx graph 相关测试，确保正式 App graph 能解析固定 controller 依赖且不再要求 route value group。
- [x] 3.3 更新 architecture lint 或相关规则，使其禁止或不再要求固定 feature 使用 route registrar，同时仍保护 application/domain 不引入 Fx/Gin 边界泄漏。

## 4. 验证和收尾

- [x] 4.1 运行 `make user-service-test`，确认 user-service 路由、provider graph 和 feature 测试通过。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认结构边界和 OpenSpec 相关架构约束通过。
- [x] 4.3 如实现过程中修改 OpenAPI 注解或公开路由契约，运行 `make user-service-openapi-generate` 并检查生成物 drift；若未触及，记录无需运行的原因。
- [x] 4.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区后运行 `make lint`。
- [x] 4.5 在暂存本次预期变更后运行 `make verify`，确认完整验证不因未暂存预期变更阻塞。
