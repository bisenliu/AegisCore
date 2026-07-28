## Context

Gin 入站请求当前在不同观测入口中混用 `c.FullPath()` 和 `c.Request.URL.Path`。access log 在 `FullPath()` 为空时回退 raw path，binding/validator 失败日志直接写 raw path，user-service runtime endpoint skip 与 OTel Gin filter 使用 `request.URL.Path` 做判断。HTTP metrics route label 已使用 route template 和固定 fallback，但 fallback 仍通过 option 表达，语义不够统一。

这会让动态资源 ID、tenant、cursor、UUID 等进入结构化日志字段或观测过滤逻辑，也会让 path 字段在不同场景下语义不一致。本 change 需要跨 `common` 和 `user-service` 收敛 Gin 入站观测路径处理，影响日志、metrics、tracing、授权 object 和 runtime endpoint skip 判断，但不改变外部 HTTP API、数据库 schema 或 OpenAPI 文档。

## Goals / Non-Goals

**Goals:**

- 为 Gin 入站请求提供唯一低基数 route path helper：匹配路由返回 Gin route template，未匹配路由返回 `__unmatched__`。
- 统一 access log、auth failure log、binding failure log、HTTP metrics route label、trace span name、授权 object 和 runtime endpoint skip 判断的路径语义。
- 删除生产 Gin 入站观测链路中对 `c.Request.URL.Path` 或 `request.URL.Path` 的依赖。
- 固定未匹配路由观测 fallback，移除或忽略可配置 route fallback 的公共契约。
- 用测试证明动态 URL path 不进入日志、metrics label 或 span name。

**Non-Goals:**

- 不改变业务 HTTP endpoint、请求响应 envelope、OpenAPI 生成物或权限目录模型。
- 不修改 Ent schema、Atlas migration、PostgreSQL 或 Redis 数据契约。
- 不新增 tracing collector、span processor、dashboard 或 alert 过滤能力。
- 不在日志中新增 `raw_path`、`url_path` 等兼容字段，不做双写迁移。

## Decisions

1. 在 `common/http/route` 提供低基数 helper。

   选择：新增中性包 `common/http/route`，提供 `TemplateOrUnmatched(c *gin.Context) string` 和稳定常量 `Unmatched = "__unmatched__"`。

   理由：`common/http/binding` 不应反向依赖 `common/http/middleware`；route helper 是跨 binding、middleware、Casbin 和服务 provider 共享的 HTTP primitive，不承载 user-service 业务语义。

   备选：把 helper 放在 `common/http/middleware` 并由 binding 导入。拒绝原因是会扩大 binding 到 middleware 的依赖方向，边界不够清晰。

2. 固定 fallback 为 `__unmatched__`。

   选择：所有未匹配 Gin 入站观测路径统一使用 `__unmatched__`，trace span name 使用 `METHOD __unmatched__`。

   理由：固定值可防止未匹配路由产生高基数；同一 fallback 也让日志、metrics 和 tracing 可一致验证。

   备选：保留 `route not found` 作为 tracing 专用 fallback。拒绝原因是同一状态在不同信号中出现不同值，增加查询和测试复杂度。

3. 停止在 OTel Gin filter 阶段按 raw path 过滤 runtime endpoint。

   选择：移除 `otelgin.WithFilter(traceBusinessRequest(...))` 中基于 `request.URL.Path` 的过滤，保留基于 route template 的 span name 逻辑。

   理由：OTel Gin filter 只接收 `*http.Request`，无法安全获得 `c.FullPath()`；继续使用 raw path 会违背本次安全和低基数目标。

   备选：保留 raw path filter 仅用于 `/metrics`、`/livez` 等静态路径。拒绝原因是会继续允许 Gin 入站观测链路在路由匹配前读取 raw URL path，形成例外和后续误用入口。

4. 将 runtime endpoint 判断函数改为 route 语义。

   选择：把 `IsMetricsPath`、`IsLowNoiseRuntimePath` 替换为 `IsMetricsRoute`、`IsLowNoiseRuntimeRoute`，调用方传入 `c.FullPath()` 或 route helper 结果。

   理由：函数名表达输入必须是 route template，不再鼓励传入原始 URL path。metrics path 配置仍只用于注册和静态 route template 比较。

   备选：保留旧函数名但修改内部行为。拒绝原因是命名仍会诱导调用方传 raw path。

5. 不为兼容保留 `HTTPMetricsOptions.RouteFallback` 可变语义。

   选择：删除该字段，或在实现中停止对外暴露任意 fallback 选择；未匹配路由统一由 route helper 决定。

   理由：这是公共 middleware 配置契约的有意不兼容收敛，避免外部调用方配置高基数 fallback。

   备选：保留字段但强制忽略。拒绝原因是隐藏行为容易造成误解，除非短期编译迁移必须，否则应直接移除。

## Risks / Trade-offs

- [Risk] 移除 OTel Gin raw path filter 后，`/metrics` 和健康检查可能重新产生 server span。→ Mitigation：接受这是安全优先的不兼容取舍；如需降噪，在 collector 侧基于稳定 route/span name 过滤。
- [Risk] 删除 `HTTPMetricsOptions.RouteFallback` 会让内部或未来跨服务调用方编译失败。→ Mitigation：通过编译错误暴露迁移点，并统一改用固定 `__unmatched__`。
- [Risk] `c.FullPath()` 在 Gin middleware 链中何时可用取决于路由匹配阶段。→ Mitigation：只依赖 Gin 已匹配路由返回模板；为空时统一低基数 fallback，不尝试回退 raw path。
- [Risk] runtime endpoint skip 判断改用 route template 后，配置化 metrics path 规范化必须与注册 route 保持一致。→ Mitigation：保留现有 metrics path normalize 逻辑，并补充 `/metrics`、`/livez`、`/readyz`、`/startupz` 成功请求 skip 测试。

## Migration Plan

1. 在 `common/http/route` 增加低基数 helper 和 `__unmatched__` 常量。
2. 将 `common/http/middleware` 日志、metrics 和 Casbin/授权观测字段切换到 helper，移除 raw path fallback 和可配置 route fallback。
3. 将 `common/http/binding` 绑定失败日志切换到 helper。
4. 将 `user-service/internal/router` 的低噪声判断函数改为 route template 语义，并更新 `user-service/internal/providers/gin.go` 调用点。
5. 移除 OTel Gin raw path filter，统一 HTTP server span fallback。
6. 更新或新增单元测试和集成级 middleware 测试，确认动态路径值不出现在日志、metrics label 和 span name 中。

回滚方式：如生产 tracing 噪声明显不可接受，可回滚到上一版本；不要局部恢复 raw path filter。后续降噪应通过 collector 侧稳定 route/span name 规则完成。

## Open Questions

- 无。当前方案明确选择安全和低基数一致性优先于应用内 raw path 早期 tracing 降噪。
