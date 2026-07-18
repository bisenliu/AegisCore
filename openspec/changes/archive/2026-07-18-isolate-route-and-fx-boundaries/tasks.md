## 1. Route Registrar 边界

- [x] 1.1 在 `user-service/internal/router` 或等价 user-service composition 边界新增 public、authenticated、authorized route registrar contract，并确保 contract 不依赖 feature infrastructure implementation。
- [x] 1.2 为 auth、permission、role 和 user feature 增加对应 registrar provider，保持现有 path、method、controller 调用和 middleware 层级不变。
- [x] 1.3 重构 `user-service/internal/providers/routes.go` 和 `user-service/internal/router/router.go`，移除 `RegisterRouteParams`/`RouteParams` 中直接持有 feature controller 的 fan-in，改为消费分层 registrar group。
- [x] 1.4 增加或更新 route graph、auth middleware 和 metrics conflict 测试，验证 public auth、authenticated auth、authorized permission/role/user、health、OpenAPI 和 metrics 路由行为保持不变。

## 2. RBAC Public Contract 与 Health 边界

- [x] 2.1 在 permission application/authorization 边界定义 policy health 和 policy watcher status public interface，只暴露 `LastError()`、`Running()` 等只读状态，不引入 Redis、Casbin、Ent、Gin、Fx 或 Dig 类型。
- [x] 2.2 更新 permission Fx composition，使 Casbin engine 和 watcher 通过 public status contract 暴露给 service-level health，同时保留 feature 内部 lifecycle 对 concrete 的使用。
- [x] 2.3 重构 `user-service/internal/providers/health.go`，删除对 `features/permission/infrastructure/casbin` 和 `features/permission/infrastructure/redis` concrete 包的 import，改为依赖 public status contract。
- [x] 2.4 增加或更新 health provider/Fx graph 测试，验证 Casbin policy 和 watcher readiness 语义保持不变，且父 module 不能直接解析 permission infrastructure concrete。

## 3. Fx Provider 可见性收缩

- [x] 3.1 拆分 `user-service/internal/features/permission/fx.go` 的 internal/public provider，删除不再需要跨 module 的 `fx.As(fx.Self())`，并对 store、engine、watcher、tracker、cache holder 和 metrics implementation 等内部 provider 应用 `fx.Private` 或等价隔离。
- [x] 3.2 审视并调整 auth、role、user feature `fx.go` 的 provider 分组，仅在 public provider 暴露 controller、route registrar、authorizer 或 application port，避免新增 concrete self 暴露。
- [x] 3.3 更新 feature module wiring 测试，验证 public contract 可解析、内部 concrete 不跨 module 可解析，并且正式 graph 缺失必需安全依赖时构造失败。
- [x] 3.4 更新 `user-service-architecture-lint` 规则或测试数据，阻止父 module 重新导入 permission infrastructure concrete、feature application/domain 引入框架依赖或 route registrar 依赖 Fx group 顺序。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./user-service/internal/providers ./user-service/internal/router ./user-service/internal/bootstrap ./user-service/internal/features/auth ./user-service/internal/features/permission ./user-service/internal/features/role ./user-service/internal/features/user`，并修复失败。
- [x] 4.2 运行 `make user-service-architecture-lint`，并修复架构边界失败。
- [x] 4.3 如实现触碰 HTTP 注解或 OpenAPI 生成物，运行 `make user-service-openapi-generate` 并检查 `git diff --exit-code user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml`；未触碰时在实现记录中说明未运行原因。
- [x] 4.4 暂存本次预期代码、OpenSpec artifacts 和相关文档变更后运行 `make lint`，并修复失败。
- [x] 4.5 暂存本次预期代码、OpenSpec artifacts 和相关文档变更后运行 `make verify`，并修复失败，确保最终 drift 检查不会被未暂存的预期变更阻塞。
