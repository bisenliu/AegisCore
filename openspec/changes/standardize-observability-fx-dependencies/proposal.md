## Why

当前 `common/runtime/observability` 的 metrics/tracing Fx provider 使用的 `FxParams` 只包裹无 `name`、`optional`、`group` 或其他 DI metadata 的强类型依赖，增加了共享 provider API 的噪声，也让调用方难以判断哪些输入是真正的 Fx metadata。user-service 的正式 Ent provider 同时将 metrics/tracing 标记为 optional，但 `providers.Module` 始终注册共享 metrics/tracing provider，且 disabled 配置已经由非 nil disabled/no-op provider 表达，因此 optional 降级语义与服务级依赖图不一致。

## What Changes

- metrics provider：删除只包含无 tag `*config.Config` 的 `FxParams`，将 `NewFxProvider` 改为直接接收 `*config.Config`，继续拒绝 nil config，并继续从共享 runtime config 投影 service name、environment 和 metrics `Options`。
- tracing provider：删除只包含无 tag `fx.Lifecycle` 与 `*config.Config` 的 `FxParams`，将 `NewFxProvider` 改为直接接收这两个强类型参数，继续传播 provider 构造错误，并继续注册 `OnStop: provider.Shutdown` lifecycle hook。
- `NewFxProvider` 不会退化为 `NewProvider` 的无语义别名；它继续承担从共享 runtime config 组合 service name、environment、metrics/tracing 配置和 lifecycle 的职责。
- user-service 继续通过 `internal/providers/fx.go` 注册 `commonmetrics.NewFxProvider` 和 `commontracing.NewFxProvider`，不新增服务私有 observability wrapper。
- Ent provider 删除 `NamedEntClientParams.Metrics` 与 `Tracing` 上的 `optional:"true"` tag，使正式 `providers.Module` 构图缺失任一共享 observability provider 时失败；metrics disabled 配置继续返回 disabled `*Provider`，tracing disabled 配置继续返回使用 `NeverSample` 的 provider。
- 更新 metrics、tracing、Ent provider 的直接构造测试和 Fx module/app 测试，覆盖 enabled/disabled provider、nil config、lifecycle shutdown 和缺失正式 provider 时的构图失败。
- 更新必要的 docs/OpenSpec 规格，表达长期稳定的 provider composition 和 lifecycle 语义，而不是把删除 `FxParams` 类型本身作为业务 capability。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：共享 runtime primitive 的 Fx provider 装配应在依赖类型唯一且无 DI metadata 时使用普通强类型参数，并保持只消费 `common/runtime/config.Config` 的跨服务配置边界。
- `runtime-observability`：共享 metrics/tracing provider 的配置投影、disabled/no-op、错误传播和 tracing shutdown lifecycle 语义保持稳定；user-service 正式 Ent provider 必须依赖非 optional 的 metrics/tracing provider，禁用状态由非 nil provider 表达。

## Impact

- 影响代码限定为 `common/runtime/observability/metrics/fx.go`、`common/runtime/observability/tracing/fx.go`、`user-service/internal/providers/ent.go`、对应测试，以及必要的 docs/OpenSpec 规格。
- 不改变 metrics enable/disable、Prometheus registry、metric family、label、HTTP handler 或 scrape context 行为。
- 不改变 tracing exporter、resource attribute、propagator、span、shutdown timeout 或错误语义。
- 不修改 logger、datastore、localcache、scheduler、workerpool、user-service feature composition、OpenAPI、数据库或部署资产；仅调整服务级 Ent provider 的 observability 输入必需性。
- 不机械删除其他拥有 named/optional tag、lifecycle orchestration、配置裁剪、multi-result 输出或测试 seam 的 Params/adapter。
- 该变更不会引入新的外部 API、HTTP API、数据库 schema、OpenAPI 或部署资产变更。
