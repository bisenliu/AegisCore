## Context

user-service 的 Gin engine 当前在 `user-service/internal/providers/gin.go` 中调用 `SetTrustedProxies(nil)`，并通过测试固定为不信任 `X-Forwarded-For` 和 `X-Real-IP`。认证登录控制器把 `c.ClientIP()` 写入 `authctx.ClientContext`，共享 HTTP middleware 也使用 `c.ClientIP()` 生成 access log 和认证失败日志字段。

该设计在服务没有 trusted proxy 配置契约时可以避免客户端伪造 forwarded headers，但在真实部署拓扑中，user-service 通常位于 Ingress、gateway、ALB、Envoy、Nginx 或 service mesh 后方。此时应用层 TCP peer 是代理地址，导致认证审计和观测日志中的 `client_ip` 无法表达真实客户端来源。

本次变更跨越 `common/runtime/config`、`user-service/internal/providers`、认证 HTTP 边界、共享 middleware 日志语义、部署文档和 OpenSpec 主规格。它不改变 HTTP API、OpenAPI、数据库 schema、RBAC policy 或 Redis key schema。

## Goals / Non-Goals

**Goals:**

- 新增唯一稳定配置项 `server.http.trusted_proxies`，使用 IP 或 CIDR 列表显式声明受信任上游代理。
- Gin engine 使用该配置调用 `SetTrustedProxies`，让 `c.ClientIP()` 只在请求来自受信任代理时解析 forwarded headers。
- 登录认证审计上下文、共享 access log 和认证失败日志统一保留 `c.ClientIP()` 作为客户端地址来源。
- 未配置 trusted proxies 时保持安全默认值：不信任任何代理，也不读取旧配置位置。
- 部署文档明确入口层必须覆盖或重建 forwarded headers，生产配置必须按真实入口拓扑填入代理 CIDR。

**Non-Goals:**

- 不新增 controller 或 middleware 层的手写 `X-Forwarded-For` / `X-Real-IP` 解析逻辑。
- 不兼容、不迁移、不读取 `http.trusted_proxies` 或其他旧配置键。
- 不改变登录、refresh、logout、RBAC 授权、OpenAPI、数据库 migration 或 Redis 数据结构。
- 不为不同代理产品增加特定分支；代理拓扑差异只通过 `server.http.trusted_proxies` 表达。

## Decisions

### 使用 Gin trusted proxy 机制作为唯一解析入口

`NewGinEngine` 将从 `params.Config.Server.HTTP.TrustedProxies` 读取受信任代理列表，并直接传给 `engine.SetTrustedProxies`。controller、authctx 和共享日志 middleware 继续消费 `c.ClientIP()`。

备选方案是新增 middleware 无条件解析 `X-Real-IP` 或 `X-Forwarded-For` 并写入 context。该方案会绕过 Gin 对 remote peer 是否可信的校验，一旦服务被绕过入口直接访问或入口未清洗 header，客户端可伪造审计 IP，因此不采用。

### 配置归属放在共享 HTTP runtime 配置

`server.http.trusted_proxies` 属于 HTTP server 运行时行为，放在 `common/runtime/config.HTTPServerConfig`。它不进入 auth 私有配置，因为该地址语义同时影响认证审计、access log、认证失败日志以及未来其他 HTTP 入口能力。

备选方案是把配置放在 user-service 私有配置或 auth 配置下。该方案会让共享 middleware 与 Gin engine 配置来源割裂，并错误地把通用 HTTP server 行为绑定到认证 feature，因此不采用。

### 不保留旧配置兼容路径

配置项只接受 `server.http.trusted_proxies`。现有严格解码继续拒绝 `http.trusted_proxies`，不新增 normalize、alias、迁移读取或双写行为。

备选方案是同时支持旧位置和新位置。该方案会扩大配置面，增加部署歧义，且当前主线已经显式拒绝旧 `http.trusted_proxies`，因此不采用。

### 默认值继续保持零信任代理

默认 `TrustedProxies` 为 `nil`，Gin 不信任任何代理。生产或集成部署若需要真实客户端 IP，必须显式配置入口代理 IP/CIDR。

备选方案是默认信任私有网段。该方案在多租户网络、节点直连或 service mesh 混合拓扑中可能误信任非入口客户端，安全边界不清晰，因此不采用。

## Risks / Trade-offs

- [Risk] 配置了过宽 CIDR 会信任非入口来源并允许 header 伪造。→ Mitigation：文档要求只配置真实入口代理 CIDR，并在测试和配置示例中避免默认私网全量信任。
- [Risk] 未配置 trusted proxies 的环境仍记录代理地址。→ Mitigation：保留安全默认值，并在部署文档、Nacos 示例、Kubernetes/Helm 说明中把该项列为生产入口拓扑必配项。
- [Risk] 入口层未覆盖客户端传入的 forwarded headers 时，可信代理会转发伪造链路。→ Mitigation：部署要求 Ingress、gateway 或 service mesh 在边界处覆盖或重建 forwarded headers，不透传未清洗原始值。
- [Risk] `client_ip` 日志字段语义变化可能影响日志查询或告警解释。→ Mitigation：runtime-observability 规格明确字段语义，发布说明同步说明从 TCP peer 变为可信代理解析后的客户端地址。

## Migration Plan

1. 更新 OpenSpec delta 和架构文档，确认 `server.http.trusted_proxies` 是唯一配置入口。
2. 更新 `common/runtime/config.HTTPServerConfig`、默认值、严格解码和校验测试。
3. 更新 `NewGinEngine` 使用 `server.http.trusted_proxies` 调用 `SetTrustedProxies`。
4. 更新 Gin trusted proxy 测试，覆盖受信任代理解析 forwarded client IP，以及未配置或不受信任代理时回退 TCP peer。
5. 更新认证 controller 和共享 middleware 相关测试，确认 `client_ip` 来源继续通过 `c.ClientIP()` 进入 authctx 和日志字段。
6. 更新 README、架构、Kubernetes、Helm 或 Nacos 配置示例，明确入口 header 清洗和代理 CIDR 配置要求。
7. 回滚时同步回滚应用代码和配置文档；运行环境可删除 `server.http.trusted_proxies` 使服务恢复不信任代理的安全默认行为。

## Open Questions

- 无。
