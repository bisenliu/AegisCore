# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本仓库不新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认 change 目录使用 `docs/changes/add-observability-config-contract/`。
- [x] 梳理 `common/runtime/config` 当前 `Config`、`Load`、`Validate` 和测试 fixture。
- [x] 确认本变更只建立配置契约，不实现 metrics exporter、中间件、`/metrics` 路由、tracer provider 或 Gin tracing middleware。

## Common Config Contract

- [x] 在 `common/runtime/config/config.go` 为根 `Config` 增加 `Observability ObservabilityConfig`。
- [x] 新增 `ObservabilityConfig`，包含 `Metrics MetricsConfig` 和 `Tracing TracingConfig`。
- [x] 新增 `MetricsConfig` 字段：`Enabled`、`Path`、`IncludeRuntime`，mapstructure key 分别为 `enabled`、`path`、`include_runtime`。
- [x] 新增 `TracingConfig` 字段：`Enabled`、`SampleRatio`、`Exporter`、`OTLPEndpoint`、`Insecure`，mapstructure key 分别为 `enabled`、`sample_ratio`、`exporter`、`otlp_endpoint`、`insecure`。
- [x] 保持注释为中文，Go identifier 和配置 key 使用现有英文风格。

## Validation

- [x] 在 `common/runtime/config/validation.go` 增加 tracing exporter 允许集合，第一阶段至少包含 `none` 和 `otlp`。
- [x] 在 `Config.Validate` 中调用 `validateObservability`。
- [x] 校验 `observability.metrics.path` 非空。
- [x] 校验 metrics enabled 时 `observability.metrics.path` 以 `/` 开头。
- [x] 校验 `observability.tracing.sample_ratio` 位于 `0.0` 到 `1.0`。
- [x] 校验 `observability.tracing.exporter` 非空且属于允许集合。
- [x] 校验 `exporter: otlp` 时 `observability.tracing.otlp_endpoint` 非空。
- [x] 校验生产类环境中 `exporter: otlp` 不允许 `observability.tracing.insecure: true`。
- [x] 确认错误消息不回显 `otlp_endpoint` 原文。

## User Service Config

- [x] 更新 `user-service/configs/config.yaml`，新增 `observability.metrics` 和 `observability.tracing` 配置段。
- [x] 本地默认建议：metrics `enabled: false`、`path: /metrics`、`include_runtime: true`。
- [x] 本地默认建议：tracing `enabled: true`、`sample_ratio: 1.0`、`exporter: none`、`otlp_endpoint: ""`、`insecure: false`。
- [x] 注释说明当前阶段不注册 `/metrics` 路由，也不要求本地部署 OTLP Collector。
- [x] 注释说明生产 exporter endpoint 和认证信息应通过安全配置渠道管理，当前 endpoint 不应包含凭据。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md`，说明 observability 配置契约属于 `common/runtime/config`。
- [x] 在 `docs/ARCHITECTURE.md` 明确本变更不实现 metrics exporter、中间件、`/metrics` 路由、tracer provider 或 Gin tracing middleware。
- [x] 在 `docs/ARCHITECTURE.md` 明确 `common/runtime/observability/*` 只能承载无业务语义的未来 runtime primitive，不承载业务指标、dashboard、告警或部署清单。
- [x] 更新 `docs/DEVELOPMENT.md` 配置章节，说明 observability 字段、默认值和环境变量覆盖。
- [x] 在 `docs/DEVELOPMENT.md` 给出至少一个覆盖示例，例如 `AEGISCORE_OBSERVABILITY_TRACING_EXPORTER=otlp`。
- [x] 在 `docs/DEVELOPMENT.md` 说明当前阶段本地不强制部署 `otel-collector:4317`。
- [x] 确认文档不重新引入 OpenSpec/OPSX 流程或目录。

## Tests

- [x] 更新 `common/runtime/config/loader_test.go` 的显式 YAML fixture，包含完整 observability 配置。
- [x] 更新 `TestLoadExplicitConfig`，断言 metrics 和 tracing 字段反序列化正确。
- [x] 更新 `TestLoadEnvironmentOverride`，覆盖 `AEGISCORE_OBSERVABILITY_METRICS_ENABLED`、`AEGISCORE_OBSERVABILITY_METRICS_PATH`、`AEGISCORE_OBSERVABILITY_TRACING_SAMPLE_RATIO`、`AEGISCORE_OBSERVABILITY_TRACING_EXPORTER`、`AEGISCORE_OBSERVABILITY_TRACING_OTLP_ENDPOINT` 和 `AEGISCORE_OBSERVABILITY_TRACING_INSECURE`。
- [x] 新增或扩展非法配置测试，覆盖空 metrics path。
- [x] 新增或扩展非法配置测试，覆盖非法 metrics path。
- [x] 新增或扩展非法配置测试，覆盖非法 sample ratio。
- [x] 新增或扩展非法配置测试，覆盖非法 tracing exporter。
- [x] 新增或扩展非法配置测试，覆盖 `exporter: otlp` 但 endpoint 为空。
- [x] 新增或扩展生产类不安全配置测试，覆盖 `exporter: otlp` 且 `insecure: true`。
- [x] 如 user-service 尚无配置样例加载测试，新增窄测试确认 `user-service/configs/config.yaml` 可成功加载。
- [x] 运行 `gofmt` 格式化修改过的 Go 文件。

## Verification

- [x] 运行 common 配置测试：

```bash
cd common && go test ./runtime/config
```

- [x] 运行用户服务测试：

```bash
cd user-service && go test ./...
```

- [x] 运行完整测试：

```bash
make test
```

- [x] 运行架构边界检查：

```bash
make architecture-lint
```

- [x] 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 检查配置和文档变更范围：

```bash
git diff -- docs/changes/add-observability-config-contract common/runtime/config user-service/configs/config.yaml docs/ARCHITECTURE.md docs/DEVELOPMENT.md
```

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/`。
- [x] 不实现真实 metrics exporter、中间件或 `/metrics` 路由。
- [x] 不实现 OpenTelemetry tracer provider、Gin middleware、span 创建或日志字段迁移。
- [x] 不新增业务指标、dashboard、告警规则或部署清单。
- [x] 不修改 HTTP API 响应、认证、RBAC、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 不在配置样例、错误消息或文档中写入真实 collector 地址、token、Authorization header、Cookie 或其他敏感凭据。
