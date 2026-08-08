## Context

`common/runtime/observability/metrics` 是跨服务共享 metrics primitive，当前同一个 package 内已经包含 provider、Prometheus registry、context-aware gather、runtime collector 以及 SQL、Redis、scheduler、workerpool、localcache 和 component status 等 adapter。现有导出 API 已被 user-service router、datastore provider、scheduler、localcache 和测试消费，因此本次变更必须保持 package、import path、导出 API 与指标契约兼容，只整理同 package 文件职责并补齐文档与示例。

本变更同时影响 `runtime-observability` 和 `shared-platform-primitives`：前者约束 metrics 运行时启停、HTTP scrape context、registry 和低基数指标契约；后者约束 `common` 的业务中立 primitive 边界、测试示例和不引入服务私有语义。

## Goals / Non-Goals

**Goals:**

- 将 Provider/registry、`ContextCollector`/gather wrapper、runtime collector 与各 adapter 拆分为可独立审查的同 package 文件。
- 保持 `metrics.Provider`、`NewProvider`、`NewMetricsProvider`、`Register`、`MustRegister`、`Registerer`、`Gatherer`、`GatherContext`、`HTTPHandler` 等导出 API 和行为兼容。
- 通过 `doc.go` 说明 enabled/disabled provider、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 和 label cardinality 约束。
- 通过 `example_test.go` 使用本地 registry 演示 enabled/disabled provider、自定义 collector 注册、gather 和 `HTTPHandler`，并确保示例可由 `go test` 执行。
- 更新 OpenSpec delta，明确 runtime metrics provider 的稳定使用契约和 common 业务中立边界。

**Non-Goals:**

- 不新增业务指标，不修改现有指标名称、label、bucket 或采集语义。
- 不安装或依赖 Prometheus global registry，不把共享 provider 变成进程全局 singleton。
- 不创建 `metrics` 子包，不改变调用方 import path。
- 不引入 user-service feature DTO、业务状态、业务 key schema、policy loader 或服务私有配置。
- 不修改 HTTP API、数据库 schema、OpenAPI 生成物、部署清单、dashboard 或 Prometheus rules。

## Decisions

- 决策：保留同 package 拆分文件，而不是创建子包。
  原因：调用方已依赖 `common/runtime/observability/metrics` 的导出 API；子包会增加 import path 和跨包可见性成本，且不符合“不创建 metrics 子包”的约束。备选方案是创建 `metrics/provider`、`metrics/collector` 等子包，但会造成兼容性风险和过度抽象。

- 决策：将 Provider/registry 与 context-aware gather 逻辑拆开，但不拆分公开类型归属。
  原因：Provider、registry、启停语义和 HTTP handler 是一个稳定运行时入口；`ContextCollector` wrapper 与 `GatherContext` 是 scrape context 传播机制，单独文件更便于审查 Redis PING 取消等行为。备选方案是保持在单个 `provider.go`，但审查者仍需要在一个文件内跨职责阅读。

- 决策：保留独立 registry 和重复注册成功语义。
  原因：共享 provider 必须允许多个测试或多个 App 并行构造，不污染 Prometheus global registry；重复注册成功是现有调用方可安全重复装配 collector 的兼容行为。备选方案是直接暴露 global registry 或将 `AlreadyRegisteredError` 返回给调用方，但会破坏隔离性或现有行为。

- 决策：`HTTPHandler` 继续通过 request context 调用 `GatherContext`，普通 `Gatherer().Gather()` 和标准 `Collect` 路径继续使用 background context。
  原因：只有 HTTP scrape 具备请求取消信号；标准 Prometheus collector 接口没有 context 参数，强行把取消语义扩展到所有路径会造成误导。备选方案是在 collector 内保存外部 context 或要求调用方传递 context-aware gatherer，但会增加状态共享和竞态风险。

- 决策：示例测试只使用本地 registry、自定义轻量 collector 和 `httptest`。
  原因：示例的目标是固化 API 用法和边界，不应连接公网、真实 PostgreSQL、Redis 或 scheduler worker。备选方案是复用现有 runtime collector，但会让示例依赖真实 runtime 状态或不稳定输出。

## Risks / Trade-offs

- 文件拆分过程中遗漏导出 symbol 或改变注释归属 -> 通过 `go test ./common/runtime/observability/metrics`、示例测试和 `go vet ./common/runtime/observability/metrics` 验证。
- `ContextCollector` wrapper 拆分后出现 scrape context 串扰 -> 保留 provider 内既有 gather mutex 和 context 清理语义，并运行 race 测试覆盖 context-aware gather。
- 示例输出受 Prometheus 文本格式或默认 collector 影响 -> 示例使用独立 registry 和最小自定义 collector，避免注册 Go runtime/process 默认 collector。
- 文档承诺超过实现能力 -> `doc.go` 只描述现有稳定行为和本次整理后必须保持的约束，不声称普通 `Collect` 能感知 HTTP 取消。
- 仅重构文件导致行为 drift 难以发现 -> 对现有 metrics 包测试、race 测试、lint 和 `make verify` 建立任务门禁。

## Migration Plan

本变更不需要数据迁移、配置迁移、部署顺序调整或运行时兼容层。实现时先新增 OpenSpec delta，再在同 package 内移动代码和补充文档/示例；如果验证失败，回滚同 package 文件拆分和新增文档/示例即可，不影响数据库、HTTP API、OpenAPI 或部署资产。

## Open Questions

无。当前需求已明确不改变导出 API、指标契约、import path、部署资产或业务指标语义。
