## Why

`common/runtime/localcache` 已重构为带 `StatsSource` 的有界 loading cache，并由 runtime observability collector 导出结构化 Prometheus 指标；但现有 Grafana、告警和真实 metrics load 脚本中的 PromQL 还没有系统覆盖这些 localcache metric family。现在需要补齐观测消费面，避免缓存命中率、回源错误、singleflight、写入丢弃、准入拒绝、淘汰和容量压力在重构后不可见。

## What Changes

- 审计 `aegiscore_localcache_*` 指标与现有 PromQL 资产，确认 dashboard、alert rule、metrics load 脚本和 runbook 是否遗漏 localcache 观测查询。
- 在 Grafana dashboard 中补充 localcache 请求命中率、回源成功/错误、singleflight shared/double-check、write-side set dropped/rejected、evictions 和 capacity 的 PromQL 面板或 target。
- 在 Prometheus alert rule 中补充 localcache 回源错误、准入拒绝/写入丢弃、淘汰压力等可行动告警，表达式只使用当前稳定指标名和固定 label。
- 更新真实 metrics load 脚本的服务端 metric presence 和 Prometheus sample query，确保运行验证能发现 localcache 指标缺失。
- 更新观测文档或 runbook，说明 localcache 指标含义和排障入口。
- **BREAKING**: 不保留旧 metric name、旧 label 或旧 PromQL 兼容查询；所有消费方直接迁移到当前 `aegiscore_localcache_*` contract。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 明确 localcache Prometheus 指标的消费资产必须覆盖重构后的全部稳定 metric family，并禁止通过兼容 PromQL 同时查询旧 metric 或旧 label。

## Impact

- 观测资产：`deployments/observability/grafana/user-service-overview.json`、`deployments/compose/grafana/dashboards/user-service-overview.json`、`deployments/observability/prometheus/user-service-alerts.yaml`。
- 验证脚本：`deployments/compose/scripts/generate-real-metrics-load.sh` 需要把 localcache metric family 纳入 presence check 和 Prometheus sample。
- 文档：`docs/observability/user-service-runbook.md` 或 `deployments/observability/README.md` 需要补充 localcache 指标解释和告警排障提示。
- 代码与测试：如审计发现 collector、注册或测试缺少稳定指标语义，需要调整 `common/runtime/observability/metrics/localcache.go`、`common/runtime/observability/metrics/localcache_test.go`、`user-service/internal/providers/metrics.go` 或 provider 路由测试；不新增兼容 alias。
- 部署：不改变 HTTP API、数据库 schema、OpenAPI、Ent migration 或服务启动参数；会改变 Prometheus/Grafana 观测输出和告警规则。
