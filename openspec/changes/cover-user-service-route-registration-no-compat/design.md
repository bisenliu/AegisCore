## Context

`user-service/internal/router/router.go` 是 user-service HTTP route graph 的聚合入口，负责注册健康检查、OpenAPI、metrics、pprof，以及 `/api/v1` 下 auth、permission、role 和 user feature 路由。当前已有健康检查、OpenAPI、metrics、pprof 的局部测试，但缺少覆盖 `RegisterUserServiceHTTPRoutes` 和 `registerV1Routes` 的聚合测试。

本次变更属于测试补齐，不改变生产代码职责。测试需要以当前 Gin route inspection 为准，验证当前 `/api/v1` 路由图和中间件链，避免引入真实业务 use case、Ent、Redis、Casbin policy loader 或 OpenAPI 生成依赖。

## Goals / Non-Goals

**Goals:**

- 覆盖 `RegisterUserServiceHTTPRoutes` 的运行时路由组装，包含健康检查、OpenAPI、metrics、pprof 和 `/api/v1` feature 路由。
- 覆盖 `registerV1Routes` 的 auth、permission、role、user 路由注册和当前认证/RBAC 中间件链。
- 覆盖 Permission/Role controller 为 nil 时的条件注册行为。
- 覆盖 metrics 配置错误从 `RegisterUserServiceHTTPRoutes` 返回，以及 pprof 开关对注册结果的影响。
- 让新增测试满足 `docs/TESTING.md` 和 `delivery-operations` 主规格中的语义化断言规范。

**Non-Goals:**

- 不修改 feature controller 行为、JWT 校验、token version 校验、Casbin 授权或 OpenAPI 生成逻辑。
- 不新增 `/api`、`/v1`、旧 metrics path、旧 pprof base path 或旧认证绕过兼容别名。
- 不新增生产测试专用 API、跨 feature mock 包或真实业务 use case 依赖。
- 不修改数据库 schema、migration、部署资产、Prometheus/Grafana 资产或 OpenAPI 生成物。

## Decisions

### Decision: 使用 Gin route inspection 与请求边界验证注册结果

测试通过 `engine.Routes()` 收集 method 和 path，直接验证路由是否挂载到当前路径。Gin 当前 `RouteInfo` 不暴露 handler chain 长度，因此中间件链通过轻量 HTTP 请求结果验证：公开认证路由在无 access token 时进入绑定错误而不是认证失败；受保护路由在无 access token 时被认证中间件拦截；RBAC 路由在有效 access token 但授权拒绝时被授权中间件拦截。这样可以覆盖注册行为，同时避免触发真实业务 use case、Ent、Redis、Casbin policy loader 或 OpenAPI 生成细节。

备选方案是构造完整 HTTP 请求并模拟所有 use case。该方案会把路由注册测试耦合到 feature controller 业务行为，不符合本次“只验证 route graph 和 middleware chain”的范围。

### Decision: 使用轻量 controller 构造依赖，避免真实业务实现

对于必须传入的 controller，测试只需要能取得 handler method value，可使用零值 controller 指针或最小构造对象完成路由注册；测试不调用这些 handler。对于 Permission/Role 的 nil 条件注册，测试直接传 nil controller 并断言对应可选路由缺失。

备选方案是在 router 包新增 adapter 或测试 hook。该方案会引入仅服务测试的生产代码，不符合架构边界中避免测试驱动冗余生产代码的要求。

### Decision: 中间件链使用边界响应验证

公开 auth 路由应在无普通 access token 时进入请求绑定错误；受保护 auth 路由应在无 access token 时返回认证失败；permission、role 和 user 业务路由应在有效 access token 但授权拒绝时返回 RBAC 授权失败。测试通过这些边界响应验证当前链路，不断言 Gin 内部 handler 名称。

备选方案是断言完整 handler name。该方案对函数名、编译器符号和 wrapper 细节过于敏感，不利于路由注册契约长期稳定。

### Decision: OpenSpec delta 表达测试覆盖要求

本次不改变主流程生产行为，因此各 capability delta 增加“路由注册测试覆盖”要求，而不是修改原有业务需求语义。delivery-operations delta 约束新增测试的断言规范和覆盖率验收。

## Risks / Trade-offs

- [Risk] Gin route inspection 只能证明路由和中间件链已注册，不能证明 controller 内部业务行为正确。Mitigation：controller 行为继续由各 feature transport/application 测试覆盖，本次只覆盖 router 聚合边界。
- [Risk] 请求边界验证只能覆盖关键中间件是否位于 controller 前，不能枚举 Gin 内部完整 handler chain。Mitigation：route graph 由 `engine.Routes()` 覆盖，认证与 RBAC 边界由响应结果覆盖；如未来需要精确 chain 结构，应在生产代码暴露稳定诊断能力前先更新设计。
- [Risk] metrics provider 构造或 pprof 注册可能带来额外测试成本。Mitigation：使用禁用/最小配置和现有 provider API，避免真实 scrape 输出验证。

## Migration Plan

无运行时迁移。实施步骤为新增 OpenSpec change artifacts、补充 `user-service/internal/router` 测试、运行 router 包覆盖率和 OpenSpec 验证。若需要回滚，删除新增测试和本 change artifacts 即可恢复当前行为。

## Open Questions

无。
