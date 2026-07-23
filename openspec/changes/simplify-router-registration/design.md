## Context

当前 user-service 通过 `router.PublicRouteRegistrar`、`router.AuthenticatedRouteRegistrar` 和 `router.AuthorizedRouteRegistrar` 三类 contract 收集 feature 路由，并由 Fx value group 注入到 `providers.RegisterRoutes`。该方案把访问层级集中在 `user-service/internal/router/router.go`，但 auth、permission、role 和 user 都是当前服务的固定核心 feature，实际并不需要插件式路由发现。

本变更只调整 user-service 的 HTTP route composition 方式。HTTP path、method、controller、OpenAPI 注解、Casbin permission baseline、token version validator、RBAC authorizer 和 runtime endpoint 行为保持不变。

受影响路径主要包括 `user-service/internal/router/`、`user-service/internal/providers/`、`user-service/internal/features/*/fx.go` 和相关 route graph/provider graph 测试。`common/` 不应承载 user-service 路由图、feature controller 或 RBAC 业务语义；`deployments/`、Ent schema、Atlas migration 和观测资产不需要变更。

## Goals / Non-Goals

**Goals:**

- 由 user-service composition root 显式集中注册固定 feature 路由，降低 Fx group 和 feature-local registrar 样板。
- 保留 `/api/v1` public、authenticated 和 authorized 三层访问边界。
- 保持缺失 token version validator、RBAC authorizer 或必需 controller 时启动期 fail-closed。
- 保持 route graph 测试和 RBAC permission baseline 对实际路由的稳定校验能力。
- 删除或停止生产路径使用仅用于固定 route 转发的 `route_registrar.go` 和 `group:"*_routes"` 注入。

**Non-Goals:**

- 不改变任何 HTTP API path、method、请求响应结构、错误契约或 OpenAPI 注解。
- 不改变 auth、permission、role、user 的 controller、use case、store、policy sync 或 Casbin 授权行为。
- 不引入插件式 feature enable/disable、动态 route discovery 或配置化路由开关。
- 不改变数据库 schema、migration、Redis key、Prometheus/Grafana 资产或部署清单。

## Decisions

### 决策 1：集中注册固定 feature 路由

将 `router.RouteParams` 从三类 registrar slice 调整为固定 controller 输入或固定 route controller 聚合输入。`registerV1Routes` 仍创建 public、authenticated 和 authorized 三个 Gin group，但直接调用现有 transport route 函数：auth public/protected、permission、role 和 user。

选择该方案是因为当前服务的 feature 集合固定，且主规格和测试已经要求 auth、permission、role、user route graph 必须存在。集中注册能让安全层级和实际 path 挂载点在一个位置可见。

备选方案是合并为一个 `RouteRegistrar` 并传入 `RouteGroups`。该方案保留扩展性，但仍需要每个 feature 维护薄 registrar，且注册到哪个 group 依赖实现约定，不如固定服务集中注册直接。

### 决策 2：保留 router 层负责安全中间件分层

认证和 RBAC 中间件继续只在 router composition 中安装。public auth route 不经过 access token middleware；authenticated auth route 经过 `AuthWithTokenVersionValidator`；permission、role 和 user route 在 authenticated group 之上再经过 `permissionhttp.Authorize`。

选择该方案是因为安全分层是现有设计的核心收益，简化注入方式不应把认证或授权决策下放给各 feature transport。

备选方案是让每个 feature 自己安装 middleware。该方案会扩大出错面，容易出现新增路由遗漏 RBAC 或重复安装 middleware，因此不采用。

### 决策 3：Fx group 只从路由装配中移除，不扩散到 application 边界

feature `fx.go` 继续负责提供 application、infrastructure 和 HTTP controller。路由层不再消费 `group:"public_routes"`、`group:"authenticated_routes"` 和 `group:"authorized_routes"`，而是由 providers 或 router 参数显式消费固定 controller。

选择该方案是为了把 DI 元数据留在 composition 边界，同时避免 controller constructor 暴露 Fx、Dig 或 named/group metadata。

备选方案是把 `fx.In` 直接嵌入 controller 或 transport constructor。该方案违反 feature transport 的 framework-neutral constructor 边界，因此不采用。

### 决策 4：用现有测试覆盖行为不变

更新 `router_registration_test`、providers graph 测试和 architecture lint 期望，确保集中注册后的 route graph 仍包含相同 runtime 和 `/api/v1` 路由，且 permission baseline 与 authorized route graph 仍双向一致。

选择该方案是因为本变更是结构重构，主要风险是漏挂路由或挂错 middleware。行为测试比新增生产兼容层更合适。

备选方案是保留旧 registrar 与新集中注册并行一段时间。该方案会带来重复路由、双路径维护和测试歧义，不适合当前无外部插件消费者的内部重构。

## Risks / Trade-offs

- [Risk] 集中注册降低未来 feature 自注册扩展性 → Mitigation: 当前服务 feature 固定，未来若出现真实插件式 feature 需求，再以独立 OpenSpec change 引入新的注册 contract。
- [Risk] 重构时漏挂某个 path 或挂到错误访问层级 → Mitigation: 保留并更新 route graph 测试、auth middleware 测试和 RBAC baseline 双向校验。
- [Risk] providers/router 参数直接 import feature HTTP 包增加 composition root 耦合 → Mitigation: 耦合限制在 user-service composition/router 层，不进入 application、domain、common 或 shared 边界。
- [Risk] 删除 route registrar 影响 Fx graph 测试或 architecture lint 规则 → Mitigation: 同步更新测试和 lint 期望，明确新规则是不再要求固定 feature 使用 route registrar。

## Migration Plan

1. 调整 route composition 参数，使路由注册入口显式接收 auth、permission、role 和 user controller 以及安全依赖。
2. 在 router 中集中创建 public、authenticated 和 authorized group，并调用现有 feature transport route 注册函数。
3. 从 feature `fx.go` 中移除 route registrar provider 和对应 Fx group tag，删除或停用薄 `route_registrar.go`。
4. 更新 route graph、provider graph、architecture lint 相关测试。
5. 运行 `make user-service-test` 和 `make user-service-architecture-lint`；如触及 OpenAPI 注解或生成物，再运行 `make user-service-openapi-generate`。

回滚方式是恢复原三类 route registrar contract、feature route registrar provider 和 `providers.RegisterRoutes` 对 Fx group 的消费。由于不涉及数据或部署资产，回滚不需要 migration。

## Open Questions

无。当前方案按固定 user-service feature route graph 实施，不预留动态插件式路由扩展。
