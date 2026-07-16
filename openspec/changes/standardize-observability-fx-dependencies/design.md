## Context

`common/runtime/observability/metrics` 与 `common/runtime/observability/tracing` 目前都提供面向 Fx 的 `NewFxProvider`。metrics 的 `FxParams` 只包含一个无 tag 的 `*config.Config`；tracing 的 `FxParams` 只包含无 tag 的 `fx.Lifecycle` 与 `*config.Config`。这些输入容器没有承载 named、optional、group 或其他 DI metadata，却让共享 provider API 看起来比实际依赖更复杂。

user-service 的 `providers.Module` 始终注册 `commonmetrics.NewFxProvider` 和 `commontracing.NewFxProvider`。在 disabled 配置下，metrics provider 仍返回有效的 disabled `*Provider`，tracing provider 仍返回使用 `NeverSample` 的 provider。因此 `user-service/internal/providers/ent.go` 中正式 Ent provider 的 `Metrics` 与 `Tracing` 输入不应继续通过 `optional:"true"` 表达降级；缺失任一 provider 应由 Fx 构图失败暴露，禁用状态则由非 nil provider 的运行时语义表达。

本变更横跨 `common` 与 `user-service`，但不改变 HTTP API、数据库 schema、OpenAPI、部署清单、观测资产或安全边界。`common/runtime/observability` 继续只消费 `common/runtime/config.Config`，不得读取 user-service 私有配置类型。

## Goals / Non-Goals

**Goals:**

- 删除 metrics/tracing 中无 DI metadata 价值的 `FxParams` 输入容器，使用普通强类型参数表达唯一依赖。
- 保留 `NewFxProvider` 的 composition 职责：从共享 runtime config 投影 service name、environment、metrics/tracing 配置，并在 tracing 中注册 lifecycle shutdown hook。
- 保留 metrics nil config 错误、runtime config 到 `Options` 的转换、disabled provider 语义和 tracing provider 构造错误传播。
- 保留 tracing 对 `fx.Lifecycle` 的合理依赖；本变更只删除无 metadata 价值的输入容器，不宣称 tracing package 完全框架无关。
- 让 user-service 正式 Ent provider 消费非 optional 的 metrics/tracing provider，缺失任一 provider 时 Fx graph 校验失败。
- 明确 Ent observability 的 nil fallback 只可作为纯函数或直接构造测试的防御语义保留，正式 `providers.Module` 不得依赖 nil metrics/tracing 实现降级。

**Non-Goals:**

- 不改变 metrics enable/disable、Prometheus registry、metric family、label、HTTP handler 或 scrape context 行为。
- 不改变 tracing exporter、resource attribute、propagator、span、shutdown timeout 或错误语义。
- 不修改 logger、datastore、localcache、scheduler、workerpool、user-service feature composition、OpenAPI、数据库或部署资产；仅调整服务级 Ent provider 的 observability 输入必需性。
- 不新增服务私有 observability wrapper；user-service 继续通过 `internal/providers/fx.go` 注册共享 metrics/tracing `NewFxProvider`。
- 不将 provider lifecycle 移到调用方，不依赖全局 tracer shutdown，不新增 package-level mutable state。
- 不机械删除其他拥有 named/optional tag、lifecycle orchestration、配置裁剪、multi-result 输出或测试 seam 的 Params/adapter。

## Decisions

1. 普通强类型参数用于无 metadata 的唯一依赖。

   `NewFxProvider(cfg *config.Config)` 和 `NewFxProvider(lc fx.Lifecycle, cfg *config.Config)` 比 `FxParams` 更直接，且不会丢失 Fx 注入能力。参数对象只有在承载真实 DI metadata、较复杂输出映射或能显著提升多依赖构造可读性时才保留。

   备选方案：保留 `FxParams` 并添加注释解释无 metadata。该方案保留了当前噪声，无法形成共享 runtime provider API 的稳定治理规则，因此不采用。

2. `NewFxProvider` 保留 composition 职责，而不是成为 `NewProvider` 的别名。

   metrics `NewFxProvider` 继续负责拒绝 nil config，并将 `config.Config` 中的 app/server/log/observability 相关信息投影为 metrics `Options`。tracing `NewFxProvider` 继续负责读取共享 runtime config、构造 provider、传播构造错误，并注册 `fx.Hook{OnStop: provider.Shutdown}`。

   备选方案：让调用方直接构造 `Options` 后调用 `NewProvider`。该方案会把共享 runtime config 投影逻辑分散到 user-service 或其他调用方，削弱 `common/runtime/observability` 的跨服务 composition 边界，因此不采用。

3. tracing 继续依赖 `fx.Lifecycle`。

   tracing provider 拥有 shutdown lifecycle 语义，且 Fx app 停止时必须调用 `provider.Shutdown`。本变更只删除无 metadata 价值的输入容器，不改变 tracing provider 与 Fx lifecycle 的关系。

   备选方案：由调用方注册 shutdown hook。该方案会让 lifecycle 语义分散到服务模块，并增加遗漏 shutdown 的风险，因此不采用。

4. Ent provider 的正式 metrics/tracing 输入改为非 optional。

   因为 `providers.Module` 始终注册共享 metrics/tracing provider，且 disabled 配置由有效 provider 表达，正式 Ent provider 不再使用 optional tag 隐式降级。缺失任一 provider 时构图失败可以更早暴露 provider module drift。

   备选方案：继续保留 optional tag。该方案会掩盖 `providers.Module` 漂移，并让 disabled 与 missing 两种状态混在一起，因此不采用。

5. 保留 Ent observability nil fallback 的直接构造防御语义。

   如果当前 Ent provider 的纯函数或直接构造测试已有 nil fallback，可保留为测试 seam 或防御逻辑；但正式 `providers.Module` 必须通过非 nil metrics/tracing provider 进入 graph，不能依赖 nil fallback 表达正式降级。

   备选方案：完全删除 nil fallback。该方案可能迫使纯函数测试引入无关 Fx graph 或测试构造样板，收益有限；本变更的关键目标是正式 graph 必选输入，而不是禁止直接测试的防御路径。

## Risks / Trade-offs

- [Risk] 直接构造测试仍使用旧 `FxParams` 会编译失败。→ 更新 metrics/tracing provider 测试和所有调用点，覆盖正常配置、nil config 错误和 lifecycle stop。
- [Risk] 删除 Ent optional tag 后测试 module 缺少 observability provider 会构图失败。→ 更新 Ent provider 直接构造和 module 测试，显式提供 enabled/disabled metrics/tracing provider，并增加缺失任一 provider 时 graph 校验失败的测试。
- [Risk] 将 disabled 与 missing 语义区分后，历史测试若依赖 nil provider 表达 disabled 会失败。→ 测试中使用共享 provider 返回的 disabled/no-op provider 表达 disabled，nil fallback 仅保留给纯函数或直接构造防御语义。
- [Risk] OpenSpec 误把删除 `FxParams` 写成长期业务能力。→ specs 只表达 provider composition、配置来源、lifecycle 和正式 graph 依赖语义，不把类型删除本身作为 capability。

## Migration Plan

1. 修改 `common/runtime/observability/metrics/fx.go` 和 `tracing/fx.go` 的 `NewFxProvider` 签名，并删除不再需要的 `FxParams` 类型。
2. 更新 metrics/tracing 直接构造测试，覆盖正常配置、nil config 错误、provider 构造和 lifecycle stop 调用 shutdown。
3. 修改 `user-service/internal/providers/ent.go`，删除 `NamedEntClientParams.Metrics` 与 `Tracing` 的 `optional:"true"` tag。
4. 更新 Ent provider 直接构造和 module 测试，显式提供 enabled/disabled metrics/tracing provider，并验证缺失任一正式 provider 时 graph 校验失败。
5. 使用 Fx module/app 测试验证 user-service `providers.Module` 仍能解析并启动共享 metrics/tracing provider。
6. 运行 common 与 user-service 受影响测试、`make user-service-architecture-lint`、`openspec validate standardize-observability-fx-dependencies`，并在暂存预期变更后运行 `make lint` 与 `make verify`。

回滚方式：恢复旧 `FxParams` 类型、`NewFxProvider` 签名和 Ent optional tag，并回退对应测试与 spec delta。由于不涉及数据、HTTP API、OpenAPI 或部署资产，回滚不需要 migration 或发布编排。

## Open Questions

- 无。
