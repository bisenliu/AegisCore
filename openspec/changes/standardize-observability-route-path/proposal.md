## Why

当前 Gin 入站观测链路在部分日志字段和过滤判断中仍会回退使用原始 URL path，未匹配路由或绑定失败时可能把真实 UUID、cursor、tenant、资源 ID 等动态路径参数写入结构化日志字段或观测判断链路，带来高基数和敏感标识泄露风险。

需要把入站 HTTP 观测契约统一为低基数 route template，并为未匹配路由提供固定 fallback，避免日志、metrics、tracing 和授权相关观测字段继续依赖 raw path。

## What Changes

- **BREAKING** 将 Gin 入站观测字段 `path` 的语义统一为 Gin route template，不再表示“可能为原始请求路径”。
- **BREAKING** 未匹配 Gin 路由的入站观测路径统一使用固定值 `__unmatched__`，不再回退到 `c.Request.URL.Path`。
- **BREAKING** HTTP metrics route fallback 固定为低基数 `__unmatched__`；不再允许调用方通过公共 middleware 配置注入任意 fallback 语义。
- **BREAKING** 移除 user-service 在 OTel Gin filter 阶段基于 `request.URL.Path` 的 runtime endpoint 早期 tracing 跳过逻辑，避免路由匹配前使用 raw path 做观测过滤。
- 统一日志、metrics、trace span name、绑定失败日志、认证失败日志、runtime endpoint 跳过判断和授权 object 的 route path helper。
- 将 runtime endpoint 判断函数改为 route template 语义，避免调用方继续传入原始 URL path。
- 补充测试覆盖动态路由、未匹配路由、绑定失败、认证失败、metrics skip、runtime endpoint skip、trace span name 和 raw UUID 禁止输出。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 入站 HTTP 观测路径字段、metrics label、trace span name 和观测过滤必须使用 route template 或固定 `__unmatched__`，禁止默认发出或依赖 raw URL path。

## Impact

- 影响 `common/http/middleware` 的日志字段、HTTP metrics middleware、Casbin/授权中间件观测字段和共享 route path helper。
- 影响 `common/http/binding` 的绑定和校验失败日志字段。
- 影响 `user-service/internal/providers/gin.go` 的 Gin middleware 装配、runtime endpoint 观测 skip 判断和 HTTP server span name。
- 影响 `user-service/internal/router` 中 metrics/health 低噪声判断函数的输入语义和命名。
- 影响 Prometheus HTTP route label fallback、结构化日志 `path` 字段、OpenTelemetry span name 和应用内 tracing 过滤行为。
- 不涉及数据库 schema、Ent 生成代码、OpenAPI 生成物或外部 HTTP API 请求/响应契约。
