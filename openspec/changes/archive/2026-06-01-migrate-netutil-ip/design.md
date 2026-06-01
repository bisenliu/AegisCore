## Context

AegisCore 的 `common` 模块已经集中提供共享配置、日志、中间件、响应契约和基础设施装配。当前 `common/middleware.RequestLogger` 记录 `client_ip` 时直接使用 `gin.Context.ClientIP()`，而待迁移的 `go-micro-scaffold/common/pkg/netutil/ip.go` 已经实现了基于常见代理头的客户端真实 IP 提取逻辑。

该变更属于 `shared-infrastructure` 能力扩展：新增共享网络工具，而不是新增业务 API 或服务运行时依赖。

## Goals / Non-Goals

**Goals:**

- 在 `common` 模块提供可复用的 Gin 客户端 IP 提取工具。
- 保留源实现的头部优先级，并优化空白值、多 IP 值和 fallback 处理。
- 通过单元测试明确代理头解析、空值忽略和 Gin fallback 行为。
- 让现有请求日志可以复用同一 IP 提取逻辑，避免日志与其他调用点行为不一致。

**Non-Goals:**

- 不新增 HTTP 路由、控制器、业务 service 或 repository。
- 不改变响应信封、错误码、配置加载、Redis/PostgreSQL/Fx 装配或数据库 schema。
- 不引入对代理可信网段、私有 IP 过滤或安全策略配置的完整实现；这些策略可以作为后续独立变更设计。

## Decisions

- 将工具包放在 `common/netutil`。
  备选方案是放入 `common/middleware` 或 `user-services/internal`。选择 `common/netutil` 是因为 IP 提取不是单一中间件职责，也不应绑定到某个服务模块；该位置可以被共享中间件、controller 或未来服务复用。
- 暴露 `GetClientIP(c *gin.Context) string` 和头部常量。
  备选方案是创建不依赖 Gin 的 `http.Request` helper。当前迁移源和现有项目 HTTP 层均基于 Gin，保留 Gin 入口可以最小化迁移成本；未来如需标准库适配，可在同包新增独立函数。
- 头部解析顺序保持为 `X-Forwarded-For`、`X-Real-IP`、`X-Client-IP`、`c.ClientIP()`。
  该顺序与源实现兼容，并覆盖常见反向代理部署。优化点是忽略空白 header 和 `X-Forwarded-For` 中的空白分段，避免返回空字符串或带空格值。
- `common/middleware.RequestLogger` 使用 `netutil.GetClientIP(c)`。
  备选方案是只新增工具不改现有调用点。选择复用可以立即让稳定能力中的请求日志 `client_ip` 字段使用统一逻辑，同时不改变日志字段名或日志结构。

## Risks / Trade-offs

- 代理头可被客户端伪造 -> 该工具仅统一提取顺序，不声明可信代理安全边界；生产环境仍应由 Gin trusted proxies 或网关策略控制可信来源。
- `X-Forwarded-For` 中第一个非空值可能不是部署期望的真实客户端 -> 当前保持源实现兼容和行业常见默认；如需按可信链路从右向左解析，应后续增加可配置策略。
- 请求日志中的 `client_ip` 可能从 Gin fallback 变为代理头值 -> 这是该变更的目标行为，字段名、响应和 API 均保持兼容。
