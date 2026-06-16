# Design

## Overview

本变更只建立配置契约，不建立观测 runtime 行为。

```text
common/runtime/config
  -> observability.metrics
  -> observability.tracing

future runtime wiring
  -> metrics route/exporter
  -> tracer provider/exporter
  -> Gin middleware/span/log correlation
```

配置契约先落在 `common/runtime/config`，因为 metrics、tracing、采样率和 exporter 类型是跨服务稳定 runtime 输入，不属于 user-service 某个 feature。用户服务配置样例只提供该契约的一个本地默认实例。

## Current State

当前配置结构：

- `Config` 已包含 `system`、`app`、`http`、`auth`、`ent`、`log`、`redis` 和 `postgres`。
- `Load` 使用 Viper 读取 YAML，并绑定已发现 key 的 `AEGISCORE_` 环境变量覆盖。
- `Validate` 已按 section 聚合错误，并在生产类环境拒绝不安全 JWT secret 和 PostgreSQL `sslmode: disable`。
- `user-service/configs/config.yaml` 当前没有 `observability` 配置段。
- `common/runtime/observability/metrics` 和 `common/runtime/observability/tracing` 当前只有 README 边界说明，没有实际 exporter 或 middleware。

本变更应延续现有配置加载和校验风格，不新增服务侧手写解析逻辑。

## Config Shape

新增根级配置：

```go
type Config struct {
    // ...
    Observability ObservabilityConfig `mapstructure:"observability"`
}

type ObservabilityConfig struct {
    Metrics MetricsConfig `mapstructure:"metrics"`
    Tracing TracingConfig `mapstructure:"tracing"`
}

type MetricsConfig struct {
    Enabled        bool   `mapstructure:"enabled"`
    Path           string `mapstructure:"path"`
    IncludeRuntime bool   `mapstructure:"include_runtime"`
}

type TracingConfig struct {
    Enabled      bool    `mapstructure:"enabled"`
    SampleRatio  float64 `mapstructure:"sample_ratio"`
    Exporter     string  `mapstructure:"exporter"`
    OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
    Insecure     bool    `mapstructure:"insecure"`
}
```

字段语义：

| 字段 | 默认建议 | 说明 |
|---|---:|---|
| `observability.metrics.enabled` | `false` 或服务样例显式值 | 是否允许后续 metrics route/exporter wiring 消费该开关 |
| `observability.metrics.path` | `/metrics` | 未来 HTTP metrics 暴露路径；本变更不注册路由 |
| `observability.metrics.include_runtime` | `true` | 未来是否包含 Go runtime/process 指标 |
| `observability.tracing.enabled` | `true` | 是否启用 tracing 配置入口；第一阶段不等于对外导出 |
| `observability.tracing.sample_ratio` | `1.0` | 采样比例，范围 `[0.0, 1.0]` |
| `observability.tracing.exporter` | `none` | 第一阶段允许 `none` 和 `otlp` |
| `observability.tracing.otlp_endpoint` | `""` | `exporter: otlp` 时必填，示例可用 `localhost:4317` |
| `observability.tracing.insecure` | `false` | OTLP 连接是否允许明文或跳过 TLS 语义，生产类环境禁止 |

`tracing.enabled: true` 且 `tracing.exporter: none` 表示后续 tracer provider 可以在本进程内初始化和采样，但不向外部 Collector 导出。这样本地开发无需部署 OTLP Collector。

## Local Default Config

`user-service/configs/config.yaml` 建议新增：

```yaml
observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: none
    otlp_endpoint: ""
    insecure: false
```

理由：

- metrics path 先固定为常见 `/metrics`，便于后续实现直接消费。
- metrics 默认不开启，避免用户误以为本阶段已经暴露 `/metrics` route。
- tracing 默认开启配置入口，但 exporter 为 `none`，不依赖本地 `otel-collector:4317`。
- `insecure: false` 作为安全默认；本地如果未来使用明文 OTLP，可通过环境变量显式覆盖，但生产类环境应被校验拒绝。

## Validation Rules

在 `Config.Validate` 中追加 `validateObservability`：

```go
func (c Config) Validate() error {
    // ...
    errs = append(errs, c.validateObservability()...)
    // ...
}
```

建议规则：

- `observability.metrics.path`
  - trim 后不能为空。
  - metrics enabled 时必须以 `/` 开头。
  - 可选：拒绝包含空白字符的 path。
- `observability.tracing.sample_ratio`
  - 必须 `>= 0` 且 `<= 1`。
- `observability.tracing.exporter`
  - trim + lowercase 后不能为空。
  - 允许集合第一阶段为 `none`、`otlp`。
  - 保存原始值可不强制规范化；校验时按 lowercase 判断。
- `observability.tracing.otlp_endpoint`
  - exporter 为 `none` 时允许为空。
  - exporter 为 `otlp` 时 trim 后不能为空。
  - 不在错误消息中回显 endpoint 原文，避免未来 endpoint 中携带 token 或用户信息时泄漏。
- 生产类环境：
  - 当 `app.environment` 属于 `prod`、`production`、`staging` 且 exporter 为 `otlp` 时，拒绝 `insecure: true`。
  - 第一阶段不强制 production tracing enabled，也不强制 endpoint 使用特定 scheme，因为当前 endpoint 是后续 exporter 的输入契约。

错误消息示例：

- `observability.metrics.path is required`
- `observability.metrics.path must start with /`
- `observability.tracing.sample_ratio must be between 0 and 1`
- `observability.tracing.exporter must be one of none, otlp`
- `observability.tracing.otlp_endpoint is required when exporter is otlp`
- `observability.tracing.insecure must not be true with otlp exporter in production-like environments`

## Environment Overrides

沿用现有 `AEGISCORE_` 前缀和 key 替换规则：

| YAML key | 环境变量 |
|---|---|
| `observability.metrics.enabled` | `AEGISCORE_OBSERVABILITY_METRICS_ENABLED` |
| `observability.metrics.path` | `AEGISCORE_OBSERVABILITY_METRICS_PATH` |
| `observability.metrics.include_runtime` | `AEGISCORE_OBSERVABILITY_METRICS_INCLUDE_RUNTIME` |
| `observability.tracing.enabled` | `AEGISCORE_OBSERVABILITY_TRACING_ENABLED` |
| `observability.tracing.sample_ratio` | `AEGISCORE_OBSERVABILITY_TRACING_SAMPLE_RATIO` |
| `observability.tracing.exporter` | `AEGISCORE_OBSERVABILITY_TRACING_EXPORTER` |
| `observability.tracing.otlp_endpoint` | `AEGISCORE_OBSERVABILITY_TRACING_OTLP_ENDPOINT` |
| `observability.tracing.insecure` | `AEGISCORE_OBSERVABILITY_TRACING_INSECURE` |

Viper 当前只为已发现 key 绑定环境变量，因此 `user-service/configs/config.yaml` 和测试 fixture 必须包含完整 observability key，才能在不额外改 loader 绑定策略的前提下支持覆盖。

## Sensitive Boundary

当前字段本身不包含 token、Authorization header 或 exporter header map。仍需在文档和校验实现中保持以下边界：

- `otlp_endpoint` 不应包含 token、username/password 或 query 中的凭据。
- 不新增 `headers`、`authorization`、`api_key` 等字段；未来如需 exporter 认证，必须单独设计 Secret 注入方式。
- 校验错误不回显 `otlp_endpoint` 原文。
- 配置样例不写生产地址、真实 collector 地址或组织内部域名。

## Documentation Updates

`docs/ARCHITECTURE.md`：

- 在 `Common Organization` 或 `Infrastructure` 中说明 `common/runtime/config` 拥有 observability 配置契约。
- 明确 metrics/tracing runtime 行为暂未实现，本变更只建立配置结构和校验。
- 明确 `common/runtime/observability/*` 当前只能承载无业务语义的未来 metrics/tracing primitive，不能放 user-service 业务指标、dashboard 或部署清单。

`docs/DEVELOPMENT.md`：

- 在配置章节增加 observability 字段说明。
- 给出环境变量覆盖示例，例如 `AEGISCORE_OBSERVABILITY_TRACING_EXPORTER=otlp`。
- 说明当前阶段本地不要求部署 `otel-collector:4317`；默认 `exporter: none`。
- 说明 OTLP endpoint 不应包含敏感凭据。

## Testing Strategy

`common/runtime/config/loader_test.go`：

- `TestLoadExplicitConfig` 断言 YAML 反序列化 observability 字段。
- `TestLoadEnvironmentOverride` 覆盖 metrics enabled/path、tracing sample ratio/exporter/endpoint/insecure。
- 新增非法采样率测试，覆盖小于 0 和大于 1。
- 新增非法 exporter 测试。
- 新增 metrics path 为空或不以 `/` 开头测试。
- 新增 `exporter: otlp` 但 endpoint 为空测试。
- 更新生产类不安全配置测试，断言 `otlp + insecure` 被拒绝且错误不泄漏 endpoint。

`user-service`：

- 如果已有用户服务配置加载测试，应断言 `user-service/configs/config.yaml` 可被 `common/runtime/config.Load` 成功加载。
- 如没有专门测试，可在 implementation 阶段新增窄配置测试，避免示例配置漂移。

建议验证命令：

```bash
cd common && go test ./runtime/config
cd user-service && go test ./...
make test
make architecture-lint
```

## Risks And Mitigation

风险：配置字段落地后被误解为 `/metrics` 或 tracing exporter 已经可用。

缓解：配置样例和文档明确第一阶段只建立契约，不注册 metrics 路由、不创建 exporter、不接入 Gin middleware。

风险：`tracing.enabled: true` 与 `exporter: none` 语义不清。

缓解：文档明确 enabled 表示 tracing 配置入口和未来 provider 可启用，exporter 决定是否对外导出；第一阶段 `none` 是本地默认。

风险：未来 exporter 认证需要敏感字段，当前契约不足。

缓解：本变更刻意不引入 token/header 配置；未来必须单独设计 Secret 注入和脱敏规则。

风险：环境变量覆盖未生效。

缓解：保持 YAML 中显式列出所有 observability key，并用 loader 测试覆盖 `AEGISCORE_OBSERVABILITY_*`。

风险：生产校验过严影响 staging 本地调试。

缓解：生产类环境集合沿用现有 `prod`、`production`、`staging`；需要明文 OTLP 的调试环境应使用非生产类 environment 名称，或后续单独设计显式例外机制。
