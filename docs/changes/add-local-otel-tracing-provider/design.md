# Design

## Overview

本变更把 `common/runtime/observability/tracing` 从边界 README 推进为真实 runtime primitive。

```text
common/runtime/config.TracingConfig
  -> common/runtime/observability/tracing.Options
  -> sdktrace.TracerProvider
  -> parent-based sampler
  -> resource attributes
  -> W3C TraceContext + Baggage propagator
  -> Shutdown(ctx)
```

第一阶段只要求 `exporter: none` 可用。该模式仍创建 SDK `TracerProvider`，因此 `tracer.Start(ctx, name)` 会生成标准 OTel trace ID 和 span ID；区别是 provider 不安装 span processor 和 exporter，所以 span 不会离开进程，也不会出现在 Jaeger、Tempo 或 Collector 中。

## Current State

已有基础：

- `common/runtime/config.TracingConfig` 已包含 `enabled`、`sample_ratio`、`exporter`、`otlp_endpoint` 和 `insecure`。
- 配置校验已允许 `exporter: none` 和 `exporter: otlp`。
- `user-service/configs/config.yaml` 本地默认 `observability.tracing.enabled: true`、`sample_ratio: 1.0`、`exporter: none`。
- `common/runtime/observability/tracing/README.md` 只声明边界，没有 Go 实现。
- `docs/ARCHITECTURE.md` 当前仍写明不实现 OpenTelemetry tracer provider，需要随本变更更新为“已支持本地 none exporter provider，尚未接入 middleware/exporter”。

约束：

- `common/runtime/observability/tracing` 必须保持跨服务、无业务语义。
- 不导入 Gin、Ent、Redis、SQL、user-service feature 包或服务侧 DTO。
- 注释使用中文；日志消息如果新增则必须是英文。本变更首选不输出日志。
- 不新增 `openspec/` 或 `docs/opsx/`。

## Package Shape

建议文件：

```text
common/runtime/observability/tracing/
  README.md
  provider.go
  fx.go
  provider_test.go
```

`provider.go` 承载纯构造逻辑和 shutdown wrapper。

`fx.go` 可选承载 Fx provider 适配。若实现后发现当前没有服务侧消费，可保留窄 provider 函数而不把它接入 `common/runtime/config` Fx module 或 user-service `AppModule`，避免配置存在即启动 tracing runtime 的行为漂移。

## Public API

建议公开 API：

```go
type Options struct {
    Config      config.TracingConfig
    ServiceName string
    Environment string
    Version     string
    InstanceID  string
}

type Provider struct {
    TracerProvider *sdktrace.TracerProvider
    TextMapPropagator propagation.TextMapPropagator
}

func NewProvider(ctx context.Context, opts Options) (*Provider, error)

func (p *Provider) Shutdown(ctx context.Context) error
```

说明：

- `Options.Config` 复用现有配置契约。
- `ServiceName` 来自 `config.AppConfig.Name`。
- `Environment` 来自 `config.AppConfig.Environment`。
- `Version` 和 `InstanceID` 第一阶段可为空；非空时写入 resource。
- `Provider.TracerProvider` 暴露 SDK provider，便于测试和后续服务侧注入。
- `Provider.TextMapPropagator` 暴露组合 propagator，便于后续 middleware 或 external client instrumentation 复用。
- `Shutdown` 包装 SDK provider shutdown，便于 Fx lifecycle 调用。

可选便利方法：

```go
func (p *Provider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer
```

该方法只转发到 `p.TracerProvider.Tracer`，不引入业务命名规则。

## Exporter Behavior

`exporter: none`：

- 不创建 OTLP exporter。
- 不设置 batch span processor 或 simple span processor。
- 创建 SDK provider，并设置 sampler、resource 和 ID generator 默认行为。
- span 会在进程内正常创建；结束后不会导出。

`exporter: otlp`：

- 第一阶段可以返回明确错误，例如 `otlp tracing exporter is not implemented`。
- 也可以保留内部 switch 分支但暂不开放实现。
- 不要 silently fallback 到 `none`，避免生产环境以为 span 已导出。

是否启用：

- 如果 `Config.Enabled == false`，建议仍返回一个 provider，但 sampler 可使用 `NeverSample`；或者返回 no-op provider。
- 本变更目标是本地 provider 基础能力，优先选择“创建 SDK provider + sampler 决定记录与否”，这样调用方 API 稳定。
- 文档需要说明 `enabled` 是是否启用 tracing runtime；如果实现选择 disabled 返回 no-op，则测试覆盖该语义。

推荐实现：

```go
sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))
if !opts.Config.Enabled {
    sampler = sdktrace.NeverSample()
}
```

这样 disabled 时仍能安全注入 provider，但不会记录 sampled span。未采样 span context 仍可以包含有效 trace ID/span ID；测试要避免把 sampled 与 ID 有效性混为一谈。

## Sampling

采样率来源：

- `observability.tracing.sample_ratio`
- 配置校验已经保证范围 `[0.0, 1.0]`。
- 构造层仍应 defensively clamp 或拒绝非法值，避免调用方绕过配置校验直接使用 package。

采样器：

```go
sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
```

预期：

- root span 按 ratio 采样。
- 远端或本地 sampled parent 的子 span 继承 sampled。
- 远端或本地 non-sampled parent 的子 span 继承 non-sampled。

测试策略：

- ratio `1.0` 创建 root span，断言 `SpanContext().IsSampled()` 为 true。
- ratio `0.0` 创建 root span，断言 `SpanContext().IsSampled()` 为 false，同时 trace ID 和 span ID 仍有效。
- parent sampled context + ratio `0.0` 创建 child span，断言 child sampled 仍为 true，用于证明 parent-based 行为。

## Resource Attributes

使用 `go.opentelemetry.io/otel/sdk/resource` 和 semantic conventions。

必填：

- `service.name`
- `deployment.environment`

可选：

- `service.version`
- `service.instance.id`

建议：

```go
attrs := []attribute.KeyValue{
    semconv.ServiceName(opts.ServiceName),
    semconv.DeploymentEnvironmentName(opts.Environment),
}
if opts.Version != "" {
    attrs = append(attrs, semconv.ServiceVersion(opts.Version))
}
if opts.InstanceID != "" {
    attrs = append(attrs, semconv.ServiceInstanceID(opts.InstanceID))
}
```

具体 semantic convention 函数名需要按仓库实际引入的 OTel 版本确认；如果新版本只提供 attribute key 常量，使用对应 key 构造 attribute。

构造层应校验 `ServiceName` 和 `Environment` 非空，返回不含敏感值的错误消息。`Version` 和 `InstanceID` 可为空。

## Propagation

Provider 应提供标准 W3C propagator：

```go
propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},
)
```

是否设置 OpenTelemetry global：

- 构造函数默认不应隐式调用 `otel.SetTracerProvider` 或 `otel.SetTextMapPropagator`，避免单元测试和多服务进程中产生全局副作用。
- 可提供显式方法或 Fx provider 选项做全局安装，例如 `InstallGlobal()`，由服务侧在接入时主动调用。
- 第一阶段若没有调用方需要 global，先只返回 provider 和 propagator。

测试不依赖 global state，优先直接使用返回的 provider 和 propagator。

## Fx Lifecycle

可提供窄 Fx provider：

```go
type FxParams struct {
    fx.In

    Lifecycle fx.Lifecycle
    Config    config.Config
}

func NewFxProvider(params FxParams) (*Provider, error)
```

行为：

- 从 `params.Config.App.Name`、`params.Config.App.Environment` 和 `params.Config.Observability.Tracing` 构造 provider。
- 注册 `OnStop` 调用 `provider.Shutdown(ctx)`。
- 不在本变更中接入 `common/runtime/config.Module` 或 user-service `AppModule`，除非 implementation 阶段确认已有合适的 common runtime module 需要消费。

如果引入 Fx provider 会造成未使用 API 或依赖膨胀，可先只提供 `RegisterLifecycle(lc fx.Lifecycle, provider *Provider)` 之类的小函数，或者延后到实际服务 wiring 变更。

## Dependency Plan

需要在 `common/go.mod` 新增 OpenTelemetry 依赖：

- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/sdk`
- `go.opentelemetry.io/otel/trace`

具体版本通过 `go get` 选择当前可用稳定版本，并让 `go mod tidy` 更新 `common/go.sum`。如果未来实现 OTLP exporter，再新增 `go.opentelemetry.io/otel/exporters/otlp/otlptrace/...`；本变更不应为了 `exporter: none` 提前引入 exporter 依赖。

## Documentation Updates

`common/runtime/observability/tracing/README.md`：

- 从“当前没有真实 runtime primitive”更新为“当前支持本地 SDK provider 和 `exporter: none`”。
- 明确可放置 provider、sampler、resource、propagator 和 Fx lifecycle wiring。
- 明确仍禁止业务 span 名称、业务 attribute、Gin controller、Ent、Redis、SQL、部署清单和 tracing backend 配置。
- 明确 `exporter: none` 不提供可视化。

`docs/ARCHITECTURE.md`：

- 更新 Common Organization 中 observability tracing 的当前状态。
- 说明 `common/runtime/observability/tracing` 拥有本地 OTel SDK provider、parent-based sampler、resource attributes、W3C propagator 和 shutdown。
- 明确尚未接入 Gin middleware、logger、外部调用 instrumentation 或 OTLP exporter。

`docs/DEVELOPMENT.md`：

- 更新配置章节：`exporter: none` 在本地会创建 SDK provider，但不导出 span。
- 说明没有 Collector 时不会有 trace UI。
- 保留环境变量覆盖说明。

## Testing Strategy

`common/runtime/observability/tracing/provider_test.go`：

- `TestNewProviderWithNoneExporterCreatesSpanContext`
  - 使用 enabled true、sample ratio 1、exporter none。
  - 创建 tracer 和 span。
  - 断言 trace ID 与 span ID valid。
  - 断言 span sampled。
- `TestNewProviderWithZeroSampleRatioKeepsValidIDsButNotSampled`
  - sample ratio 0。
  - 创建 root span。
  - 断言 IDs valid 且 not sampled。
- `TestParentBasedSamplerHonorsSampledParent`
  - 用 ratio 0 provider。
  - 构造 sampled parent span context。
  - 创建 child span。
  - 断言 child sampled。
- `TestProviderResourceAttributes`
  - 读取 `provider.TracerProvider.Resource().Attributes()`。
  - 断言 service name、deployment environment，以及可选 version/instance ID。
- `TestProviderShutdown`
  - 调用 shutdown 成功。
  - 可重复 shutdown 不 panic；若 SDK 返回错误，包装行为应稳定。
- `TestNewProviderRejectsUnsupportedExporter`
  - exporter otlp 第一阶段返回未实现错误。
  - 不泄漏 endpoint。
- `TestNewProviderRejectsMissingServiceIdentity`
  - service name 或 environment 为空时报错。

验证命令：

```bash
cd common && go test ./runtime/observability/tracing
cd common && go test ./...
```

如果更新文档和 go.mod 后范围仍较小，可额外运行：

```bash
make test-common
make architecture-lint
```

## Risks And Mitigation

风险：开发者以为 `exporter: none` 可以在 UI 中看到 trace。

缓解：文档、配置注释和 README 明确该模式只生成标准上下文，不导出 span。

风险：构造函数修改 OpenTelemetry global state 导致测试互相污染。

缓解：默认返回 provider 和 propagator，不隐式安装 global；需要 global 时后续由服务 wiring 显式调用。

风险：OTLP exporter 配置已经存在但本变更不实现 exporter。

缓解：`exporter: otlp` 返回明确未实现错误，不静默降级；后续单独变更接入 exporter。

风险：引入 OTel 依赖范围过大。

缓解：第一阶段只依赖 OTel API 和 SDK，不引入 OTLP exporter 包或 backend 客户端。

风险：disabled tracing 语义与 ID 生成需求冲突。

缓解：disabled 使用 `NeverSample` SDK provider，调用方仍可安全创建 span context；文档说明 disabled 不记录 sampled span。
