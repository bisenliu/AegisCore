## Why

用户服务已经具备 normalized RBAC 数据模型、权限目录、角色绑定能力，以及 permission feature 内的内存 Casbin policy engine，但 HTTP 运行时尚未把 JWT 认证后的业务受保护路由接入 RBAC 授权。当前只要 access token 有效，用户即可进入受保护业务路由，权限目录和角色授权无法在请求边界生效。

本变更在 Gin 路由层增加 RBAC 授权中间件，并将其挂载在 JWT 认证之后、业务受保护路由之前，使受保护业务 API 在进入 controller 前按当前内存 policy 做 fail-closed 授权判断。

## What Changes

- 在 `user-service/internal/features/permission/application/authorization/` 新增面向 HTTP 中间件的授权服务接口和实现边界。
- 授权服务接收 `userID string`、Gin route template、HTTP method，并在内部将用户 subject 规范化为 `user:<user_uuid>` 后委托现有 Casbin engine 判断。
- 保持角色 policy 关系由现有 policy loader 根据用户绑定的 `role_id` 构造为 `role:<role_uuid>`，不依赖 `roles.code`。
- 在 `user-service/internal/features/permission/transport/http/` 新增 Gin 授权中间件。
- 中间件从认证中间件写入的 Gin context 或 request context 读取 `user_id`。
- 中间件使用 `c.FullPath()` 作为 Casbin object，使用 `c.Request.Method` 作为 Casbin action。
- 中间件支持显式白名单，并默认让 `OPTIONS` 请求绕过 Casbin，避免预检请求被误拦截。
- 在 `user-service/internal/router/router.go` 中将 RBAC 中间件挂载到 JWT 认证之后、用户/权限/角色等业务受保护路由之前。
- 保持 public auth、health、Swagger 不进入 RBAC 中间件。
- 补充单元测试或路由级测试，覆盖挂载顺序、白名单、未授权 403、`c.FullPath()`、无每请求数据库访问等行为。

## Non-Goals

- 不实现角色 CRUD。
- 不实现权限 CRUD。
- 不实现策略刷新触发机制。
- 不实现 Redis 多实例同步。
- 不改变 JWT 认证逻辑或 token version 校验逻辑。
- 不使用实际 URL path 代替 `c.FullPath()`。
- 不使用 `keyMatch2` 作为第一期默认匹配方式。
- 不依赖 `roles.code`。
- 不新增 `casbin_rules` 表。

## Impact

- Affected code: `user-service/internal/features/permission/application/authorization/`、`user-service/internal/features/permission/transport/http/`、`user-service/internal/features/permission/fx.go`、`user-service/internal/providers/routes.go`、`user-service/internal/router/router.go`。
- Affected runtime behavior: JWT 认证通过后，业务受保护路由会先进行 RBAC 授权；未授权请求返回统一 response envelope 的 403。
- Affected tests: user-service 路由和 permission HTTP middleware 测试需要覆盖授权成功、拒绝、认证信息缺失、白名单和 `OPTIONS` bypass。
- Operational note: 每次请求只访问内存 Casbin enforcer，不访问数据库；policy 新鲜度仍依赖后续独立的 reload 触发机制。
