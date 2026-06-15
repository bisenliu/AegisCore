## Context

当前 `user-service/internal/router/router.go` 已将 `/api/v1/auth` public 路由挂载在 JWT 中间件之外，并将 protected auth 路由、permission、role、user 路由挂载在 JWT 中间件之后。代码中已有注释说明未来 Casbin 授权中间件应在认证之后挂载。

permission feature 已有 `infrastructure/casbin.Engine`，它从 normalized RBAC 表加载 active user-role 与 role-permission 关系，并在内存中执行 `Enforce`。该 engine 当前接收 `uuid.UUID` 类型用户 ID。Gin 中间件边界接收到的是认证中间件写入的字符串 `user_id`，因此需要一个 application authorization 层作为 transport-neutral 适配边界，负责输入规范化和授权调用。

## Goals / Non-Goals

**Goals:**

- 在 permission application 层提供面向中间件的 `Authorizer` 接口：`Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error)`。
- 在 authorization service 内部解析并校验 `userID`，把有效 UUID 交给现有 Casbin engine，由 engine 继续产生 `user:<user_uuid>` subject。
- 保持 role subject 和 role relation 由现有 Casbin loader 使用 `role:<role_uuid>` 表达。
- 在 permission HTTP transport 层实现 Gin middleware，读取认证上下文、获取 `c.FullPath()`、调用授权服务并按统一 response envelope 输出 401/403/500。
- 中间件显式支持白名单，使个别已认证但不需要 RBAC 的路由可跳过授权。
- 将 RBAC middleware 挂载在 JWT middleware 之后、业务受保护路由之前。
- 保证 public auth、health、Swagger 和 `OPTIONS` 不进入 Casbin 授权。
- 保证授权判断每请求不访问数据库。

**Non-Goals:**

- 不改造 Casbin model 为 path matcher 或 `keyMatch2`。
- 不把 raw URL path 作为 policy object。
- 不实现 policy reload 触发、订阅、Redis 同步或多实例一致性。
- 不改变登录、刷新、登出和 token version 校验逻辑。
- 不新增持久化 schema 或 `casbin_rules` 表。

## Decisions

### Decision: authorization package belongs to permission application

新增 `permission/application/authorization`，定义中间件消费的 `Authorizer` 接口和默认 service。该 service 是 permission feature 的业务授权入口，接收 transport-neutral 输入，不导入 Gin，也不直接访问 Ent/Redis。

Rationale: 授权是 permission feature 的 application 能力，不是 common runtime primitive。把接口放在 application 层可以避免 HTTP middleware 直接依赖 infrastructure/casbin 细节，同时保持消费边界清晰。

Alternative considered: 在 `transport/http` 直接依赖 Casbin engine。该方案更短，但会让 transport 直接绑定 infrastructure 类型，并把 userID 解析等 application 输入规则放入 HTTP 层。

### Decision: service parses string userID before calling Casbin engine

`Authorizer.Enforce` 接收认证中间件暴露的字符串 user ID。实现中使用 UUID 解析校验，非法或空用户 ID 按拒绝处理并返回可归一化错误；有效 UUID 传给现有 Casbin engine。

Rationale: HTTP middleware 不应知道 Casbin engine 的 UUID 类型和 subject 规则。规范化集中在 authorization service，便于未来复用到其他 transport。

### Decision: middleware reads Gin context first, request context second

中间件优先读取 `c.Get(auth.UserIDKey)`，以兼容当前认证中间件对 Gin context 的写入；如果不存在，再读取 `commonauth.UserIDFromContext(c.Request.Context())`，以兼容 request context 传播。两者都缺失或类型非法时返回统一 401。

Rationale: 用户要求支持从 Gin context 或 request context 读取 `user_id`。双读取能降低对认证中间件内部实现的耦合，不改变 JWT 认证逻辑。

### Decision: use c.FullPath as object and method as action

中间件必须在 Gin route 已匹配后读取 `c.FullPath()`，并将该 route template 传给 `Authorizer.Enforce`。如果 `FullPath()` 为空，按 fail-closed 返回 403。HTTP action 使用 `c.Request.Method`。

Rationale: 权限目录使用 route template 作为 policy object。使用 raw URL path 会导致 `/api/v1/users/:user_id` 与 `/api/v1/users/<uuid>` 不匹配，并绕开目录约束。

### Decision: whitelist is explicit and route-template based

中间件支持显式白名单，建议按 `method + pathTemplate` 进行精确匹配，也可保留 `method=*` 用于少量全方法 route template。白名单只应用于已进入 RBAC middleware 的 route group；public auth、health 和 Swagger 通过路由分组天然不进入该 middleware。

Rationale: 显式白名单便于处理受认证保护但无需 Casbin 的内部端点，同时避免隐式 pattern 或 raw path 匹配带来的放权风险。

### Decision: OPTIONS bypass lives in middleware

`OPTIONS` 请求在 RBAC middleware 内直接 `Next()`，不调用 Casbin。CORS middleware 已在 Gin engine 层处理跨域；该 bypass 防止预检请求因没有权限 catalog 记录而被拒绝。

Rationale: 验收要求保持 `OPTIONS` 不被 Casbin 误拦截。按 method 处理比向权限 catalog 写入所有 OPTIONS 更稳定。

### Decision: router creates an authorized subgroup after JWT

`registerV1Routes` 中保留 `authenticated := v1.Group("")` 并挂载 JWT middleware。protected auth routes 继续挂在 `authenticated.Group("/auth")`，不挂 RBAC。随后创建 `authorized := authenticated.Group("")` 并挂载 permission HTTP RBAC middleware，再注册 permission、role、user 等业务受保护路由。

Rationale: 这样可以保证执行顺序为 JWT -> RBAC -> business controller，同时 public auth 和 protected auth session control 不被 RBAC 拦截。health 与 Swagger 在 `/api/v1` 外或独立注册，也不受影响。

## API Shape

Application authorization package:

```go
type Authorizer interface {
    Enforce(ctx context.Context, userID string, pathTemplate string, method string) (bool, error)
}
```

HTTP middleware package:

```go
type AuthorizationOption func(*authorizationConfig)

func WithAuthorizationWhitelist(rules ...AuthorizationWhitelistRule) AuthorizationOption

func Authorize(authz authorization.Authorizer, opts ...AuthorizationOption) gin.HandlerFunc
```

Whitelist rule should be stable and explicit:

```go
type AuthorizationWhitelistRule struct {
    Method       string
    PathTemplate string
}
```

## Risks / Trade-offs

- Stale policy can deny newly granted permissions until reload is implemented or triggered; this change intentionally preserves current in-memory enforcement semantics and leaves reload workflow out of scope.
- Path template mismatch between permission catalog and Gin route registration will cause fail-closed 403; tests should assert `c.FullPath()` is passed to the authorizer.
- A broad whitelist could bypass RBAC for sensitive routes; keep whitelist definitions local, explicit, and covered by tests.
- Initial Casbin load failures currently make the engine fail closed; after middleware wiring this can make all business protected routes return 403 until policy load succeeds.

## Test Plan

- Unit-test authorization service parsing: valid UUID delegates to engine, invalid or empty user ID denies without panic.
- Unit-test middleware behavior: missing user ID returns 401, invalid Gin context user ID returns 401, empty `FullPath()` returns 403, denied access returns 403 envelope, authorizer error returns internal error envelope, allowed access calls next handler.
- Unit-test middleware input: authorizer receives `c.FullPath()` rather than raw URL path and receives `c.Request.Method`.
- Unit-test whitelist and `OPTIONS`: whitelisted route and `OPTIONS` do not call authorizer.
- Router/provider test: public auth routes do not call RBAC; protected business routes call JWT first and RBAC second.
- Run `go test ./...` under `user-service/`.

## Rollback

Rollback is code-only: remove the authorization service provider, HTTP middleware, route/provider wiring, and tests. No schema or data migration is introduced by this change.
