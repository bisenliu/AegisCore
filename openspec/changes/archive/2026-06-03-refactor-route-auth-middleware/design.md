## Context

当前用户服务在 `NewGinEngine` 中把认证中间件作为全局中间件注册，并通过 `auth.whitelist` 对健康检查、Swagger、登录、刷新、改密等路径做前缀豁免。这个模式把安全边界分散到运行时配置，路由代码无法直接表达哪些接口公开、哪些接口需要认证，也不利于后续在局部路由组上接入 Casbin 授权中间件。

相关边界如下：`common/config` 负责配置结构和加载；`common/middleware` 提供可复用认证中间件；`user-services/internal/bootstrap` 负责 Gin engine 全局基础中间件；`user-services/internal/router` 负责 HTTP 路由分组和 handler 挂载。变更不改变 controller/service/repository 分层，也不引入新的数据模型或数据库迁移。

## Goals / Non-Goals

**Goals:**

- 从 YAML 示例、配置结构、配置加载测试和认证中间件中彻底移除 `auth.whitelist`。
- 将认证中间件从全局挂载迁移到用户服务路由局部分组挂载。
- 在路由注册代码中明确公开访问、仅认证访问、预留授权访问三类分组。
- 保持既有路由路径、响应信封、HTTP 401 错误语义和 trace-id 日志传播不变。
- 为后续 Casbin 中间件预留可组合挂载点，使授权逻辑可以接在认证之后、业务 handler 之前。

**Non-Goals:**

- 不在本变更中实现 Casbin adapter、policy model、enforcer provider 或权限策略存储。
- 不改变 JWT 签发、解析、token version 校验、Refresh Token 会话控制或密码校验语义。
- 不新增 API 路径，不改变 controller 方法职责，不调整数据库 schema。
- 不通过兼容代码继续接受 `auth.whitelist`，因为目标是彻底移除白名单配置依赖。

## Decisions

### Decision 1: 认证中间件不再读取白名单

`common/middleware.AuthWithTokenVersionValidator` 将删除 `cfg.Whitelist` 路径检查和 `isWhitelistedPath` helper。中间件只对实际挂载到它的请求执行 `Authorization: Bearer <token>` 校验、JWT 解析、token version 校验和认证上下文传播。

备选方案是保留白名单字段但不在默认配置中使用。该方案仍会留下运行时配置扩大公开面的风险，也会让 Casbin 前的认证边界继续不透明，因此不采用。

### Decision 2: 全局中间件只保留请求基础设施能力

`NewGinEngine` 继续全局挂载 `TraceID`、`Recovery`、`RequestLogger` 和 `CORS`，不再全局挂载认证。认证中间件所需的 `zap.Logger`、`JWTService` 和 `SessionStore` 由 bootstrap 组装成路由参数传给 `router.RegisterRoutes`，由路由层决定挂载位置。

备选方案是在 bootstrap 中创建多个 engine 或在 controller 内部校验认证。前者增加运行时复杂度，后者会破坏 controller 只做 HTTP 解析和响应映射的边界，因此不采用。

### Decision 3: 路由分组表达访问控制层级

`router.RegisterRoutes` 采用以下组织方式：

- `public`：直接挂载到 engine 或 `/api/v1` 下的公开路由，包括 `GET /healthz`、Swagger 文档、`POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/change-password`。
- `authenticated`：在 `/api/v1` 下挂载认证中间件，覆盖需要普通 Access Token 的路由，包括 `POST /api/v1/auth/logout`、`POST /api/v1/auth/logout-all` 和当前用户资料 API。
- `authorized`：在 `authenticated` 之下预留 Casbin 挂载点，后续可按资源域拆分为 `users`、`admin` 等组，并在认证中间件之后追加 Casbin 中间件。

`change-password` 保持公开路由组但仍要求请求携带改密凭据，由 controller/service 继续按受限改密凭据校验；它不应使用普通 Access Token 认证中间件，否则会和受限凭据用途冲突。

### Decision 4: Casbin 以路由组局部组合方式接入

后续 Casbin 中间件建议提供类似 `Authorization(enforcer, resource, action)` 或从 Gin route metadata 推导 resource/action 的 Gin middleware。挂载顺序应为 `TraceID -> Recovery -> RequestLogger -> CORS -> Auth -> Casbin -> handler`。当前变更只建立 `authenticated` 与 `authorized` 的结构位置，不引入空的 Casbin stub，避免产生未使用依赖。

## Risks / Trade-offs

- 配置中残留 `auth.whitelist` 不再生效 -> 在实施和发布说明中明确这是 breaking change，并通过配置加载测试确认该字段已从契约移除。
- 某个原白名单公开路径忘记迁移到公开路由组 -> 增加路由级测试，覆盖健康检查、Swagger、登录、刷新、改密未携带普通 Access Token 时不被认证中间件拒绝。
- 受保护路由误挂到公开组 -> 增加缺失认证 header 的路由测试，覆盖用户资料 API、退出当前设备和退出全部设备返回 HTTP 401。
- Casbin 资源动作命名未来可能调整 -> 当前只预留分组和挂载顺序，不固化 policy model，降低后续授权设计约束。

## Migration Plan

1. 移除 `config.AuthConfig.Whitelist`、默认 YAML 中 `auth.whitelist`、配置加载测试中的白名单断言和环境变量样例。
2. 修改认证中间件实现和测试，删除白名单放行行为，保留认证失败响应、日志和上下文传播行为。
3. 调整 bootstrap 与 router 参数，使认证中间件在路由层局部挂载。
4. 更新路由测试和相关主规格，验证公开路由不触发普通认证、受保护路由必须认证。
5. 发布时删除部署配置中的 `auth.whitelist`。如需回滚，可恢复旧代码和旧配置字段；本变更不涉及数据库迁移或持久化数据回滚。

## Open Questions

无。当前变更只重构认证挂载边界；Casbin model、policy 存储、资源动作命名和超级管理员策略在后续独立 change 中确定。
