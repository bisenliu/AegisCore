## Why

前次 Prometheus 指标梳理确认 `aegiscore_user_service_rbac_enforce_duration_seconds` 已在代码侧导出，但 user-service Grafana JSON 未消费该 histogram，导致线上只能看到 RBAC 授权速率和 allow/deny/error 分布，不能直接定位授权判定慢路由或慢异常。

本次变更补齐 RBAC Enforce latency 在部署观测资产中的展示，使 SRE 和开发者可以通过现有 dashboard 查看授权判定 P95/P99 延迟，并保持低基数标签和无兼容方案的最终 PromQL。

## What Changes

- 在 user-service 通用 Grafana dashboard 中新增 RBAC Enforce P95/P99 延迟面板，直接消费 `aegiscore_user_service_rbac_enforce_duration_seconds_bucket`。
- 同步生成 Compose Grafana dashboard，保持 datasource uid 差异之外的面板结构和 PromQL 一致。
- 更新 OpenSpec delta，要求部署观测资产必须覆盖 RBAC Enforce latency histogram 的分位展示。
- **BREAKING** 不保留旧指标名、旧 label 或兼容 PromQL；dashboard 直接消费当前稳定的 RBAC Enforce latency 指标和标签。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 扩展部署观测资产要求，要求 user-service Grafana dashboard 覆盖 RBAC Enforce latency histogram 分位，并继续遵守低基数标签约束。

## Impact

- 影响观测资产：`deployments/observability/grafana/user-service-overview.json` 与生成的 `deployments/compose/grafana/dashboards/user-service-overview.json`。
- 影响规格：新增 `openspec/changes/add-rbac-enforce-latency-observability/specs/runtime-observability/spec.md` delta。
- 影响验证：需要运行 `openspec validate add-rbac-enforce-latency-observability`、`make compose-dashboard-generate`、`make compose-dashboard-check` 和 `make user-service-architecture-lint`。
- 不影响外部 HTTP API、OpenAPI、数据库 schema、Atlas migration、RBAC 授权结果、metrics 注册代码或 Redis/Ent 观测代码。
