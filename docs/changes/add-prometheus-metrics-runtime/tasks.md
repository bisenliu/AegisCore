# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`。
- [x] 确认本 change 目录使用 `docs/changes/add-prometheus-metrics-runtime/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 梳理 `common/runtime/config.MetricsConfig` 当前字段、校验规则和用户服务默认配置。
- [x] 梳理 `common/runtime/observability/metrics/README.md` 当前边界说明。
- [x] 梳理 `common/runtime/scheduler.Metrics` 和 `common/runtime/workerpool.Stats()`，确认 Prometheus 接入不反向污染 scheduler/workerpool。
- [x] 确认本变更不挂载用户服务 `/metrics` 路由，不新增业务指标、dashboard、告警或部署清单。

## Dependencies

- [x] 在 `common/` 中引入 Prometheus Go client 依赖。
- [x] 只引入 `github.com/prometheus/client_golang` 及其必要间接依赖；不要引入 OpenTelemetry Metrics。
- [x] 运行 `go mod tidy` 更新 `common/go.mod` 和 `common/go.sum`。

## Metrics Provider

- [x] 在 `common/runtime/observability/metrics` 新增 provider 实现文件。
- [x] 定义 `Options`，包含 `config.MetricsConfig`、`ServiceName` 和 `Environment`。
- [x] 定义 `Provider`，持有启用状态、Prometheus registerer 和 gatherer。
- [x] 实现 `NewProvider(opts)`。
- [x] 校验 `ServiceName` 非空。
- [x] 校验 `Environment` 非空。
- [x] metrics disabled 时返回 no-op provider，不创建 registry、不注册 collector、不注册 runtime/process collector。
- [x] metrics enabled 时创建独立 `prometheus.Registry`，不使用默认全局 registry。
- [x] enabled provider 对外暴露稳定 `Registerer()` 和 `Gatherer()`。
- [x] 将 `service` 与 `environment` 作为 provider-level const labels 注入。
- [x] 实现 `Enabled()`。
- [x] 实现 `Register(collector)`，对 disabled provider 保持零副作用。
- [x] 实现重复注册保护，`prometheus.AlreadyRegisteredError` 不导致失败。
- [x] 如提供 `MustRegister`，不要在重复注册时 panic。

## Runtime Collectors

- [x] `include_runtime: true` 时注册 Go runtime collector。
- [x] `include_runtime: true` 时注册 process collector。
- [x] `include_runtime: false` 时不注册 runtime/process collector。
- [x] runtime/process collector 只注册到 provider 独立 registry。
- [x] runtime/process collector 注册使用统一重复注册保护。

## Label And Naming Contract

- [x] 新增 label key 常量：`service`、`environment`、`method`、`route`、`status_class`、`code`。
- [x] 实现 `StatusClass(status int)` helper，返回稳定 `2xx`、`3xx`、`4xx`、`5xx` 或非法状态 fallback。
- [x] 文档明确 `service` 与 `environment` 由 provider 注入，其他 collector 不得重复定义同名 label。
- [x] 文档明确禁止 user ID、role ID、permission ID、session ID、token ID、trace ID、span ID、request ID、IP、User-Agent、query、raw path、email、username、SQL、Redis key、JWT、Authorization header、Cookie 和原始错误消息作为 label。
- [x] 文档明确 HTTP、scheduler 和 workerpool 指标命名约定。
- [x] 确认指标名不包含服务名或环境名。

## Fx Wiring

- [x] 评估并新增窄 Fx provider。
- [x] Fx provider 从 `config.Config.App.Name`、`config.Config.App.Environment` 和 `config.Config.Observability.Metrics` 构造 provider。
- [x] Fx provider 不自动挂载 `/metrics` route。
- [x] Fx provider 不创建后台资源；如没有 shutdown 需求，不注册 OnStop。
- [x] 不在本变更中接入 user-service `AppModule`，除非只注入 provider 且不会改变 HTTP 路由行为。

## Optional Adapters

- [x] 评估是否在本变更中实现 scheduler Prometheus adapter；本变更仅保留命名和 adapter 边界，不实现具体 adapter。
- [x] 未实现 scheduler adapter；确认后续若实现必须放在 metrics package 或服务侧 wiring，不放回 `common/runtime/scheduler`。
- [x] 未实现 scheduler adapter；确认后续 disabled provider 应返回 `scheduler.NopMetrics{}`。
- [x] 未实现 scheduler adapter；确认后续 job key 和 reason 必须是固定枚举式低基数值。
- [x] 评估是否在本变更中实现 workerpool collector；本变更仅保留命名和 adapter 边界，不实现具体 collector。
- [x] 未实现 workerpool collector；确认后续只读取 `workerpool.Stats()` snapshot，不改变 pool 生命周期。
- [x] 未实现 workerpool collector；确认后续 pool label 使用固定配置名。
- [x] 不在本变更中实现 Gin HTTP middleware 或用户服务 route instrumentation。

## Documentation

- [x] 更新 `common/runtime/observability/metrics/README.md`，说明当前支持 Prometheus registry/provider。
- [x] 在 README 中说明 disabled 零副作用、enabled 独立 registry 和 include_runtime 行为。
- [x] 在 README 中说明 provider 不使用默认全局 registry，不自动挂载 `/metrics`。
- [x] 在 README 中说明 label key、低基数限制和禁止的敏感/高基数 label。
- [x] 在 README 中说明 HTTP、scheduler、workerpool 指标命名约定和边界。
- [x] 更新 `docs/ARCHITECTURE.md`，将 metrics 当前状态改为已支持 Prometheus runtime primitive。
- [x] 在 `docs/ARCHITECTURE.md` 明确 common metrics 不承载业务指标、dashboard、告警或部署清单。
- [x] 在 `docs/ARCHITECTURE.md` 明确 scheduler/workerpool Prometheus 接入通过 adapter 或服务侧 wiring 完成。
- [x] 更新 `docs/DEVELOPMENT.md`，说明 metrics 配置字段、provider 行为和本变更仍不挂载 `/metrics`。
- [x] 确认文档不重新引入 OpenSpec/OPSX 流程或目录。

## Tests

- [x] 新增 `common/runtime/observability/metrics/provider_test.go`。
- [x] 覆盖 disabled provider 零副作用。
- [x] 覆盖 enabled provider 创建独立 registry。
- [x] 覆盖注册测试 counter 后 gather 可读取 metric family。
- [x] 覆盖 provider-level `service` 和 `environment` const labels。
- [x] 覆盖 `include_runtime: true` 注册 Go runtime/process collector。
- [x] 覆盖 `include_runtime: false` 不注册 runtime/process collector。
- [x] 覆盖重复注册同一 collector 不报错。
- [x] 覆盖非重复注册错误不被吞掉。
- [x] 覆盖缺失 service name 或 environment 返回错误。
- [x] 新增 `labels_test.go`，覆盖 label 常量值和 `StatusClass`。
- [x] 未实现 scheduler adapter，因此不新增 completed、failed、skipped 和 disabled nop 行为测试。
- [x] 未实现 workerpool collector，因此不新增 stats snapshot gauge/counter 测试。

## Verification

- [x] 格式化修改过的 Go 文件：

```bash
gofmt -w common/runtime/observability/metrics/*.go
```

- [x] 运行 metrics package 测试：

```bash
cd common && go test ./runtime/observability/metrics
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
git diff -- docs/changes/add-prometheus-metrics-runtime common/runtime/observability/metrics common/go.mod common/go.sum docs/ARCHITECTURE.md docs/DEVELOPMENT.md
```

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/`。
- [x] 不在 common 中定义 auth/user/role/permission 业务指标。
- [x] 不新增或挂载用户服务 `/metrics` 路由。
- [x] 不接入 Gin middleware、HTTP access metrics 或具体 route 采集实现。
- [x] 不接入 Grafana dashboard、告警规则、Prometheus scrape 配置、Kubernetes ServiceMonitor 或部署清单。
- [x] 不引入 OpenTelemetry Metrics。
- [x] 不修改 HTTP API 响应、认证、RBAC、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 不在错误消息、配置样例或文档中写入真实 Prometheus endpoint、token、Authorization header、Cookie 或其他敏感凭据。
