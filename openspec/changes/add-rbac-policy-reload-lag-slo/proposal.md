## Why

当前 RBAC policy sync 采用 Redis Pub/Sub 快速路径与周期性 policy version 补偿实现最终一致。Pub/Sub 丢失、watcher 短暂不可用或 Redis 网络抖动时，非写入副本可能在补偿周期内继续使用旧 Casbin policy，现有 SLO、dashboard 和 alert 不能直接量化“权限变更最终生效延迟”。

## What Changes

- 明确 RBAC 在线写成功后的多副本 policy 最终生效延迟 SLO。
- 新增 RBAC policy reload lag 观测契约，用于表达本实例已应用 policy version 与 Redis 最新 policy version 的差值。
- 在 Grafana dashboard、Prometheus alert 和 runbook 中补充 lag 展示、告警和排障说明。
- 保持授权热路径不读取 Redis version，不引入强一致请求路径或兼容分支。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 补充在线 RBAC 写后多副本最终收敛 SLO 和 policy reload lag 行为要求。
- `runtime-observability`: 补充 RBAC policy reload lag 指标、dashboard、alert 与 runbook 的稳定观测契约。

## Impact

- 影响 `user-service/internal/features/permission/` 中 RBAC policy sync 指标接口、Prometheus recorder 和 Redis watcher 观测点。
- 影响 `deployments/observability/grafana/`、`deployments/compose/grafana/dashboards/` 和相关生成/校验流程。
- 影响 `deployments/observability/prometheus/user-service-alerts.yaml` 的 RBAC 告警规则。
- 影响 `docs/observability/user-service-runbook.md` 的 RBAC policy sync 排障说明。
- 不改变 HTTP API、OpenAPI、数据库 schema、Redis key schema、Casbin 授权模型或请求热路径行为。
