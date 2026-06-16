# Replace custom trace id with OTel Gin

## What

使用 OpenTelemetry Gin middleware 完全替代当前自定义 `X-Trace-ID` Gin middleware。

本变更将用户服务 HTTP 入站 tracing 从私有请求头契约迁移到标准 W3C Trace Context：

- 在 `user-service/internal/providers/gin.go` 中移除 `commonmw.TraceID()`。
- 接入 `otelgin.Middleware`，使用当前服务名、已配置的 OpenTelemetry tracer provider、健康探针过滤规则和稳定 span name formatter。
- HTTP 入站请求由 OTel 创建 server span，并从标准 `traceparent` / `tracestate` 请求头提取父级上下文。
- span name 优先使用 Gin route template，例如 `GET /api/v1/users/:user_id`，未匹配路由时降级为 URL path。
- 不再读取、写入或回传 `X-Trace-ID`，并删除相关测试契约。
- 更新文档，将 HTTP trace 传播协议说明为 W3C Trace Context。

## Why

当前 `common/http/middleware/trace_id.go` 通过 `X-Trace-ID` 读取或生成自定义 trace id，并把它写入 Gin context、Go context、响应头和日志字段。这套机制只能表达私有关联 ID，不能和 OpenTelemetry trace/span 模型、上游标准传播协议或未来 exporter 自然衔接。

仓库已经具备 `common/runtime/observability/tracing` 的本地 OTel SDK provider，下一步应让 HTTP 入站请求真正进入 OTel tracing 体系。这样可以：

- 使用 W3C `traceparent` / `tracestate` 与网关、上游服务和未来外部 client instrumentation 对齐。
- 由 OTel server span 统一生成有效 trace ID 与 span ID，而不是依赖私有 header。
- 为后续日志 trace/span 关联、OTLP exporter 和跨服务链路打基础。
- 移除 `X-Trace-ID` 响应头兼容契约，避免长期维护两套传播语义。

## Scope

包括：

- 在用户服务 provider 层接入 `common/runtime/observability/tracing.NewFxProvider` 或等价窄 provider，使 `NewGinEngine` 可取得已配置的 tracer provider 和 propagator。
- 在 `user-service/internal/providers/gin.go` 中使用 `otelgin.Middleware` 替换 `commonmw.TraceID()`。
- 为 `otelgin.Middleware` 配置：
  - service name 使用 `config.App.Name`。
  - tracer provider 使用 `common/runtime/observability/tracing.Provider.TracerProvider()`。
  - propagator 使用 `common/runtime/observability/tracing.Provider.TextMapPropagator()`，或通过显式 OTel global wiring 保证 W3C 传播可用。
  - health probe skip/filter，覆盖 `/livez`、`/readyz`、`/startupz`。
  - span name formatter：优先 `METHOD + " " + c.FullPath()`，未匹配时使用 `METHOD + " " + URL.Path`。
- 增加或调整测试，验证 handler 内 `c.Request.Context()` 包含有效 OTel server span context。
- 更新 route/e2e 测试，移除所有 `X-Trace-ID` 请求头、响应头和日志字段值的兼容断言。
- 删除或废弃 `common/http/middleware/trace_id.go`，并删除依赖它的单元测试。
- 调整 `common/http/middleware` 中 recovery/request logger 对 trace id 的依赖，使其不再要求 `TraceID()` 先运行。
- 更新 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和相关 historical change 文档中仍描述 `X-Trace-ID` 当前契约的内容。
- 运行用户服务测试和架构边界检查。

不包括：

- 不实现 OTLP exporter。
- 不新增 `Trace-Id`、`X-Trace-ID` 或其他响应头替代品。
- 不接入 Redis、PostgreSQL、Ent 或外部 HTTP/gRPC/events client tracing。
- 不新增 metrics exporter、`/metrics` 路由、dashboard 或告警。
- 不改变健康探针、CORS、auth、RBAC 授权和业务路由行为。
- 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- Gin 请求进入 handler 时，`c.Request.Context()` 中存在有效 OTel span context。
- 带有效 `traceparent` 的入站请求会把该 trace ID 作为 server span trace ID。
- 未带 `traceparent` 的入站请求会创建新的有效 OTel trace ID 和 span ID。
- 健康探针不产生业务 trace span，或通过 `otelgin` filter 被明确跳过。
- 已匹配业务路由的 server span name 使用 Gin route template，例如 `GET /api/v1/users/:user_id`。
- 未匹配路由的 server span name 降级为 HTTP method 与 URL path。
- 所有 `X-Trace-ID` 请求头、响应头和兼容测试断言已移除。
- `common/http/middleware/trace_id.go` 不再作为 HTTP 中间件被服务使用；若删除，所有引用同步清理。
- HTTP access log 和 panic recovery 不依赖自定义 `TraceID()` 中间件才能运行。
- 文档说明 HTTP trace 传播协议为 W3C Trace Context，而不是 `X-Trace-ID`。
- `make test-user-service` 通过。
- `make architecture-lint` 通过。
