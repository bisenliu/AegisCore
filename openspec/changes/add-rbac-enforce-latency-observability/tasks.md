## 1. OpenSpec 提案确认

- [x] 1.1 确认 `proposal.md`、`design.md` 和 `specs/runtime-observability/spec.md` 内容一致，并运行 `openspec validate add-rbac-enforce-latency-observability --strict`。
- [x] 1.2 确认 `openspec status --change add-rbac-enforce-latency-observability` 显示 proposal、design、specs、tasks 可用于 apply。

## 2. Grafana dashboard 实现

- [x] 2.1 更新 `deployments/observability/grafana/user-service-overview.json`，在 Auth 与 RBAC 区域新增 RBAC Enforce P95/P99 延迟 timeseries 面板。
- [x] 2.2 确认新增面板 PromQL 使用 `aegiscore_user_service_rbac_enforce_duration_seconds_bucket` 和 `histogram_quantile(0.95|0.99, ...)`，且只按 `le`、`method`、`route_template`、`result` 聚合。
- [x] 2.3 运行 `make compose-dashboard-generate`，同步更新 `deployments/compose/grafana/dashboards/user-service-overview.json`。

## 3. 验证

- [x] 3.1 运行 `make compose-dashboard-check`，确认通用 dashboard 与 Compose dashboard 无 drift。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认观测资产和 OpenSpec 变更未破坏架构边界。
- [x] 3.3 将本次预期 OpenSpec artifacts 和 dashboard 变更加到暂存区。
- [x] 3.4 运行 `make lint`，失败时修复后重新运行。
- [x] 3.5 运行 `make verify`，失败时修复 drift 或测试问题后重新运行。
- [x] 3.6 确认 `git diff --cached` 仅包含本 change 预期内容，未暂存 diff 不包含本 change 遗漏文件。
