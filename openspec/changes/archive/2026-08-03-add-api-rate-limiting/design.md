## Context

user-service 当前通过 Gin route graph 挂载公开认证接口、已认证会话控制接口和 RBAC 保护的业务接口。认证中间件 `common/http/middleware.AuthWithTokenVersionValidator` 在 access token 和 token version 校验通过后，会把 `user_id` 写入 `request.Context()` 和 Gin context；RBAC 中间件随后使用该身份、`FullPath()` 和 HTTP method 做授权。现有链路没有限流门禁，登录、refresh、强制改密、用户、角色和权限接口在高请求量下会直接消耗密码校验、JWT、Redis、PostgreSQL 和 Casbin 资源。

本次 change 横跨 `common` 和 `user-service`：`common` 提供业务中立错误契约、HTTP 渲染和本地限流 primitive；`user-service` 私有拥有路由挂载策略、匿名与已认证限流配置、资源生命周期接线和服务内默认值。数据库 schema、OpenAPI 路由形态、RBAC 权限基线、部署清单和观测 dashboard 不在本次变更范围内。

## Goals / Non-Goals

**Goals:**

- 为匿名公开认证接口提供按客户端 IP 的服务内限流。
- 为普通已登录业务接口和已认证会话控制接口提供认证后按 User ID 的服务内限流。
- 使用 `golang.org/x/time/rate` 作为本地 token bucket 实现。
- 使用分片本地 store 降低高请求量下的锁竞争。
- 使用后台 janitor 定期清理长时间未访问的 limiter key，不采用请求路径惰性全量清理。
- 新增稳定限流错误语义，并由共享 response helper 渲染 `429 Too Many Requests` 失败 envelope。
- 通过 Fx lifecycle 显式启动和停止限流自有后台资源。
- 提供单元测试、路由注册测试和配置校验测试，覆盖匿名 IP、User ID、清理和超限响应。

**Non-Goals:**

- 不实现 Redis、网关或跨实例全局精确限流。
- 不改变 JWT claims、token version、refresh session 或 RBAC policy sync 语义。
- 不修改数据库 schema、Ent 生成物、Atlas migration 或 RBAC 默认权限基线。
- 不为旧配置字段、旧路由挂载方式或无清理本地 limiter 提供兼容路径。
- 不把 user-service 的限流策略、配置默认值或业务路由知识放入 `common`。

## Decisions

### 使用本地分片 limiter 作为本次实现

本次实现使用 `golang.org/x/time/rate` 提供单实例本地 token bucket，外层定义业务中立 `Limiter` 或 store primitive。key 按 hash 分配到多个 shard，每个 shard 持有独立 map 和锁，请求热路径只访问一个 shard。

备选方案是直接使用一个全局 map 和 mutex。该方案实现更少，但在高请求量和高 key 基数下容易出现锁竞争，且清理时会阻塞所有请求，因此不采用。

备选方案是直接实现 Redis Lua 全局限流。该方案可以跨实例精确计数，但会把每次业务请求绑定到 Redis 热路径，增加依赖复杂度和故障模式；本次需求先落地服务内兜底限流，Redis 全局限流留给后续独立能力。

### 使用后台 janitor 清理 limiter key

本地 store 为每个 key 保存 `lastSeen`，后台 janitor 按配置间隔扫描并删除超过 TTL 未访问的 key。janitor 由 user-service composition 通过 Fx lifecycle 启动，停止时使用 context 取消，保证没有 goroutine 泄漏。

备选方案是在请求路径中根据 `lastSweep` 顺带清理。该方案低流量场景简单，但高 key 基数下某个用户请求会承担扫描成本，并可能引入尾延迟尖刺；由于项目预期请求量很大，不采用。

### `common` 只承载业务中立 primitive

`common/http/middleware` 可以提供 Gin 限流中间件、key resolver 接口、`ClientIP` resolver、从已有 auth context 读取 User ID 的通用 resolver，以及本地限流 store。匿名接口、已认证接口、各自默认阈值和是否启用由 `user-service/internal/config` 与 `user-service/internal/router` 拥有。

备选方案是在 `user-service/internal/router` 内完整实现 limiter。该方案更局部，但会复制错误响应、清理和并发控制逻辑，后续其他服务无法复用，因此不采用。

### 超限响应进入共享错误契约

新增限流专用 `Kind`、`Reason` 和 `Code`，`common/http/response` 将其映射为 `429 Too Many Requests`。所有限流中间件通过 `response.Fail` 或等价共享 helper 输出统一 envelope，响应不暴露 key、IP、User ID、分片编号或内部错误。

备选方案是复用 `CodeServiceUnavailable` 并手写 `429`。该方案可以避免新增错误码，但会让限流与依赖不可用语义混淆，且不满足稳定 API 契约要求，因此不采用。

### 路由链路先认证、再限流、再授权

公开认证接口使用匿名 IP 限流，挂载在 `/api/v1/auth` public group。已认证 group 先运行 JWT 与 token version 校验，再运行 User ID 限流。RBAC 保护的业务接口在限流通过后才进入授权中间件。

备选方案是在认证前对所有 `/api/v1` 按 IP 限流。该方案会误伤 NAT 或公司出口下的正常已登录用户，且不能满足按 User ID 限流要求，因此不采用。

## Risks / Trade-offs

- [Risk] 本地限流在多副本下不是全局精确配额。→ Mitigation: 在配置和文档中明确其为单实例兜底，后续用网关或 Redis 独立 change 承载全局限流。
- [Risk] 后台清理扫描在 key 极多时仍可能造成 shard 锁短暂竞争。→ Mitigation: 使用分片 store，并允许配置 shard 数、TTL 和 cleanup interval；实现时可按 shard 分散扫描，避免单轮长时间持锁。
- [Risk] 客户端 IP 解析依赖 Gin trusted proxies。→ Mitigation: 使用 Gin `ClientIP()`，部署侧必须配置 trusted proxies；测试覆盖 `X-Forwarded-For` 行为时只验证 Gin 约定，不在限流中间件中重新实现代理解析。
- [Risk] 限流配置过严会影响正常用户。→ Mitigation: user-service 配置必须提供启停开关和明确默认值，发布时允许按环境调整。
- [Risk] 限流错误码是新公开契约。→ Mitigation: 同步更新 `common/contract/errors`、`common/http/response`、测试和 OpenSpec delta，保持 envelope 稳定。

## Migration Plan

1. 在 `common` 新增限流错误语义、HTTP status 映射、本地 limiter store 和 Gin middleware。
2. 在 `user-service/internal/config` 新增限流配置、默认值和校验，不读取或兼容旧字段。
3. 在 `user-service/internal/providers` 或 router composition 中构造限流资源，并通过 Fx lifecycle 启停 janitor。
4. 在 `user-service/internal/router/router.go` 将匿名 IP 限流挂载到公开认证接口，将 User ID 限流挂载到已认证 group 的认证中间件之后、RBAC 中间件之前。
5. 增加 common middleware/error tests、user-service config tests 和 router registration tests。
6. 执行 `make test`、`make user-service-architecture-lint`、`make lint` 和 `make verify`。

回滚时移除路由中的限流 middleware 挂载，保留代码不会改变接口行为；如需完全回滚，删除本次新增配置、common primitive 和错误契约，并同步回滚测试和 OpenSpec change。

## Open Questions

无。初始阈值、TTL、cleanup interval 和 shard 数采用 user-service 私有默认值，并通过配置显式调整。
