## Context

`common/runtime/observability/tracing` 是跨服务共享的 OpenTelemetry tracing primitive。当前实现集中在 `provider.go` 与 `fx.go`，同一文件同时承担配置校验、resource 构造、OTLP exporter 构造、动态 tracer facade、底层 SDK provider lifecycle 和 Fx adapter，导致职责边界不清晰。

constructor 阶段的 Redis、Gin、Ent instrumentation 需要拿到稳定、非 nil 的 `trace.TracerProvider` 或 tracer facade，但 Fx 启动阶段才允许连接 OTLP exporter、创建 batch processor 或执行可能阻塞的初始化。因此本变更需要把立即构造与延迟启动的所有权分开，并定义重复启动、Shutdown、rollback 和关闭后继续使用动态 tracer 的结果。

受影响路径集中在 `common/runtime/observability/tracing/` 和 `openspec/changes/harden-runtime-tracing-package/specs/`。不涉及 `user-service` feature 业务代码、HTTP API、数据库 migration、OpenAPI 生成物、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 将 tracing package 拆分为聚焦文件：`provider.go`、`resource.go`、`exporter.go`、`dynamic.go`、`lifecycle.go`、`fx.go`、`doc.go` 和 `example_test.go`。
- 保持 constructor 阶段返回稳定、非 nil、可安全注入的动态 tracer provider；未启动或关闭后使用 no-op，启动后委托真实 SDK provider。
- 明确普通 constructor 与 Fx adapter 的 lifecycle 所有权：普通 constructor 立即启动并由调用方显式 `Shutdown`；Fx adapter 延迟到 `OnStart` 启动并由 `OnStop` 或 Fx rollback 关闭。
- 防止重复 `Start` 泄漏旧 SDK provider、batch processor 或 exporter；重复或非法 lifecycle 调用必须返回稳定、可测试的结果。
- 收窄 `Start` 导出面，使仅供 Fx lifecycle 使用的启动逻辑留在包内；如果存在包外调用方，迁移到普通 constructor 或 Fx adapter。
- 补齐 disabled provider、span 创建、propagator inject/extract 和显式 `Shutdown` 的 executable examples，且不连接真实 OTLP endpoint。

**Non-Goals:**

- 不安装、替换或依赖 OpenTelemetry global tracer provider 或 global propagator。
- 不新增 exporter protocol、采样策略、配置字段或兼容双 lifecycle 路径。
- 不修改 HTTP、Redis、SQL、Ent instrumentation 的业务接线。
- 不修改 user-service 配置 schema、HTTP API、OpenAPI、Ent schema、Atlas migration、Prometheus/Grafana 资产或部署文件。

## Decisions

1. 文件职责按 lifecycle 关注点拆分。

`provider.go` 保留 `Options`、`Provider`、公开 constructor、`Tracer`、`OTelTracerProvider`、`TextMapPropagator` 和 `Shutdown` 等 package 入口；`resource.go` 放置 resource 属性构造；`exporter.go` 放置 OTLP exporter factory 与错误包装；`dynamic.go` 放置动态 tracer provider 和 tracer；`lifecycle.go` 放置启动状态机、重复启动检查和关闭状态转换；`fx.go` 只负责把共享 runtime config 转换为 tracing options 并注册 Fx hook。

备选方案是继续保留单文件并只加测试。该方案改动更小，但无法降低职责混杂带来的后续维护风险，也不符合本次范围对聚焦文件的要求。

2. 使用单一动态 tracer facade，而不是 global provider 或双 provider 路径。

`Provider` 持有一个可替换的底层 SDK provider 指针。constructor 阶段动态 tracer 始终可创建 span；未启动或关闭后返回 no-op span，启动后转发给当前 SDK provider。这样 Redis、Gin、Ent 等 constructor 阶段可以安全保存 tracer provider，而无需知道 Fx lifecycle 是否已经运行。

备选方案是启动时安装 OpenTelemetry global provider。该方案会污染进程级状态，增加并行 App 和测试隔离风险，与既有规格边界冲突，因此不采用。

3. lifecycle 所有权必须唯一。

普通 constructor 负责立即创建真实 SDK provider；返回成功后调用方拥有 `Shutdown(ctx)`。Fx adapter 只创建未启动 facade 并注册 hook；`OnStart` 创建真实 SDK provider，`OnStop` 或 Fx rollback 调用同一 `Shutdown(ctx)`。Fx constructor 阶段不得创建 exporter、连接 OTLP endpoint 或启动 batch processor。

备选方案是保留公开 `Start` 供外部调用方自行组合。该方案让同一 provider 可能同时被普通代码和 Fx hook 启动，容易形成重复 Start、所有权不明和资源泄漏，因此本变更倾向将 Start 改为包内 helper。如果包外调用方存在，任务阶段先用搜索确认并迁移。

4. 重复和非法 lifecycle 调用使用明确错误或幂等语义。

`Shutdown(ctx)` 对 nil provider、未启动 provider 和已关闭 provider 保持幂等 nil 返回；启动后的第一次 Shutdown 关闭底层 SDK provider 并恢复 no-op。重复 Start 必须失败并保持原有已启动 provider 不变，避免泄漏旧 exporter 或把正在使用的 provider 静默替换。nil context、nil provider、缺失 resource 或缺失 exporter factory 等非法启动输入必须返回明确错误。

备选方案是重复 Start 自动 Shutdown 旧 provider 后替换。该方案会在并发 instrumentation 使用中引入不可预期切换和关闭错误组合问题，不符合“所有权唯一”的验收要求。

5. 示例测试使用 disabled provider 或 in-memory/no-op 路径。

`example_test.go` 展示 disabled provider、span 创建、propagator inject/extract 和显式 `Shutdown`。示例不得要求真实 OTLP endpoint，也不得依赖网络、全局 provider 或计时不稳定行为。

备选方案是为示例启动 fake OTLP collector。该方案增加测试复杂度和外部依赖，不适合 executable examples。

## Risks / Trade-offs

- `Start` 若当前被包外直接调用，收窄导出会产生 breaking change → 先搜索调用方；若只在包内使用则直接改为未导出；若存在包外调用方，迁移到 `NewProvider` 或 `NewTracingProvider` 并在任务中标记 breaking impact。
- 重复 Start 从“静默替换”变为错误可能暴露隐藏测试或调用方问题 → 添加针对重复 Start 的测试，并确保 Fx adapter 不会注册双 hook 或重复启动同一实例。
- 动态 tracer 在 Shutdown 后恢复 no-op 可能隐藏调用方晚于 lifecycle 使用 tracing 的问题 → 该行为符合安全降级契约，测试必须覆盖关闭后不 panic、不使用已关闭 SDK provider。
- 文件拆分可能引入循环依赖或测试 helper 访问内部字段困难 → 保持同一 package 内拆分，不新增接口层或测试专用生产 API。
- examples 可能因输出不可稳定断言而脆弱 → 示例只断言稳定的布尔或字段存在性，不输出 trace/span ID。

## Migration Plan

1. 搜索 `Provider.Start`、`.Start(ctx, cfg` 或 tracing 包外直接启动调用，确认是否存在包外 API 消费。
2. 将 `Start` 收窄为包内 lifecycle helper；包外调用方迁移到 `NewProvider(ctx, Options)`，Fx 调用方继续使用 `NewTracingProvider(lifecycle, cfg)`。
3. 保持 `Provider`、`Options`、`NewProvider`、`NewTracingProvider`、`Tracer`、`OTelTracerProvider`、`TextMapPropagator` 和 `Shutdown` 作为主要公开入口。
4. 部署回滚只需要回滚 Go 代码；本变更不涉及数据、配置、部署资产或外部 API 迁移。

## Open Questions

- 当前仓库外是否已有消费者直接调用 `(*Provider).Start` 无法通过本仓库搜索发现；若有，归档说明中需要标记 breaking impact，并要求迁移到普通 constructor 或 Fx adapter。

## Validation

- 运行 `go test ./runtime/observability/tracing` 于 `common/` module。
- 运行 `go test -race ./runtime/observability/tracing` 于 `common/` module。
- 运行 `go vet ./runtime/observability/tracing` 于 `common/` module。
- 运行 `go test ./runtime/observability/tracing -run Example` 验证 executable examples。
- 交付前运行 `make common-test`、`make user-service-architecture-lint`、`make lint` 和 `make verify`。
