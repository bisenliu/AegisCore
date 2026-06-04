## Context

`user-services/internal/router/router.go` 当前以 `RegisterRoutes` 作为路由注册入口，并在一个函数中完成 `/healthz`、Swagger、`/api/v1/auth`、认证中间件和 `/api/v1/users` 的注册。该入口由 `user-services/internal/bootstrap/routes.go` 在 Fx 组合根中调用，属于 `http-service-runtime` 能力的一部分。

这次变更只调整用户服务路由注册代码的组织方式和命名，不改变 Gin engine 创建、共享中间件顺序、controller/service/repository 分层、认证中间件实现、Swagger 启用规则或任何 HTTP API contract。

## Goals / Non-Goals

**Goals:**

- 使用更具体的服务级入口命名表达用户服务 HTTP 路由总注册职责。
- 将 `router` 包内路由注册逻辑按 API surface 拆为聚焦源文件和私有注册函数。
- 保持现有 `/healthz`、Swagger、认证接口和用户接口路径、方法、handler 绑定、认证边界完全等价。
- 保留后续 Casbin 授权中间件在认证之后、业务 handler 之前挂载的稳定位置。

**Non-Goals:**

- 不新增 admin、internal、v2 或其他新 API surface。
- 不实现 Casbin 授权中间件或修改认证中间件逻辑。
- 不修改 controller/service/repository 行为。
- 不修改配置、Redis、PostgreSQL、Ent schema、Atlas migration 或响应信封。
- 不新增第三方依赖。

## Decisions

### 使用扁平化 `router` 包源文件拆分

采用 `router.go`、`system.go`、`auth.go`、`users.go`、`swagger.go` 的同包文件组织方式。`router.go` 保留 `RouteParams` 和总入口，`system.go` 承载 `/healthz`，`auth.go` 承载公共与受保护认证路由，`users.go` 承载用户资源路由，`swagger.go` 保持现有 Swagger 注册逻辑。

选择该方案是因为当前只有一个服务和一个 API version，拆成子包会引入额外导入、可见性和共享参数传递成本。备选方案是创建 `router/v1`、`router/system` 等子包，但当前规模下收益不足，且会让 Gin 分组和 controller 依赖跨包传播。

### 将总入口命名为 `RegisterUserServiceHTTPRoutes`

`router.RegisterRoutes` 将改为 `router.RegisterUserServiceHTTPRoutes`，表达该函数注册的是用户服务完整 HTTP surface，而非任意路由集合。`bootstrap` 层作为 Fx glue code 可继续保留自身 `RegisterRoutes` 名称，仅更新对 router 包入口的调用。

备选方案是 `RegisterHTTPRoutes` 或 `MountUserServiceRoutes`。`RegisterUserServiceHTTPRoutes` 更明确地区分服务范围和 HTTP 路由范围，同时不暗示新增框架或插件机制。

### 使用私有函数表达分组边界

`RegisterUserServiceHTTPRoutes` 只负责串联：系统路由、Swagger、v1 路由。`registerV1Routes` 负责创建 `/api/v1`、公共认证分组、认证中间件分组和预留授权分组。`registerPublicAuthRoutes`、`registerProtectedAuthRoutes` 和 `registerUserRoutes` 负责具体 handler 绑定。

该设计让认证边界集中在 `registerV1Routes`，避免把中间件挂载细节分散到各资源文件。后续新增业务资源时，可在认证/授权边界之后挂载新的资源注册函数。

### 保持外部行为等价

所有路径、HTTP 方法、handler、Swagger 启用逻辑、认证中间件参数和中间件顺序必须保持不变。基础中间件仍由 `bootstrap.NewGinEngine` 注册，路由局部认证仍通过 `commonmw.AuthWithTokenVersionValidator` 注册。

## Risks / Trade-offs

- 路由拆分后可能遗漏 handler 绑定 -> 通过现有测试和 `go test ./...` 验证，并优先保持函数内注册语句一一搬移。
- 总入口重命名可能遗漏调用点 -> 使用代码搜索确认 `RegisterRoutes` 引用，并更新 `bootstrap/routes.go`。
- 同包多文件会增加文件数量 -> 以更清晰职责边界换取少量文件增加，不引入子包以控制复杂度。
- 规格要求偏向实现结构 -> 仅将长期稳定的路由组织和服务契约等价性写入 `http-service-runtime`，不描述一次性临时细节。

## Migration Plan

1. 在 `user-services/internal/router` 中按职责拆分源文件。
2. 将 router 包总入口改为 `RegisterUserServiceHTTPRoutes`。
3. 更新 `user-services/internal/bootstrap/routes.go` 调用点。
4. 运行 `gofmt` 格式化改动文件。
5. 在 `user-services` 模块运行 `go test ./...`，必要时在 `common` 模块运行 `go test ./...` 确认共享中间件相关行为未受影响。

回滚策略：若测试发现路由行为变化，可将拆分文件中的注册语句恢复为原 `RegisterRoutes` 单函数结构，不涉及数据或迁移回滚。

## Open Questions

无。当前变更范围限定为用户服务路由注册命名与结构拆分，不需要产品或外部 API 决策。
