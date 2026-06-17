# Expose user-service metrics endpoint

## What

在用户服务中显式暴露 Prometheus metrics scrape endpoint，默认路径为 `GET /metrics`。

本变更只做服务侧 wiring：

- 在 `user-service/internal/providers` 中接入 `common/runtime/observability/metrics` provider。
- 在服务级路由组装中根据 `observability.metrics.enabled` 挂载配置化 metrics path，默认 `/metrics`。
- 使用 common metrics provider 的独立 Prometheus gatherer 输出 Prometheus 文本格式。
- 确保 metrics endpoint 不进入 `/api/v1` feature 路由组，不经过 JWT 认证或 RBAC 授权。
- 成功 metrics scrape 跳过 HTTP access log，减少 Prometheus 高频 scrape 噪声；失败请求仍保留 access log。
- 增加 route/provider 测试覆盖启用、禁用、路径配置、Prometheus 文本响应、access log 跳过和 RBAC 白名单行为。

## Why

`common/runtime/observability/metrics` 已经提供 Prometheus 独立 registry/provider、可选 Go runtime/process collector、低基数 label 约定和 Fx 友好构造，但它按边界规则不自动挂载 HTTP 路由。用户服务当前配置中已经有 `observability.metrics.enabled`、`observability.metrics.path` 和 `observability.metrics.include_runtime`，开发文档也说明服务侧需要单独显式挂载 endpoint。

没有 `/metrics` 路由时，即使启用了 metrics provider，Prometheus 也无法 scrape 用户服务自身的 runtime/process 指标。这会让服务在上线后缺少基础运行时可观测性，也无法验证后续 HTTP、scheduler 或 workerpool 指标适配是否可被 scrape。

把 `/metrics` 挂载放在 `user-service/internal/providers` 与 `user-service/internal/router` 边界，可以保持 feature-first 结构清晰：metrics 是服务运行时能力，不属于 user/auth/role/permission 任一业务 feature，也不应进入 RBAC 权限目录。

## Scope

包括：

- 将 common metrics Fx provider 接入 `providers.Module`，从共享配置构造用户服务 metrics provider。
- 在 `router.RouteParams` 或等价服务级路由输入中传入 metrics provider 和 metrics 配置。
- 新增 router-owned metrics route helper，或在现有服务级路由组装中以独立根路径注册。
- 仅当 `observability.metrics.enabled: true` 且 provider enabled 时注册 metrics route。
- 使用 `observability.metrics.path` 作为 endpoint path；配置默认值为 `/metrics`。
- 对 metrics path 做服务侧防御性规范化和校验，避免空路径、非 `/` 开头路径、通配符路径或与已有健康/OpenAPI/API 根路径冲突。
- 使用 `promhttp.HandlerFor(provider.Gatherer(), ...)` 输出 Prometheus text exposition format。
- 确认 metrics endpoint 不进入 JWT authentication middleware，也不进入 permission RBAC middleware。
- 扩展成功请求 access log skip 逻辑，使成功 metrics scrape 跳过 access log，失败仍记录。
- 扩展健康探针 tracing/access log skip 的命名或实现，避免把 metrics 混入 health probe 语义。
- 增加测试覆盖：
  - metrics disabled 时不注册 route。
  - metrics enabled 且默认 path 时 `GET /metrics` 返回 Prometheus 文本格式。
  - 自定义 path 生效。
  - metrics route 不触发 RBAC authorizer。
  - 成功 scrape 跳过 access log。
  - 失败 scrape 或未命中请求仍可见。
  - route scanner 不把 metrics route 纳入 RBAC permission route diff。
- 更新 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `user-service/configs/config.yaml` 注释，使文档从“未来路由”更新为“启用后由用户服务暴露”。

不包括：

- 不新增 auth/user/role/permission 业务指标。
- 不新增 HTTP request duration/request count 指标采集 middleware。
- 不新增 scheduler 或 workerpool Prometheus adapter。
- 不新增 Grafana dashboard、告警规则、Prometheus scrape 配置、ServiceMonitor、PodMonitor、Helm chart 或 Kubernetes 部署变更。
- 不改变 `/livez`、`/readyz`、`/startupz` 的路径、响应契约或依赖检查语义。
- 不修改 RBAC permission baseline，不把 `/metrics` 写入正式权限目录。
- 不修改数据库 schema、Ent generated code、Atlas migration、Redis key schema 或业务 API 响应契约。
- 不暴露 DSN、token、Authorization header、Cookie、SQL、Redis key、原始错误消息或其他敏感信息。
- 不新增 `openspec/` 或 `docs/opsx/`。

## Acceptance Criteria

- `observability.metrics.enabled: true` 时，用户服务注册 configured metrics path，默认 `GET /metrics`。
- `GET /metrics` 返回 HTTP 200，`Content-Type` 为 Prometheus text exposition 相关类型，响应体包含可 scrape 的 metric family。
- `observability.metrics.include_runtime: true` 时，metrics scrape 可看到 Go runtime/process 指标；`false` 时不要求出现 runtime/process 指标。
- `observability.metrics.enabled: false` 时，不注册 configured metrics route，请求该路径返回未命中路由的行为。
- 自定义 `observability.metrics.path` 生效，且默认 `/metrics` 不被额外注册。
- metrics endpoint 不经过 JWT authentication，不经过 permission RBAC authorizer，不出现在 route diff 的可授权路由集合中。
- 成功 metrics scrape 不产生 HTTP access log；失败或非成功状态仍按现有 access log 规则可见。
- metrics endpoint 不改变 `/livez`、`/readyz`、`/startupz`。
- 实现后 `make test-user-service` 和 `make architecture-lint` 通过。
