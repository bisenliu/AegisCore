## Context

user-service 当前通过 `otelgin` 提供 W3C TraceContext 传播，并由 `common/runtime/logger` 从有效 span context 派生 `trace_id` 和 `span_id`。这能满足已接入 tracing 的内部排障，但 HTTP 调用方和上游网关没有一个稳定、可见、可回传的请求标识；当 tracing 被过滤、未导出或调用方只支持传统 request ID header 时，日志关联能力不足。

该变更属于 `runtime-observability`，实现位置应保持在 `common/http/middleware` 和 user-service Gin provider 组装层。它不属于 auth/user/permission/role feature，也不应进入 `user-service/internal/shared`，因为请求 ID 是无业务语义的 HTTP runtime primitive。

受影响路径：

- `common/http/middleware/`: 新增 request ID middleware、context helper、日志字段扩展和单元测试。
- `user-service/internal/providers/gin.go`: 安装 middleware，保证 user-service 全局生效。
- `user-service/internal/providers/gin_test.go`: 验证服务级 middleware 链生成、透传并回写 header。
- `openspec/changes/add-request-id-middleware/specs/runtime-observability/spec.md`: 定义本次规格 delta。

不影响数据库 migration、OpenAPI 生成物、部署清单、Prometheus/Grafana 观测资产或 RBAC policy sync。

## Goals / Non-Goals

**Goals:**

- 对每个 HTTP 请求提供稳定 `X-Request-ID`：入站存在时透传，缺失时生成。
- 将最终请求 ID 写入响应头，方便客户端和网关在故障反馈中携带该值。
- 将请求 ID 写入 request context，并由 access log 使用 `request_id` 字段输出。
- 保持 request ID 与 `trace_id` / `span_id` 并存，互不替代。
- 明确 request ID 不得进入 metrics label，避免高基数观测风险。

**Non-Goals:**

- 不改变 `traceparent`、`tracestate` 或 OpenTelemetry span 的传播语义。
- 不把 request ID 放入 response envelope body 或 OpenAPI 契约。
- 不新增外部依赖、不改变数据库 schema、不生成 Atlas migration。
- 不把 request ID 作为认证、授权、限流或幂等键使用。

## Decisions

1. 在 `common/http/middleware` 实现共享 Gin middleware。

   这样保持请求标识能力为跨服务、无业务语义的 HTTP primitive，后续其他服务可复用。备选方案是在 `user-service/internal/providers` 内联实现，但会把通用观测逻辑留在单服务装配层，难以被共享日志中间件直接消费。

2. 使用 `X-Request-ID` 作为入站和出站 header，使用 `request_id` 作为日志字段。

   `X-Request-ID` 是网关和客户端常见约定，`request_id` 符合仓库日志字段稳定英文 `snake_case` 规则。备选方案是使用 `Correlation-ID` 或 `X-Correlation-ID`，但当前目标是补齐最通用的 request id 语义，避免同时支持多 header 带来的优先级歧义。

3. 缺失 request ID 时使用 `common/runtime/id.NewUUIDString()` 生成 UUID 字符串。

   该 helper 已是跨服务默认 ID primitive，当前基于 UUID v7，不需要新增依赖。备选方案是直接调用 `github.com/google/uuid` 或生成短随机字符串；前者绕过已有 primitive，后者更容易出现格式和碰撞语义争议。

4. 对入站 request ID 做基础清洗和长度限制。

   middleware 只接受 trim 后非空、长度不超过固定上限且不包含控制字符的值；不合法时生成新 ID，避免 header 注入、日志污染和异常超长字段。备选方案是原样透传所有入站值，但这会把不可信 header 直接写入日志和响应头。

5. 在 Gin middleware 链中将 request ID 放在 request logger 之前。

   这样 `RequestLogger` 在 `c.Next()` 之后记录 access log 时可以从 request context 读取 `request_id`。它可以位于 `otelgin` 之后或之前；服务级实现中放在 tracing 后、metrics/recovery/logging 前，保持 tracing 初始化先发生，同时保证后续错误与日志路径都能读取 request ID。

## Risks / Trade-offs

- [风险] 上游发送恶意或超长 `X-Request-ID` 造成日志污染。→ Mitigation: trim 后校验非空、长度上限和控制字符，不合法则丢弃并生成新值。
- [风险] request ID 和 trace ID 并存导致排障字段选择混乱。→ Mitigation: 规格和实现明确两者职责：request ID 面向客户端和网关回传，trace/span 面向 OTel 链路；日志同时记录可用字段。
- [风险] 将 request ID 放入 metrics label 会造成高基数。→ Mitigation: 规格明确禁止，implementation 只扩展日志字段，不修改 metrics middleware。
- [风险] 生成 request ID 失败。→ Mitigation: `common/runtime/id.NewUUIDString()` 理论上极少失败；middleware 可在失败时继续请求并省略或使用保守 fallback，避免 request ID 生成失败阻断业务请求。实现时优先保持请求可用性并覆盖测试。

## Migration Plan

1. 新增 common middleware 和测试。
2. 在 user-service Gin engine 全局安装 middleware。
3. 运行 `go test ./...` 于 `common` 和 `user-service` 相关包，至少覆盖 `common/http/middleware` 与 `user-service/internal/providers`。
4. 运行 `make user-service-architecture-lint` 验证边界。

回滚方式是移除 Gin engine 中的 middleware 安装，并删除新增 common middleware 或停止引用；该变更不包含状态迁移、数据库迁移或部署资产变更。

## Open Questions

无。
