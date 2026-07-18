## Why

用户服务当前的 HTTP route 总装和 Fx module 边界仍能正常工作，但新增 feature 时需要持续扩张同一个 `RegisterRouteParams`/`RouteParams` 对象，并且 permission feature 的部分内部 concrete implementation 已经对父 module 可见。现在收缩这些边界，可以在不改变 HTTP API、RBAC 行为和运行时观测语义的前提下，降低后续 feature 接入与内部实现重构的耦合面。

## What Changes

- **BREAKING**：不保留父 module 直接注入 feature 内部 concrete implementation 的兼容路径，跨 module 消费必须改为稳定 public contract。
- 将 HTTP route 总装从“集中持有所有 feature controller”调整为按 middleware 层级分发的 route registrar 模式，避免未来每新增 feature 都修改同一个参数对象。
- 明确 route registrar 不得依赖 Fx value group 的 slice 顺序；如存在顺序要求，必须通过显式层级、稳定 key 或 composition root 固定编排表达。
- 将 permission feature 的 Casbin policy health 和 watcher health 对外暴露为稳定 interface，服务级 health checks 不再导入 permission infrastructure concrete。
- 拆分 feature module 的 internal/public providers，仅对 controller、authorizer、health/status 等跨 module contract 公开，内部 store、engine、watcher、tracker、metrics implementation 等按需使用 `fx.Private` 收缩可见性。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`：健康检查和 HTTP route 总装的跨 module 依赖边界调整为稳定 public contract 和分层 registrar。
- `rbac-access-control`：RBAC policy engine、policy sync watcher 与授权能力的 Fx 暴露边界收缩为明确的 public contract，不再向父 module 暴露内部 concrete。

## Impact

- 影响 Go 代码：`user-service/internal/providers/routes.go`、`user-service/internal/providers/fx.go`、`user-service/internal/router/router.go`、`user-service/internal/providers/health.go`、各 feature `fx.go`、各 feature HTTP routes/provider 相关文件、permission application/authorization contract 文件和相关测试。
- 不改变 HTTP API 路径、方法、请求/响应结构、OpenAPI 生成物语义、数据库 schema、Atlas migration、部署清单或外部共享契约。
- 安全边界保持不变：public/authenticated/authorized 路由层级必须保持当前认证优先、RBAC 授权后置的行为；RBAC fail-closed 和 watcher readiness 语义必须保持。
- 依赖影响集中在 Fx wiring：删除父 module 对 permission infrastructure concrete 的直接依赖，改为 application/authorization 层 public interface。
- 需要更新或新增 wiring/route/health 相关单元测试，并运行 `make user-service-architecture-lint`、相关 Go 测试、`make lint` 和 `make verify`。
