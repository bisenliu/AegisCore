## Why

当前 user-service 在 Gin engine 初始化时显式禁用 trusted proxy，登录审计上下文和 HTTP access log 中的 `client_ip` 只能来自 TCP peer。服务部署在 Ingress、gateway、ALB、Envoy、Nginx 或 service mesh 后方时，该字段会记录代理地址而非真实客户端地址，导致认证审计、风险识别和安全排障信号失真。

## What Changes

- **BREAKING**：将当前“应用不接受 trusted proxy 配置”的运行时约束改为“应用必须只信任显式配置的上游代理 CIDR”。
- 在共享 runtime 配置契约中新增 `server.http.trusted_proxies`，用于配置 Gin `SetTrustedProxies` 的受信任代理 IP 或 CIDR 列表。
- Gin engine 使用 `server.http.trusted_proxies` 解析 `X-Forwarded-For` / `X-Real-IP`；未配置时继续不信任任何代理，不增加兼容分支或旧配置别名。
- 登录认证审计上下文和共享 HTTP 日志字段统一使用 Gin trusted proxy 解析后的 `c.ClientIP()`，controller 不自行解析 forwarded headers。
- 部署和配置文档明确要求入口层清洗 forwarded headers，并为各环境配置真实上游代理 CIDR。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `shared-platform-primitives`：新增共享 HTTP runtime trusted proxy 配置契约，并明确 `client_ip` helper 只依赖 Gin trusted proxy 校验后的结果。
- `auth-session-management`：登录安全审计上下文中的 `client_ip` 改为可信代理解析后的客户端地址。
- `runtime-observability`：HTTP access log 和认证失败日志中的 `client_ip` 字段改为可信代理解析后的客户端地址。
- `delivery-operations`：部署、Nacos、Kubernetes 和 Helm 文档需要提供 trusted proxy 配置与入口 header 清洗要求。

## Impact

- 代码：`common/runtime/config/`、`user-service/internal/providers/gin.go`、共享 HTTP middleware 测试、认证 controller 测试和 Gin trusted proxy 测试。
- 配置：新增 `server.http.trusted_proxies`，不读取、不兼容 `http.trusted_proxies` 或其他旧位置。
- 安全：只在请求来自显式受信任代理时接受 forwarded headers，避免无条件信任客户端可伪造的 `X-Forwarded-For`。
- 观测：`client_ip` 日志字段语义从 TCP peer 改为 Gin trusted proxy 校验后的客户端地址。
- 部署：生产和集成环境需要按入口拓扑配置代理 IP/CIDR，并保证 Ingress、gateway 或 service mesh 在边界处覆盖或重建 forwarded headers。
- API 和数据库：不改变 HTTP API、OpenAPI、Ent schema 或 Atlas migration。
