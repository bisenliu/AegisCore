## Context

`add-rbac-datastore-observability` 已经在 permission feature 中注册并记录 `aegiscore_user_service_rbac_enforce_duration_seconds` histogram，标签为 `result`、`method`、`route_template`。当前 user-service Grafana dashboard 的 Auth 与 RBAC 区域只展示 `aegiscore_user_service_rbac_policy_sync_operations_total` 和 `aegiscore_user_service_rbac_enforce_total`，没有展示 RBAC Enforce latency 的 P95/P99 分位。

通用 dashboard 位于 `deployments/observability/grafana/user-service-overview.json`，Compose 版本由 `deployments/compose/scripts/generate-grafana-dashboard.sh` 改写 datasource uid 生成到 `deployments/compose/grafana/dashboards/user-service-overview.json`。本次变更只修改观测资产和 OpenSpec delta，不修改 Go 代码、HTTP API、数据库 migration、OpenAPI 生成物或安全边界。

## Goals / Non-Goals

**Goals:**

- 在 Auth 与 RBAC 区域新增 RBAC Enforce P95/P99 延迟面板。
- PromQL 直接消费 `aegiscore_user_service_rbac_enforce_duration_seconds_bucket`，按 `le`、`method`、`route_template`、`result` 聚合。
- 生成 Compose dashboard，并用 `make compose-dashboard-check` 防止 dashboard drift。
- 更新 `runtime-observability` delta，明确部署观测资产必须覆盖 RBAC Enforce latency histogram。

**Non-Goals:**

- 不新增或重命名 RBAC metrics。
- 不修改 RBAC 授权业务结果、Casbin policy sync、Redis/Ent 观测代码或告警规则。
- 不新增兼容旧指标名、旧 label 或 fallback PromQL。
- 不调整 dashboard 变量体系，继续复用 `service` 和 `environment` 变量。

## Decisions

### Decision: 在 Auth 与 RBAC 区域新增独立 latency 面板

选择在现有 Auth 与 RBAC row 下新增独立 timeseries 面板，而不是把 latency 查询混入“RBAC 策略与授权速率”面板。速率与延迟的单位、读图方式和聚合目标不同，独立面板可以直接显示 P95/P99 并保留 `method`、`route_template`、`result` 维度。

备选方案是扩展现有 RBAC 速率面板，但这会在同一 panel 中混合 counter rate 和 seconds quantile，降低可读性。

### Decision: 使用 histogram_quantile 聚合 P95/P99

RBAC Enforce latency 是 Prometheus histogram，dashboard 使用 `histogram_quantile(0.95|0.99, sum by (le, method, route_template, result) (rate(..._bucket[5m])))`。该查询只使用已有低基数标签，不引入用户 ID、角色 ID、权限 ID、raw path、trace/span ID 或原始错误。

备选方案是只展示 `_sum / _count` 平均耗时，但平均值不能定位慢尾延迟，不满足前次报告指出的 P95/P99 缺口。

### Decision: 只以通用 dashboard 为源，Compose dashboard 由脚本生成

直接修改 `deployments/observability/grafana/user-service-overview.json`，再运行 `make compose-dashboard-generate` 更新 Compose 生成物。这样保持 Compose 版本只改 datasource uid 的既有约束，并让 `make compose-dashboard-check` 继续作为 drift 检查。

备选方案是手工同步两个 JSON 文件，但容易造成 datasource uid 或面板结构漂移。

## Risks / Trade-offs

- [Risk] 新增面板 id 或 grid 布局与现有面板冲突 -> Mitigation：检查 dashboard JSON 面板 id 和布局，保持 id 唯一并让后续面板位置顺延。
- [Risk] PromQL 聚合维度过多导致序列数量增加 -> Mitigation：只使用指标已定义的低基数标签，不引入新变量或高基数字段。
- [Risk] 通用 dashboard 和 Compose dashboard 不一致 -> Mitigation：运行 `make compose-dashboard-generate` 后再运行 `make compose-dashboard-check`。
- [Risk] OpenSpec delta 与实现不一致 -> Mitigation：运行 `openspec validate add-rbac-enforce-latency-observability`，并在 tasks 中记录验证结果。

## Migration Plan

实施时先更新 OpenSpec artifacts，再修改通用 dashboard，随后生成 Compose dashboard。发布后 Grafana provisioning 重新加载 dashboard 即可看到新增面板；如需回滚，恢复本次 dashboard JSON 和 OpenSpec change 目录即可，不涉及数据库、API、配置迁移或 RBAC policy 数据变更。

## Open Questions

无。
