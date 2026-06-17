# Design

## Overview

本变更只收敛文档和测试契约，不改变 tracing runtime。

目标状态：

```text
W3C traceparent/tracestate
  -> OTel Gin middleware
  -> request context contains valid OTel server span
  -> common/runtime/logger derives trace_id/span_id
  -> tests assert OTel context or log fields, not X-Trace-ID
```

客户端可以通过标准 W3C Trace Context 传入父级 trace，但不需要依赖用户服务返回任何 trace header。服务端在 `observability.tracing.exporter: none` 时仍使用本地 OTel SDK provider 生成标准 trace/span context，只是不导出 span，不要求 Collector。

## Current State

当前代码已经基本完成运行时迁移：

- `docs/ARCHITECTURE.md` 已说明 HTTP trace 传播使用 `traceparent` / `tracestate`，日志 helper 从 OTel span context 自动追加 `trace_id` 与 `span_id`。
- `docs/DEVELOPMENT.md` 已说明用户服务不读取、回传或兼容 `X-Trace-ID`。
- `common/runtime/logger` 已暴露 `TraceIDField = "trace_id"` 与 `SpanIDField = "span_id"`，并从 OTel span context 派生字段。
- `user-service/internal/providers/gin_test.go` 已覆盖 `traceparent` 传播和无 header 时生成有效 trace/span context。
- `common/http/middleware/logging_test.go` 与 provider 测试已覆盖 OTel `trace_id` / `span_id` 日志字段。

仍需清理的当前说明包括：

- `docs/TESTING.md` 中 middleware 验证仍描述 trace-id 透传、生成、写入 Gin context/Go context/响应头。
- `docs/TESTING.md` 中 logging 验证仍描述 trace-id 字段，而非 OTel `trace_id` / `span_id`。
- `docs/TESTING.md` 中 e2e HTTP flow 仍描述 trace-id 响应头。
- `docs/PRODUCT.md` 中 HTTP 请求流程仍描述经过 trace-id middleware。
- 可能存在 OpenAPI 注解、生成产物或测试 helper 中残留 `X-Trace-ID` / `HeaderTraceID`。

## Documentation Plan

### `docs/ARCHITECTURE.md`

保持当前 OTel 口径，并检查是否需要补充两点：

- 客户端传播使用标准 `traceparent` / `tracestate`，服务端不承诺返回 trace header。
- `exporter: none` 只生成本进程 OTel span context 和日志关联字段，不强制 Collector。

如果现有文本已经覆盖这些内容，只做最小调整，避免重复段落。

### `docs/DEVELOPMENT.md`

保持本地运行和调试说明与当前配置一致：

- `observability.tracing.exporter: none` 不需要 `otel-collector:4317`。
- 本地不会在 trace UI 中看到链路，除非后续单独实现 OTLP exporter 和后端部署。
- 调试请求关联时查看日志中的 `trace_id` 与 `span_id`，或传入标准 `traceparent` 验证传播。
- 不指导开发者查看 `X-Trace-ID` 响应头。

### `docs/TESTING.md`

将测试验证口径改为 OTel：

- Middleware：验证 OTel Gin middleware 创建或传播有效 server span context，panic recovery 输出统一错误，request logging 包含 `trace_id` / `span_id`，CORS 处理 OPTIONS。
- Logging：验证 Zap logger 初始化、分类日志文件、有效 OTel span context 下的 `trace_id` / `span_id` 字段，以及无有效 span context 时不伪造 trace/span 字段。
- E2E HTTP flow：删除 trace-id 响应头覆盖项，改为覆盖真实 Gin engine、OTel tracing middleware、JWT auth、token version validation、统一 response envelope 和关键业务流程。
- 明确 e2e helper 不应设置 `X-Trace-ID`；如果需要传播测试，使用 W3C `traceparent`。

### `docs/PRODUCT.md`

将产品运行流程中的 trace-id middleware 说明改为 OTel tracing middleware：

```text
HTTP 请求经过 OTel tracing、日志、panic recovery 和 CORS 中间件。
```

该文档只表达当前能力，不展开实现细节。

### Historical Change Docs

历史 change 文档不作为长期规则源。默认不重写历史记录。

只有在旧 change 文档仍被当前文档引用为测试说明，或明显以当前行为口吻描述 `X-Trace-ID` 时，做局部修正，并在任务中记录原因。否则允许它们作为历史背景保留旧术语。

## Test Migration Plan

### E2E HTTP Flow

重点扫描：

```bash
rg -n "X-Trace-ID|HeaderTraceID|TraceID\\(|trace-id|trace_id|span_id|traceparent|tracestate" user-service/tests/e2e
```

目标状态：

- 请求 helper 不自动设置 `X-Trace-ID`。
- 响应 helper 不统一断言 `X-Trace-ID`。
- 断言继续覆盖登录、受保护用户 API、强制改密、旧密码拒绝、登出当前设备、refresh session 失效和统一 response envelope。
- 如 e2e 仍需要 tracing 覆盖，优先使用以下之一：
  - 通过测试 handler 或日志 recorder 验证请求 context 中存在有效 OTel span context。
  - 发送合法 `traceparent`，断言日志中的 `trace_id` 等于传入 parent trace ID。
  - 只在 provider / middleware 单元测试覆盖 tracing，e2e 不重复断言 trace 细节。

不新增响应 trace header 断言。

### Provider And Middleware Tests

当前 provider 和 middleware 测试大多已经是 OTel 口径。实现阶段仍要扫描：

```bash
rg -n "X-Trace-ID|HeaderTraceID|WithTraceID|TraceIDFromContext|TraceID\\(" common/http common/runtime/logger user-service/internal/providers
```

如果发现当前测试仍依赖私有 trace-id helper：

- 改为使用 OTel API 构造固定 `trace.TraceID` 与 `trace.SpanID`。
- 使用 `trace.ContextWithSpanContext(ctx, spanContext)` 写入测试 context。
- 断言 `logger.TraceIDField` 和 `logger.SpanIDField`。
- 无有效 span context 的测试断言不出现空字符串 `trace_id` / `span_id`。

### OpenAPI And Runtime Docs

扫描服务侧 OpenAPI 注解与生成产物：

```bash
rg -n "X-Trace-ID|Trace-Id|trace header|trace-id|traceparent|tracestate" user-service/internal user-service/docs docs
```

处理规则：

- 如果当前 OpenAPI 没有 trace header 契约，不新增 `traceparent` 参数。
- 如果存在 `X-Trace-ID` 或响应 trace header 说明，删除或改为非契约性的运行时说明。
- 若源码注解变化导致生成产物变化，运行 `make openapi-generate` 并提交生成结果。
- 如果生成命令没有变化，确认 `git diff -- user-service/docs` 无未提交差异。

## Verification

实现后建议按范围运行：

```bash
rg -n "X-Trace-ID|HeaderTraceID" docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md docs/PRODUCT.md common user-service
```

允许的命中只能是历史 change 文档中明确描述过去行为的内容；当前文档、生产代码和测试不应依赖它。

```bash
rg -n "trace-id 响应头|trace-id 透传|trace-id 生成|TraceID\\(" docs/TESTING.md user-service/tests/e2e
```

这些当前测试说明和 e2e 测试中不应再出现。

运行验证：

```bash
make test-user-service
```

```bash
make test-common
```

```bash
make openapi-generate
```

```bash
make verify
```

如果 `make openapi-generate` 因环境缺少 swagger/swag 相关工具失败，应在实现记录中说明，并至少完成源码和生成产物 diff 检查。

最后确认没有恢复退役工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

该命令应无输出。
