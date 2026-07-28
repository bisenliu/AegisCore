## Why

当前 user-service 缺少统一 API 限流能力，登录、refresh 和已认证业务接口在高请求量或恶意流量下会直接消耗认证、RBAC、数据库和 Redis 资源。随着项目后续请求量增长，需要在服务内提供可观测、可配置、可清理的限流兜底，并为后续网关或 Redis 全局限流保留清晰边界。

## What Changes

- 新增 API 限流能力，使用 `golang.org/x/time/rate` 实现服务内 token bucket 限流。
- 匿名接口按客户端 IP 限流，覆盖登录、refresh 和强制改密等未认证入口。
- 普通已登录业务接口在 JWT 和 token version 校验通过后按 User ID 限流，并在 RBAC 授权前拒绝超限请求。
- 服务内本地限流器使用分片 map 降低锁竞争，并通过后台 janitor 定期清理长时间未访问 key，不保留请求路径惰性全量清理方案。
- 新增稳定限流错误语义，超限请求返回 `429 Too Many Requests` 和统一失败 envelope。
- 限流中间件保持业务中立，核心 key 解析与路由挂载策略由消费服务拥有。
- 健康检查、startup/readiness、metrics、OpenAPI 和 pprof 不纳入业务限流。

## Capabilities

### New Capabilities

- `api-rate-limiting`: 定义 API 限流契约，覆盖匿名 IP 限流、已认证 User ID 限流、本地限流资源生命周期、清理策略和超限响应。

### Modified Capabilities

- `shared-platform-primitives`: 新增共享 HTTP middleware 与应用错误契约中的限流错误语义、`429 Too Many Requests` HTTP 映射和业务中立限流 primitive。
- `auth-session-management`: 认证公开入口和已认证会话控制接口新增限流门禁要求，认证通过后可按 User ID 获取限流身份。
- `rbac-access-control`: 受 RBAC 保护的业务接口新增认证后、授权前的限流门禁要求。

## Impact

- 影响 `common/contract/errors`、`common/http/response` 和 `common/http/middleware`，新增限流错误码、HTTP status 映射、限流中间件和测试。
- 影响 `user-service/internal/router/router.go`，调整 `/api/v1/auth` 公开路由和已认证业务路由的 middleware 链。
- 影响 user-service 私有配置与校验，新增匿名接口、已认证接口、本地 store 分片数、key TTL 和清理间隔等限流配置。
- 新增 Go 依赖 `golang.org/x/time/rate`。
- 不影响数据库 schema、Ent 生成物、Atlas migration、OpenAPI 路由定义、RBAC 权限基线或部署资产。
- 多副本部署下服务内限流仅作为单实例兜底；全局精确限流由后续网关或 Redis 能力承载，不在本次 change 中实现。
