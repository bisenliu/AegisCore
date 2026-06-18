# Add observability config contract

## What

在 `common/runtime/config` 中建立可观测性配置契约，作为后续 metrics、tracing、采样和 exporter 行为的统一配置入口。

本变更新增根级 `observability` 配置段：

- `observability.metrics`：声明 metrics 是否开启、HTTP 暴露路径和是否包含 Go runtime 指标。
- `observability.tracing`：声明 tracing 是否开启、采样率、exporter 类型、OTLP endpoint 和是否允许 insecure 连接。
- 配置加载和校验统一在 `common/runtime/config` 完成，服务侧只消费结构化配置，不自行解析散落字段。
- `user-service/configs/config.yaml` 提供本地可运行默认值：metrics 可配置开启，tracing 默认使用本进程 SDK provider 语义且不要求部署 OTLP Collector。
- `docs/ARCHITECTURE.md` 和 `docs/DEVELOPMENT.md` 说明配置字段、默认值、环境变量覆盖和敏感信息边界。

第一阶段允许 `tracing.exporter: none`，表示启用本进程 tracing 配置入口但不向外部 collector 导出 span。后续真实 exporter、中间件和路由实现应复用该契约。

## Why

后续 metrics、tracing、采样、exporter 和运行时观测能力会跨服务共享基础结构。如果各服务分别新增配置字段，容易产生命名漂移、默认值不一致、生产安全校验遗漏和环境变量覆盖规则分散。

先建立配置契约可以带来：

- 为 metrics、tracing 和 exporter 后续实现提供稳定入口。
- 保持本地开发低门槛，不要求开发者额外部署 `otel-collector:4317`。
- 在配置校验阶段尽早拒绝非法采样率、空 metrics path、未知 exporter 和生产环境明显不安全组合。
- 明确 OTLP endpoint 等连接信息的敏感边界，避免文档或错误消息泄漏 token、header 或完整凭据。
- 让 `common` 拥有跨服务无业务语义的配置契约，服务模块只负责消费和服务侧 wiring。

## Scope

包括：

- 在 `common/runtime/config.Config` 增加 `Observability ObservabilityConfig` 字段，mapstructure key 为 `observability`。
- 新增 `ObservabilityConfig`、`MetricsConfig` 和 `TracingConfig`。
- `MetricsConfig` 至少包含：
  - `enabled`
  - `path`
  - `include_runtime`
- `TracingConfig` 至少包含：
  - `enabled`
  - `sample_ratio`
  - `exporter`
  - `otlp_endpoint`
  - `insecure`
- 更新配置校验：
  - metrics path 不能为空，开启 metrics 时必须是以 `/` 开头的绝对 HTTP path。
  - tracing sample ratio 必须位于 `0.0` 到 `1.0`。
  - tracing exporter 必须是允许集合，第一阶段至少支持 `none` 和 `otlp`。
  - `exporter: none` 不要求 `otlp_endpoint`。
  - `exporter: otlp` 需要非空 `otlp_endpoint`。
  - 生产类环境拒绝明显不安全组合，例如 `tracing.exporter: otlp` 同时 `tracing.insecure: true`。
- 更新 `user-service/configs/config.yaml`，提供本地默认配置。
- 更新 `docs/ARCHITECTURE.md`，说明 observability 配置归属、边界和当前阶段不实现 runtime exporter。
- 更新 `docs/DEVELOPMENT.md`，说明配置字段、环境变量覆盖示例和本地不强制部署 OTLP Collector。
- 更新配置反序列化与校验测试，覆盖默认值、非法采样率、非法 exporter、metrics path 和环境变量覆盖。

不包括：

- 不实现真实 metrics exporter、中间件或 `/metrics` 路由。
- 不实现 OpenTelemetry tracer provider、Gin middleware、span 创建或日志字段迁移。
- 不新增 `openspec/` 或 `docs/opsx/` 目录。
- 不引入业务指标、dashboard、告警规则、Grafana/Prometheus/Collector 部署清单或 Kubernetes 资源。
- 不新增新的 runtime observability 包行为；已有 `common/runtime/observability/metrics` 与 `tracing` README 只作为边界说明保留。
- 不改动 HTTP API 响应、认证、RBAC、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。

## Acceptance Criteria

- `common/runtime/config` 暴露 `ObservabilityConfig`、`MetricsConfig` 和 `TracingConfig`，并能从 YAML 与 `AEGISCORE_` 环境变量覆盖中正确反序列化。
- `user-service/configs/config.yaml` 包含可运行的本地 observability 默认配置，tracing 默认不要求本地部署 OTLP Collector。
- 配置校验覆盖并拒绝非法采样率、空或非法 metrics path、非法 exporter 类型、`otlp` exporter 的空 endpoint 和生产类环境不安全 OTLP 组合。
- `docs/ARCHITECTURE.md` 说明 observability 配置属于 `common/runtime/config` 的跨服务契约，并明确本变更不实现 exporter、中间件或 `/metrics` 路由。
- `docs/DEVELOPMENT.md` 说明配置字段、默认值、环境变量覆盖方式和本地阶段不强制部署 `otel-collector:4317`。
- 测试覆盖配置默认值、非法采样率、非法 exporter、metrics path 和环境变量覆盖。
- 实现后 `make test` 和 `make architecture-lint` 通过。
