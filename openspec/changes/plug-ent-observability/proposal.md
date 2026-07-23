## Why

当前 Ent client 的 SQL driver wrapper、SQL debug 日志、慢查询/失败/完成日志和 Prometheus query metrics 由 provider 隐式默认安装，导致默认运行时行为与配置契约不清晰，也让 tracing、SQL log 和 metrics 难以独立演进。需要通过一次破坏性重构，把 Ent 观测能力拆成显式可插拔插件，使默认 Ent client 更轻量，并让运维侧按配置明确启用所需能力。

## What Changes

- **BREAKING** 删除 `ent.sql_debug` 配置项，不再通过该字段控制成功 SQL debug 日志。
- **BREAKING** 删除 Ent SQL log 和 Ent query metrics 的隐式默认行为；默认配置下不再输出 `ent sql completed`、`ent sql slow`、`ent sql failed`，也不再注册 Ent query Prometheus metrics。
- 新增 `ent.plugins.sql_log`、`ent.plugins.tracing`、`ent.plugins.metrics` 配置契约，用于显式启用 SQL log driver plugin、Ent tracing client plugin 和 Ent metrics client plugin。
- `newEntClient` 只负责组装基础 Ent driver、按顺序应用 driver/client 插件并创建 `*ent.Client`。
- SQL log 能力从 `newEntDriver` 硬编码逻辑迁移为独立 `dialect.Driver` 插件。
- tracing 能力迁移为独立 Ent client 插件，不依赖 SQL driver wrapper。
- metrics 能力迁移为独立 Ent client 插件，不再和 tracing 插件强绑定。
- 默认配置仅启用 Ent tracing 插件；SQL log 和 metrics 必须显式启用。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`: 修改 Ent 观测配置与默认行为，新增 Ent 可插拔观测契约。

## Impact

- 代码影响：`user-service/internal/config/config.go`、`user-service/internal/providers/ent.go`、Ent 观测相关 provider 文件和对应测试需要重构。
- 配置影响：所有 `ent.sql_debug` 示例、测试 fixture 和部署配置需要删除或替换为 `ent.plugins.*`。
- 观测影响：默认不再产出 Ent SQL log 和 Ent query metrics；Prometheus/Grafana 资产如果依赖 Ent query metrics，需要说明仅在 `ent.plugins.metrics.enabled=true` 且 metrics provider 启用时有数据。
- 兼容性影响：这是破坏性配置与运行时行为变更，旧配置 `ent.sql_debug` 不再是有效契约。
- 规格影响：更新 `runtime-observability` capability，明确默认 tracing、显式 SQL log 和显式 metrics 的行为边界。
