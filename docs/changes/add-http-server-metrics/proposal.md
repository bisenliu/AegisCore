# Add HTTP server metrics

## What

为用户服务 HTTP 入站请求补充 Prometheus RED 指标，覆盖请求量、错误量、请求延迟和 in-flight 请求数。

本变更将新增一个无业务语义的 Gin HTTP metrics middleware，并在用户服务 Gin engine 中接入：

- 在 `common/http/middleware` 提供可复用 HTTP server metrics middleware。
- 使用现有 `common/runtime/observability/metrics.Provider` 的独立 Prometheus registry/registerer。
- 采集 `http_server_requests_total`、`http_server_request_duration_seconds`、`http_server_in_flight_requests`。
- 指标 label 使用 Gin route template，未匹配路由使用稳定 fallback label，避免 raw URL path 高基数。
- label 至少包含 `method`、`route`、`status_code` 或 `status_class`；如记录应用错误码，只使用稳定低基数 `code`。
- 健康探针和 metrics scrape 的成功请求默认过滤或单独标记，避免污染业务 RED 面板。
- 在 `user-service/internal/providers/gin.go` 按正确顺序接入 middleware。
- 增加 common middleware 测试和 user-service wiring 测试。

## Why

用户服务已经具备 Prometheus metrics provider 和配置化 `/metrics` scrape endpoint，但当前缺少 HTTP 入站请求 RED 指标。上线后只能看到 runtime/process 等基础指标，无法通过 Prometheus 直接回答核心服务问题：

- 每个业务路由的请求量是多少。
- 哪些 route template 正在产生 4xx/5xx。
- 请求延迟分布是否恶化。
- 当前有多少请求正在 handler 中执行。

HTTP 请求指标必须避免高基数 label。直接使用 `URL.Path` 会把用户 UUID、cursor、token 或其他动态路径片段写入时间序列，导致 Prometheus cardinality 膨胀，也可能泄露敏感输入。Gin 已经能通过 `c.FullPath()` 提供 route template，因此应以 template 作为 `route` label，并为 404/未匹配请求提供固定 fallback。

把 middleware 放在 `common/http/middleware` 可以保持边界清晰：它只依赖 Gin 与 common metrics provider，不了解 user/auth/role/permission 的业务语义。用户服务只负责在服务级 Gin provider 中接入和决定运行时端点过滤策略。

## Scope

包括：

- 新增 common Gin HTTP metrics middleware，接收 metrics provider 和选项。
- 指标命名为：
  - `http_server_requests_total`
  - `http_server_request_duration_seconds`
  - `http_server_in_flight_requests`
- 请求完成后记录请求计数和延迟 histogram。
- 请求进入 middleware 时增加 in-flight，请求结束后减少 in-flight。
- label 至少包含 `method`、`route` 和 `status_class`；如使用 `status_code`，也必须保持有限枚举。
- route label 使用 `c.FullPath()`；为空时使用稳定 fallback，例如 `__unmatched__`。
- 可选记录稳定低基数应用错误码 label `code`；不得记录原始错误字符串。
- 默认过滤成功健康探针和成功 metrics scrape，或用单独稳定 label 标记 runtime endpoint，使业务面板可排除。
- 在 `user-service/internal/providers/gin.go` 注入 metrics provider，并把 middleware 放入 Gin middleware 链。
- 确认 metrics disabled 时 middleware 零副作用，不注册 collector，不记录指标。
- 增加测试覆盖：
  - 业务请求增加 counter 并写入 duration histogram。
  - 5xx 请求计入错误维度。
  - in-flight gauge 在请求执行中增加，完成后归零。
  - route label 使用 Gin template。
  - 未匹配路由使用稳定 fallback，不使用 raw path。
  - 成功健康探针和 `/metrics` scrape 按策略过滤或单独标记。
  - user-service Gin provider 在 metrics enabled 时接入 middleware，disabled 时保持现有行为。
- 更新相关 README 或架构文档中的 HTTP metrics 接入说明。

不包括：

- 不新增 tracing 逻辑。
- 不新增业务指标或依赖指标。
- 不新增 scheduler、workerpool、datastore、Redis、PostgreSQL 等非 HTTP 指标。
- 不在 label 中放 `user_id`、`session_id`、role ID、permission ID、token、cursor、raw URL、IP、User-Agent、trace ID、span ID、Authorization header、Cookie、SQL、Redis key 或原始错误消息。
- 不改变 request logger 行为和日志字段。
- 不改变 `/livez`、`/readyz`、`/startupz`、`/metrics` 路由语义。
- 不改变 RBAC 授权、JWT 认证、OpenAPI 文档、Ent schema、Atlas migration 或部署清单。
- 不新增 `openspec/` 或 `docs/opsx/`。

## Acceptance Criteria

- metrics enabled 时，业务 HTTP 请求会增加 `http_server_requests_total` 并写入 `http_server_request_duration_seconds` histogram。
- 正在处理中的业务请求会反映在 `http_server_in_flight_requests`，请求结束后 gauge 回落。
- route label 使用 Gin route template，例如 `/api/v1/users/:id`，不使用 raw URL path。
- 未匹配路由使用稳定 fallback label，不产生无限 path label。
- 健康探针和 metrics scrape 的成功请求过滤或单独标记行为有测试覆盖。
- 指标 label 不包含用户、会话、token、cursor、IP、User-Agent、raw URL 或其他高基数/敏感值。
- request logger 行为不变。
- 实现后 `make test-common` 和 `make test-user-service` 通过。
