## Context

当前 user-service 的 HTTP route 总装位于 `user-service/internal/providers/routes.go` 和 `user-service/internal/router/router.go`。`RegisterRouteParams` 与 `router.RouteParams` 直接持有 runtime config、logger、Gin engine、JWT verifier、health、metrics、token version validator、RBAC authorizer 以及多个 feature controller。该设计目前符合 composition root 职责，但每新增 feature 都需要扩张同一组参数和总装函数。

permission feature 当前在 `user-service/internal/features/permission/fx.go` 中对 `Engine`、Redis policy `Store`、`VersionTracker`、`Watcher` 使用 `fx.As(fx.Self())` 暴露 concrete，并且 `user-service/internal/providers/health.go` 直接依赖 `*permissioncasbin.Engine` 和 `permissionredis.WatcherStatus`。这使父 module 可以依赖 permission infrastructure 实现细节，增加内部重构影响面。

本变更影响 Go wiring 和架构边界，不改变 HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或外部共享契约。安全边界必须保持：public auth route 不经过普通 access token，protected auth route 只经过认证，permission/role/user 业务路由必须认证后再经过 RBAC 授权。

## Goals / Non-Goals

**Goals:**

- 将业务 route 注册抽象为按 middleware 层级分发的 registrar，减少新增 feature 时对 `RegisterRouteParams` 和 `router.RouteParams` 的 fan-in 修改。
- 保持运行时端点、public auth、authenticated auth、authorized business route 的现有访问边界。
- 删除父 module 对 permission infrastructure concrete 的直接依赖，改为 application/authorization 层 public contract。
- 拆分 feature internal/public provider，使用 `fx.Private` 收缩明确不应跨 module 使用的 implementation。
- 通过测试和 architecture lint 锁定 route 层级、health readiness 和 Fx graph 边界。

**Non-Goals:**

- 不新增、删除或改名任何 HTTP endpoint。
- 不改变认证、token version、RBAC authorize、policy reload、Redis policy sync、用户角色缓存或健康检查语义。
- 不迁移 Ent schema，不生成 Atlas migration。
- 不调整 OpenAPI 文档内容，除非实现中发现注解 drift；本变更预期不需要重新生成 OpenAPI。
- 不把 user-service feature contract 下沉到 `common`、`internal/shared` 或 `internal/integration`。

## Decisions

### Decision: route registrar 必须按访问层级建模

采用 public、authenticated、authorized 三类 registrar，而不是单一 `api_routes` group。composition root 仍负责创建 `/api/v1`、认证 middleware 和 RBAC middleware，然后把正确的 `*gin.RouterGroup` 传给对应 registrar。

备选方案：使用单一 `RouteRegistrar` value group，让 registrar 自行注册完整路径和 middleware。拒绝该方案，因为它会把认证/RBAC 层级分散到 feature，容易让 public/authenticated/authorized 边界漂移。

### Decision: 不依赖 Fx value group 顺序

registrar group 不得表达必须依赖 slice 顺序的行为。若 route 冲突或注册顺序存在语义要求，必须在 composition root 显式固定，或让 registrar 暴露稳定 key 并由 composition root 排序。当前实现应优先保持层级正确和路径不冲突，不通过 group 顺序隐式保证安全。

备选方案：直接按 Fx group 返回顺序循环。拒绝该方案，因为 Fx value group 无顺序保证，测试通过不代表生产构图顺序稳定。

### Decision: health 只消费 RBAC public status contract

在 permission application/authorization 边界定义 policy health 和 watcher status 这类稳定 interface，`providers.HealthCheckParams` 只依赖这些 interface。`providers` 不再导入 `features/permission/infrastructure/casbin` 或 `features/permission/infrastructure/redis`。

备选方案：保留 `*permissioncasbin.Engine` 给 health 使用，只给其他 consumer 加 private。拒绝该方案，因为这会保留父 module 对 feature infrastructure concrete 的兼容路径，违背本次“不保留兼容方案”的目标。

### Decision: feature provider 拆分 internal/public 并最小化 `fx.As(fx.Self())`

feature `fx.go` 中按 internal provider 和 public provider 拆分 `fx.Provide` 调用。store、engine、watcher、tracker、cache holder、metrics implementation 等内部实现默认放在 internal provider，并在不需要跨 module concrete 注入时使用 `fx.Private`。controller、authorizer、route registrar、health/status contract 等跨 module contract 由 public provider 暴露。

备选方案：把整个 feature `fx.Provide` 标为 `fx.Private` 后再补充 wrapper。拒绝该方案，因为 public controller、authorizer 和 route registrar 仍需被 user-service composition root 消费；整组 private 会破坏正式 graph。

### Decision: 代码归属保持在 user-service 内部

route registrar contract 放在 `user-service/internal/router` 或服务级 composition 能消费且不反向依赖 feature implementation 的位置。RBAC policy health 和 watcher status contract 放在 permission application/authorization 边界。禁止将 user-service route、RBAC subject schema、policy sync 或 health status contract 放入 `common`。

备选方案：把 registrar 或 RBAC status interface 放到 `common`。拒绝该方案，因为这些 contract 包含 user-service 的 feature composition 语义，不属于跨服务无业务语义 primitive。

## Risks / Trade-offs

- [Risk] registrar 抽象可能让 route 图不如显式注册直观 → Mitigation: route graph 测试必须验证当前 `/api/v1` 路径、public/authenticated/authorized middleware 行为和 route diff 扫描结果。
- [Risk] `fx.Private` 加得过宽导致 controller、authorizer 或 health status 无法解析 → Mitigation: 拆分 internal/public provider，并用 `fx.ValidateApp` 或 `fxtest` 覆盖正式 graph。
- [Risk] 移除 concrete self 暴露后 lifecycle 仍需要 concrete `Engine` 或 `Watcher` → Mitigation: lifecycle registration 保持在 permission module 内部，同一 private module 内可继续使用 concrete；跨 module 只暴露 public contract。
- [Risk] health status interface 放错层导致 application 依赖 infrastructure 细节 → Mitigation: interface 只表达 `LastError()`、`Running()` 等只读状态，不暴露 Redis、Casbin、Ent 或 Gin 类型。
- [Risk] route registrar value group 顺序误用于解决路径冲突 → Mitigation: 对存在顺序或冲突要求的路由继续在 composition root 显式编排，或者增加稳定 key 排序并测试锁定。

## Migration Plan

1. 增加 route registrar contract 和 feature registrar provider，使现有 auth、permission、role、user route 能通过分层 registrar 注册。
2. 重写 `RegisterRoutes`/`router.RegisterUserServiceHTTPRoutes` 的参数形态，移除直接持有 feature controller 的 fan-in，同时保持运行时端点注册不变。
3. 在 permission application/authorization 边界增加 RBAC health/status public contract，替换 `providers.HealthCheckParams` 对 infrastructure concrete 的依赖。
4. 拆分 permission/auth/role/user feature `fx.go` 的 internal/public providers，对明确内部实现应用 `fx.Private`，删除不再需要的 `fx.As(fx.Self())` 跨 module 暴露。
5. 更新 route、health、Fx graph 和 architecture lint 测试，验证行为保持不变并阻止 infrastructure concrete 重新跨 module 泄漏。

回滚方式：本变更是内部 wiring 重构，无数据迁移和外部 API 迁移。若上线前发现 graph 或 route 行为异常，回退本 change 的代码提交即可恢复旧 composition root；无需数据库、OpenAPI 或部署回滚。

## Verification

- 运行 route 和 wiring 相关 Go 测试：`go test ./user-service/internal/providers ./user-service/internal/router ./user-service/internal/bootstrap ./user-service/internal/features/auth ./user-service/internal/features/permission ./user-service/internal/features/role ./user-service/internal/features/user`。
- 运行架构边界检查：`make user-service-architecture-lint`。
- 暂存本次预期代码、规格和文档变更后运行 `make lint` 和 `make verify`。
- 若实现中触碰 HTTP 注解或 OpenAPI 生成物，额外运行 `make user-service-openapi-generate` 并检查 `git diff --exit-code`。

## Open Questions

无。
