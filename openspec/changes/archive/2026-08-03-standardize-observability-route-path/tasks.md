## 1. 低基数 route helper

- [x] 1.1 在 `common/http/route` 新增 Gin route template helper 和固定 `__unmatched__` 常量，匹配路由返回 `c.FullPath()`，未匹配或 nil context 返回 `__unmatched__`。
- [x] 1.2 为 route helper 增加单元测试，覆盖动态 route template、未匹配路由和 nil context。

## 2. common 观测入口整改

- [x] 2.1 将 `common/http/middleware/log_fields.go` 的 access log 和 auth failure log `path` 字段切换到 route helper，移除 raw URL path fallback。
- [x] 2.2 将 `common/http/binding/validator.go` 的 binding/validation 失败日志 `path` 字段切换到 route helper。
- [x] 2.3 将 `common/http/middleware/metrics.go` 的 route label fallback 固定为 `__unmatched__`，删除或停止暴露 `HTTPMetricsOptions.RouteFallback` 可配置语义。
- [x] 2.4 检查 `common/http/middleware/casbin.go` 等 Gin 入站授权相关代码，确保授权 object 和默认观测字段使用 route template 或 `__unmatched__`，不回退 raw URL path。

## 3. user-service Gin provider 和 router 整改

- [x] 3.1 将 `user-service/internal/router` 中 runtime endpoint 判断函数从 Path 语义改为 Route 语义，例如 `IsMetricsRoute` 和 `IsLowNoiseRuntimeRoute`。
- [x] 3.2 更新 `user-service/internal/providers/gin.go` 的成功 runtime endpoint access log skip、metrics scrape skip 和 request counter/duration skip 调用点，传入 route template 或固定低基数 route helper 结果。
- [x] 3.3 移除 `otelgin.WithFilter(traceBusinessRequest(...))` 及其 raw path filter helper，避免 route match 前基于 `request.URL.Path` 做 tracing 过滤。
- [x] 3.4 将 HTTP server span name fallback 统一为 `METHOD __unmatched__`，确保动态业务路由 span name 使用 `METHOD <route template>`。

## 4. 测试覆盖

- [x] 4.1 更新 access log 测试，覆盖动态路由 `path=/api/v1/users/:user_id`、未匹配路由 `path=__unmatched__`，并断言日志不包含 raw UUID path。
- [x] 4.2 增加或更新 auth failure 和 binding failure 测试，覆盖动态路由失败时记录 route template 而不是 raw path。
- [x] 4.3 更新 HTTP metrics middleware 测试，覆盖未匹配路由 label 为 `__unmatched__`，动态路由 label 为 route template，且不包含 raw UUID path。
- [x] 4.4 更新 user-service runtime endpoint skip 测试，覆盖 `/metrics`、`/livez`、`/readyz`、`/startupz` 成功请求不产生 request counter/duration 或成功 access log。
- [x] 4.5 更新 tracing 相关测试，覆盖动态路由 span name 为 `GET /api/v1/users/:user_id`，未匹配路由 span name 为 `GET __unmatched__`，并确认不再依赖 raw path filter。

## 5. 搜索、规格和验证

- [x] 5.1 运行 `rg '\.Request\.URL\.Path|request\.URL\.Path|URL\.Path' -- '*.go'`，确认生产 Gin 入站观测链路不再命中 raw URL path 使用，仅保留测试或非 Gin 入站协议场景。
- [x] 5.2 运行相关包测试，例如 `go test ./common/http/...` 和 `go test ./user-service/internal/...` 中受影响 package，修复失败。
- [x] 5.3 运行 `make user-service-architecture-lint`，确认 common、user-service 和 OpenSpec 边界符合规则。
- [x] 5.4 暂存本次预期代码、测试、OpenSpec 和文档变更。
- [x] 5.5 运行 `make lint` 并修复失败。
- [x] 5.6 运行 `make verify` 并修复失败，确认未暂存 drift 不阻塞最终校验。
