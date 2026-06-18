# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`。
- [x] 确认本 change 目录使用 `docs/changes/add-local-otel-tracing-provider/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 梳理 `common/runtime/config.TracingConfig` 当前字段、校验规则和本地默认配置。
- [x] 梳理 `common/runtime/observability/tracing/README.md` 当前边界说明。
- [x] 确认本变更不接入 Gin middleware、logger、Redis、PostgreSQL、Ent、外部 HTTP/gRPC/events instrumentation 或 tracing 后端。

## Dependencies

- [x] 在 `common/` 中引入 OpenTelemetry API 和 SDK 依赖。
- [x] 只为 `exporter: none` provider 引入必要依赖；不要提前引入 OTLP exporter 包。
- [x] 运行 `go mod tidy` 更新 `common/go.mod` 和 `common/go.sum`。

## Tracing Provider

- [x] 在 `common/runtime/observability/tracing` 新增 provider 实现文件。
- [x] 定义 `Options`，包含 `config.TracingConfig`、`ServiceName`、`Environment`、可选 `Version` 和可选 `InstanceID`。
- [x] 定义 `Provider`，持有 SDK `TracerProvider` 和 W3C text map propagator。
- [x] 实现 `NewProvider(ctx, opts)`。
- [x] 校验 `ServiceName` 非空。
- [x] 校验 `Environment` 非空。
- [x] 构造层 defensively 校验或 clamp `SampleRatio` 位于 `0.0` 到 `1.0`。
- [x] `exporter: none` 时不创建 OTLP exporter、不连接 Collector、不安装 span exporter。
- [x] `exporter: otlp` 第一阶段返回明确未实现错误，且错误消息不回显 endpoint。
- [x] 未知 exporter 返回明确错误。
- [x] 使用 `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))`。
- [x] 当 tracing disabled 时使用 `sdktrace.NeverSample()` 或等价 no-recording sampler，并保持 provider API 可安全使用。
- [x] 创建 OpenTelemetry resource，至少写入 `service.name` 和 `deployment.environment`。
- [x] 当 `Version` 非空时写入 `service.version`。
- [x] 当 `InstanceID` 非空时写入 `service.instance.id`。
- [x] 创建 `TraceContext` + `Baggage` composite propagator。
- [x] 默认不隐式调用 `otel.SetTracerProvider` 或 `otel.SetTextMapPropagator`。
- [x] 实现 `Provider.Shutdown(ctx)`。
- [x] 如有必要，提供 `Provider.Tracer(name, opts...)` 转发 helper。
- [x] 保持 package 不依赖 Gin、Ent、Redis、SQL、user-service 或 feature 包。

## Fx Lifecycle

- [x] 评估是否新增窄 Fx provider 或 lifecycle helper。
- [x] 若新增 Fx provider，从 `config.Config.App` 和 `config.Config.Observability.Tracing` 构造 provider。
- [x] 若新增 Fx provider，注册 `OnStop` 调用 `Provider.Shutdown(ctx)`。
- [x] 不在本变更中自动接入 user-service `AppModule`，除非已有明确 runtime module 消费点且不会引入 middleware/exporter 行为。

## Tests

- [x] 新增 `common/runtime/observability/tracing/provider_test.go`。
- [x] 覆盖 `exporter: none` provider 可创建和关闭。
- [x] 覆盖使用 provider 创建 span 时 trace ID 和 span ID 有效。
- [x] 覆盖 sample ratio `1.0` 的 root span sampled。
- [x] 覆盖 sample ratio `0.0` 的 root span not sampled，但 trace ID 和 span ID 仍有效。
- [x] 覆盖 parent-based sampler：sample ratio `0.0` 时 sampled parent 的 child span 仍 sampled。
- [x] 覆盖 resource attributes 包含 `service.name` 和 `deployment.environment`。
- [x] 覆盖可选 `service.version` 和 `service.instance.id`。
- [x] 覆盖 shutdown 行为稳定。
- [x] 覆盖 `exporter: otlp` 返回未实现错误且不泄漏 endpoint。
- [x] 覆盖未知 exporter 错误。
- [x] 覆盖缺失 service name 或 environment 错误。
- [x] 如新增 Fx lifecycle helper，补充窄单元测试覆盖 `OnStop` shutdown。

## Documentation

- [x] 更新 `common/runtime/observability/tracing/README.md`，说明当前支持本地 SDK provider 和 `exporter: none`。
- [x] 在 README 中说明 `exporter: none` 不导出 span、不提供 trace 可视化。
- [x] 在 README 中说明可放置 provider、sampler、resource、propagator 和 Fx lifecycle wiring。
- [x] 在 README 中保留禁止业务 span 名称、业务 attribute、Gin controller、Ent、Redis、SQL、部署清单和 tracing backend 配置的边界。
- [x] 更新 `docs/ARCHITECTURE.md`，将 observability tracing 当前状态改为已支持本地 provider，但未接入 middleware/exporter。
- [x] 更新 `docs/DEVELOPMENT.md`，说明本地 `exporter: none` 会创建 SDK provider、生成标准 trace/span ID，但不会导出到 UI。
- [x] 确认文档不重新引入 OpenSpec/OPSX 流程或目录。

## Verification

- [x] 格式化修改过的 Go 文件：

```bash
gofmt -w common/runtime/observability/tracing/*.go
```

- [x] 运行 tracing provider 测试：

```bash
cd common && go test ./runtime/observability/tracing
```

- [x] 运行 common 模块测试：

```bash
cd common && go test ./...
```

- [x] 运行仓库现有 common 测试入口：

```bash
make test-common
```

- [x] 如文档或架构规则变更触发边界检查，运行：

```bash
make architecture-lint
```

- [x] 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 检查变更范围：

```bash
git diff -- docs/changes/add-local-otel-tracing-provider common/runtime/observability/tracing common/go.mod common/go.sum docs/ARCHITECTURE.md docs/DEVELOPMENT.md
```

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/`。
- [x] 不接入 Gin middleware。
- [x] 不改造 logger 或日志字段。
- [x] 不接入 Redis、PostgreSQL、Ent 或外部 HTTP/gRPC/events instrumentation。
- [x] 不部署 Collector、Jaeger、Tempo 或任何 tracing 后端。
- [x] 不新增 metrics exporter、`/metrics` 路由、dashboard 或告警。
- [x] 不保留或新增 `X-Trace-ID` 兼容逻辑。
- [x] 不修改 HTTP API 响应、认证、RBAC、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 不在错误消息、配置样例或文档中写入真实 OTLP endpoint、token、Authorization header、Cookie 或其他敏感凭据。
