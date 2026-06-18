# Add Prometheus metrics runtime

## What

在 `common/runtime/observability/metrics` 中落地跨服务通用 Prometheus metrics runtime primitive。

本变更新增无业务语义的 metrics 基础能力：

- 引入 Prometheus Go client 依赖，提供独立 registry/provider 构造能力。
- 基于 `common/runtime/config.MetricsConfig` 支持启用/禁用 metrics、配置 metrics path、可选注册 Go runtime/process collector。
- 建立跨服务统一指标命名和 label key 约定，至少覆盖 `service`、`environment`、`method`、`route`、`status_class`、`code`。
- 提供 HTTP 指标、runtime 指标、scheduler 指标和 workerpool 指标可复用的 registry 与注册约定。
- 提供 Fx 生命周期友好的 provider wiring，但不自动挂载用户服务 `/metrics` 路由。
- 更新 `common/runtime/observability/metrics` 文档，使该目录从 README 占位变成可测试 runtime primitive。

## Why

仓库已经有 `observability.metrics` 配置契约，`scheduler.Metrics` 接口和 `workerpool.Stats()` 也已经为监控接入预留了稳定输入，但当前 `common/runtime/observability/metrics` 仍只有边界 README。后续 HTTP server、定时任务和后台任务池都需要一套一致的 Prometheus registry、collector 注册方式、指标命名和 label 基数约束。

先在 `common` 中建立 Prometheus runtime primitive 可以带来：

- 避免每个服务各自创建 registry、重复注册 collector 或漂移命名规范。
- 保持 metrics 只承载跨服务 runtime 能力，不混入 auth/user/role/permission 业务指标。
- 让禁用模式具备零副作用，避免未启用 metrics 时创建全局 collector 或暴露路由。
- 让 Go runtime/process 指标、scheduler 指标和 workerpool 指标有统一接入点。
- 为后续服务侧 `/metrics` 路由挂载和 HTTP middleware 指标接入打基础，但不在本变更中扩大 user-service 行为面。

## Scope

包括：

- 在 `common/` 引入 Prometheus Go client 依赖。
- 在 `common/runtime/observability/metrics` 新增 registry/provider 实现。
- 定义 `Options` 或等价构造输入，至少包含：
  - `config.MetricsConfig`
  - service name
  - deployment environment
  - 可选 registry namespace/subsystem 约束
- 定义 `Provider`，持有 Prometheus `Registerer`、`Gatherer` 和启用状态。
- 启用模式创建独立 `prometheus.Registry`，避免使用默认全局 registry。
- 禁用模式不注册 collector、不创建 runtime/process 指标副作用，并返回可安全注入的 no-op provider。
- `include_runtime: true` 时注册 Go runtime/process collector。
- 对 collector 注册提供重复注册保护，重复注册同一 collector 不导致启动失败。
- 定义通用 label key 常量和高基数限制文档。
- 提供 HTTP server 指标、scheduler 指标和 workerpool 指标的命名/label 约定；第一阶段可先提供 registry 能力和适配接口，具体业务服务指标不在 common 中定义。
- 提供 Fx provider 或 lifecycle helper，从 `config.Config.App` 与 `config.Config.Observability.Metrics` 构造 provider。
- 增加单元测试，覆盖 registry 创建、重复注册保护、禁用模式和 runtime 指标开关。
- 更新 `common/runtime/observability/metrics/README.md`、`docs/ARCHITECTURE.md` 和 `docs/DEVELOPMENT.md`。

不包括：

- 不在 common 中定义 auth/user/role/permission 业务指标。
- 不新增或挂载用户服务 `/metrics` 路由。
- 不接入 Gin middleware、HTTP access metrics 或具体 route 采集实现。
- 不修改 `scheduler.Metrics` 的现有接口语义，除非实现 Prometheus adapter 时确有必要且保持无业务语义。
- 不改造 `workerpool.Pool` 执行模型，只消费稳定 `Stats()` 快照或定义后续 adapter 入口。
- 不接入 Grafana dashboard、告警规则、Prometheus scrape 配置、Kubernetes ServiceMonitor 或部署清单。
- 不引入 OpenTelemetry Metrics。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不修改 HTTP API 响应、认证、RBAC、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。

## Acceptance Criteria

- `common/runtime/observability/metrics` 不再只是 README，占位边界变成可测试 runtime primitive。
- metrics disabled 时构造 provider 不注册 collector，不创建 Go runtime/process collector 副作用，调用方可安全注入。
- metrics enabled 时可创建独立 Prometheus registry，并暴露稳定 `Registerer` 与 `Gatherer`。
- `include_runtime: true` 时注册 Go runtime/process collector；`false` 时不注册 runtime/process collector。
- 重复注册同一 collector 或重复注册 runtime collector 时不会让 provider 启动失败。
- 通用 label key、指标命名和高基数限制在 README 与架构文档中明确。
- provider/Fx wiring 不包含用户服务业务语义，也不自动挂载 `/metrics` 路由。
- 单元测试覆盖 registry 创建、禁用模式、runtime 指标开关、重复注册保护和 label 约束 helper。
- 实现后 `make test-common` 通过。
- 未新增 `openspec/` 或 `docs/opsx/` 目录。
