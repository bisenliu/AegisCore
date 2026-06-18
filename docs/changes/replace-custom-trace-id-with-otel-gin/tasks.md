# Tasks

## Preparation

- [ ] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`。
- [ ] 确认本 change 目录使用 `docs/changes/replace-custom-trace-id-with-otel-gin/`，不新增 `openspec/` 或 `docs/opsx/`。
- [ ] 梳理当前 `common/http/middleware/trace_id.go`、`request_logger.go`、`recovery.go` 和 `cors.go` 的 trace-id 依赖。
- [ ] 梳理 `user-service/internal/providers/gin.go`、`providers/fx.go` 和现有 tracing provider Fx 构造方式。
- [ ] 用 `rg -n "X-Trace-ID|HeaderTraceID|TraceID\\(|traceparent|tracestate"` 记录需要清理的生产代码、测试和文档位置。
- [ ] 确认本变更不实现 OTLP exporter、不新增响应 trace header、不接入 Redis/PostgreSQL/Ent/外部 client tracing。

## Dependencies

- [ ] 在 `user-service/` 中引入 `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`。
- [ ] 保持 OpenTelemetry API/SDK 版本与 `common` 当前依赖兼容。
- [ ] 运行 `cd user-service && go mod tidy` 更新 `user-service/go.mod` 和 `user-service/go.sum`。

## Tracing Provider Wiring

- [ ] 在 `user-service/internal/providers/fx.go` 中提供 `common/runtime/observability/tracing.NewFxProvider` 或等价服务侧 provider。
- [ ] 在 `user-service/internal/providers/gin.go` 的 `GinParams` 中注入 `*tracing.Provider`。
- [ ] `NewGinEngine` 在 tracing provider 缺失时返回明确错误，不回退到自定义 trace-id。
- [ ] 确认 Fx shutdown 会调用 tracing provider `Shutdown(ctx)`。
- [ ] 不在 feature module、controller、application 或 infrastructure adapter 中初始化 tracing provider。

## Gin Middleware

- [ ] 从 `NewGinEngine` middleware 链中移除 `commonmw.TraceID()`。
- [ ] 接入 `otelgin.Middleware`。
- [ ] `otelgin.Middleware` service name 使用 `params.Config.App.Name`。
- [ ] 显式传入 `params.Tracing.TracerProvider()`。
- [ ] 显式传入 `params.Tracing.TextMapPropagator()`，或在版本限制下显式安装 OTel global propagator 并在测试中恢复。
- [ ] 配置 health probe filter，使 `/livez`、`/readyz`、`/startupz` 不创建业务 trace span。
- [ ] 实现稳定 span name formatter。
- [ ] 已匹配路由 span name 使用 `METHOD + " " + c.FullPath()`。
- [ ] 未匹配路由 span name 降级为 `METHOD + " " + c.Request.URL.Path`。
- [ ] 保持 recovery、request logger 和 CORS 的既有行为，除 trace-id 契约删除外不改变业务语义。

## Logger And Common Middleware Cleanup

- [ ] 调整 `common/runtime/logger.TraceIDFromContext` 或新增 helper，使其可从 OTel span context 读取有效 trace ID。
- [ ] 保留 `logger.WithTraceID` 对非 HTTP 测试或手动 context 的兼容能力，除非确认所有调用可迁移。
- [ ] 调整 `common/http/middleware/request_logger.go`，不再依赖 `TraceID()` 预先写入 context。
- [ ] 调整 `common/http/middleware/recovery.go`，不再依赖 `TraceID()` 预先写入 context。
- [ ] 删除或迁移 `common/http/middleware/trace_id.go` 中仍被其他 middleware 需要的常量，例如 `ContextKeyLogger`。
- [ ] 删除 `common/http/middleware/trace_id.go`，或将其从公开使用路径中废弃并确保用户服务不再调用。
- [ ] 删除 `common/http/middleware/trace_id_test.go` 中 `X-Trace-ID` 传播、生成、替换和响应头断言。
- [ ] 从 `common/http/middleware/cors.go` 默认 allowed/exposed headers 中移除 `HeaderTraceID`。
- [ ] 更新 CORS 测试，不再断言默认或显式暴露 `X-Trace-ID`。

## User Service Tests

- [ ] 更新 `user-service/internal/providers/routes_test.go`，构造 `NewGinEngine` 时提供 tracing provider。
- [ ] 删除 route tests 中所有 `X-Trace-ID` 请求头设置。
- [ ] 删除 route tests 中所有 `X-Trace-ID` 响应头断言。
- [ ] 将固定日志 `trace_id == "trace-auth-test"` 等断言替换为 OTel trace ID 有效性断言，或只保留 method/path/status/user_id/latency/client_ip 断言。
- [ ] 增加 handler 内 OTel span context 有效性测试。
- [ ] 增加带 `traceparent` 请求会延续父级 trace ID 的测试。
- [ ] 增加无 `traceparent` 请求会创建新 trace ID 和 span ID 的测试。
- [ ] 增加健康探针 filter 测试，确认成功 probe 不产生业务 trace span。
- [ ] 增加或调整 span name 测试，覆盖 route template 和 fallback path。
- [ ] 保留 auth、RBAC、panic recovery envelope、健康探针日志跳过和业务路由行为断言。

## E2E Tests

- [ ] 更新 `user-service/tests/e2e/harness_test.go`，不再设置 `commonmw.HeaderTraceID`。
- [ ] 删除 e2e harness 对 `X-Trace-ID` 响应头的统一断言。
- [ ] 如 e2e 仍需要 tracing 覆盖，改用 `traceparent` 请求头或 handler/test span recorder 验证 OTel context。
- [ ] 确认 e2e 流程的认证、刷新、登出、RBAC 和错误 envelope 断言保持不变。

## Documentation

- [ ] 更新 `docs/ARCHITECTURE.md` 的 HTTP request flow middleware 描述。
- [ ] 更新 `docs/ARCHITECTURE.md` 的 observability 当前状态，说明用户服务已接入 OTel Gin middleware，但仍不实现 OTLP exporter 或外部 client tracing。
- [ ] 删除 `docs/ARCHITECTURE.md` 中 “HTTP trace header 为 `X-Trace-ID`” 当前契约，改为 W3C Trace Context。
- [ ] 更新 `docs/DEVELOPMENT.md` 的 observability 说明，删除“不接入 Gin tracing middleware”的旧状态。
- [ ] 更新 `docs/DEVELOPMENT.md` 中 HTTP trace 调试说明，使用 `traceparent` / `tracestate`。
- [ ] 更新仍作为当前行为说明的 change 文档，例如 `docs/changes/add-user-service-http-flow-integration-tests/design.md` 中的响应 trace-id 断言。
- [ ] 如 `AGENTS.md` 中当前规则仍要求 `X-Trace-ID`，同步更新为 W3C Trace Context 和 OTel span context 来源。

## Verification

- [ ] 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [ ] 运行 common 受影响测试：

```bash
cd common && go test ./http/middleware ./runtime/logger
```

- [ ] 运行 user-service provider 和 e2e 测试：

```bash
cd user-service && go test ./internal/providers ./tests/e2e
```

- [ ] 运行用户服务测试入口：

```bash
make test-user-service
```

- [ ] 运行架构边界检查：

```bash
make architecture-lint
```

- [ ] 扫描确认当前生产代码和测试不再依赖 `X-Trace-ID`：

```bash
rg -n "X-Trace-ID|HeaderTraceID|TraceID\\(" common user-service docs AGENTS.md
```

- [ ] 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [ ] 检查变更范围：

```bash
git diff -- common/http/middleware common/runtime/logger user-service/internal/providers user-service/tests/e2e docs/ARCHITECTURE.md docs/DEVELOPMENT.md AGENTS.md docs/changes/replace-custom-trace-id-with-otel-gin
```

## Guardrails

- [ ] 不新增 `openspec/` 或 `docs/opsx/`。
- [ ] 不实现 OTLP exporter。
- [ ] 不新增 `Trace-Id`、`X-Trace-ID` 或其他响应头替代品。
- [ ] 不保留 `X-Trace-ID` 请求头、响应头或兼容测试契约。
- [ ] 不接入 Redis、PostgreSQL、Ent 或外部 HTTP/gRPC/events client tracing。
- [ ] 不新增 metrics exporter、`/metrics` 路由、dashboard 或告警。
- [ ] 不改变健康探针、CORS、auth、RBAC 授权和业务路由行为。
- [ ] 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [ ] 不在日志、错误消息、配置样例或文档中写入 password、token、Authorization header、Cookie、真实 OTLP endpoint 或其他敏感凭据。
