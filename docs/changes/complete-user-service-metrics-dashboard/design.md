# Design

## Overview

本变更只调整 Grafana dashboard 资产。实现时以 `deployments/observability/grafana/user-service-overview.json` 作为 canonical dashboard，再同步到 `deployments/compose/grafana/dashboards/user-service-overview.json`，保证部署基线和本地 compose 体验一致。

设计目标：

- 覆盖当前已导出的 metrics，不设计新的指标契约。
- 面板按故障定位路径组织：入口流量、业务安全、依赖、后台任务、运行时。
- 同一面板内尽量保持单位一致；不同单位需要同时观察时使用 stat 或独立面板。
- Legend alias 使用稳定中文业务语义加必要 label，例如 `{{method}} {{route}}`、`{{operation}} / {{reason}}`。
- 使用统一 timeseries 风格：线条宽度、fill、tooltip、legend placement、no data 文案和 decimal 设置保持一致。
- 使用 threshold/value mapping 表达健康状态，不依赖颜色之外的信息。

## Current Metrics Inventory

当前 dashboard 应覆盖以下已存在指标。

HTTP:

- `http_server_requests_total`
- `http_server_request_duration_seconds`
- `http_server_in_flight_requests`

PostgreSQL:

- `aegiscore_postgres_pool_open_connections`
- `aegiscore_postgres_pool_in_use_connections`
- `aegiscore_postgres_pool_idle_connections`
- `aegiscore_postgres_pool_wait_count_total`
- `aegiscore_postgres_pool_wait_duration_seconds_total`
- `aegiscore_postgres_pool_max_open_connections`

Redis:

- `aegiscore_redis_up`
- `aegiscore_redis_ping_duration_seconds`
- `aegiscore_redis_ping_failures_total`

Workerpool:

- `aegiscore_workerpool_tasks_total`
- `aegiscore_workerpool_queued`
- `aegiscore_workerpool_running`
- `aegiscore_workerpool_waiting`

Scheduler:

- `aegiscore_scheduler_jobs_total`
- `aegiscore_scheduler_job_duration_seconds`

Runtime component and Casbin:

- `aegiscore_runtime_component_running`
- `aegiscore_runtime_component_last_error`
- `aegiscore_casbin_policy_reloads_total`
- `aegiscore_casbin_policy_reload_last_success`

Auth:

- `aegiscore_user_service_auth_operations_total`
- `aegiscore_user_service_auth_token_version_mismatches_total`
- `aegiscore_user_service_auth_session_purge_submit_failures_total`

Permission/RBAC:

- `aegiscore_user_service_rbac_policy_sync_operations_total`
- `aegiscore_user_service_rbac_policy_version_mismatches_total`
- `aegiscore_user_service_permission_route_diff`

Go runtime/process:

- `go_goroutines`
- `go_memstats_heap_alloc_bytes`
- `go_memstats_gc_cpu_fraction`
- `process_cpu_seconds_total`
- `process_resident_memory_bytes`
- `process_open_fds`

Go runtime/process 指标依赖 `observability.metrics.include_runtime: true`，面板必须能接受 no data。

## Dashboard Variables

保留或调整以下变量：

| Variable | Type | Query / Default | Notes |
|---|---|---|---|
| `datasource` | datasource | Prometheus | 便于导入到不同 Grafana 实例 |
| `service` | query/custom | `label_values(http_server_requests_total, service)`，默认 `aegiscore-user-services` | traffic 尚未产生时仍允许手动选择 |
| `environment` | query/custom | `label_values(http_server_requests_total{service="$service"}, environment)` | 支持 local/test/prod |
| `route` | query | `label_values(http_server_requests_total{service="$service",environment="$environment"}, route)` | include all，multi-select |
| `scheduler_job` | query | `label_values(aegiscore_scheduler_jobs_total{service="$service",environment="$environment"}, scheduler_job)` | include all，multi-select |

变量查询只使用低基数 label，不新增 raw path 或业务实体 ID。

## Layout

建议 row 分组：

1. `HTTP RED`
2. `Auth 与 RBAC`
3. `Runtime Dependencies`
4. `Background Jobs`
5. `Go Runtime`
6. `Dashboard Notes`

每组优先使用 24 栅格宽度下的 3 列或 2 列布局。关键 stat 放在组首，趋势和表格放在后续行。

## Panel Plan

### HTTP RED

请求速率：

```promql
sum by (method, route) (
  rate(http_server_requests_total{service="$service",environment="$environment",route=~"$route"}[5m])
)
```

Legend：`{{method}} {{route}}`，单位 `reqps`。

5xx 比例：

```promql
sum(rate(http_server_requests_total{service="$service",environment="$environment",status_class="5xx",route=~"$route"}[5m]))
/
clamp_min(sum(rate(http_server_requests_total{service="$service",environment="$environment",route=~"$route"}[5m])), 1)
```

单位 `percentunit`，threshold 建议 2% warning、5% critical。

状态分类分布：

```promql
sum by (status_class) (
  rate(http_server_requests_total{service="$service",environment="$environment",route=~"$route"}[5m])
)
```

可用 bar gauge 或 timeseries，legend：`{{status_class}}`。

P95/P99 延迟：

```promql
histogram_quantile(0.95,
  sum by (le, method, route) (
    rate(http_server_request_duration_seconds_bucket{service="$service",environment="$environment",route=~"$route"}[5m])
  )
)
```

P99 将 quantile 改为 `0.99`。Legend 需要区分 quantile，例如 `P95 {{method}} {{route}}`、`P99 {{method}} {{route}}`。

In-flight：

```promql
sum(http_server_in_flight_requests{service="$service",environment="$environment",route=~"$route"})
```

使用 stat，单位 `short`。

错误明细表：

```promql
sum by (status_class, method, route) (
  rate(http_server_requests_total{service="$service",environment="$environment",status_class=~"4xx|5xx",route=~"$route"}[5m])
)
```

表格列使用 `status_class`、`method`、`route`、`Value`，Value 单位 `reqps`。

### Auth 与 RBAC

Auth 操作结果：

```promql
sum by (operation, result) (
  rate(aegiscore_user_service_auth_operations_total{service="$service",environment="$environment"}[5m])
)
```

Auth 失败原因：

```promql
sum by (operation, reason) (
  rate(aegiscore_user_service_auth_operations_total{service="$service",environment="$environment",result="failure"}[5m])
)
```

Auth token version mismatch：

```promql
sum by (source) (
  rate(aegiscore_user_service_auth_token_version_mismatches_total{service="$service",environment="$environment"}[5m])
)
```

Auth session purge submit failure：

```promql
increase(aegiscore_user_service_auth_session_purge_submit_failures_total{service="$service",environment="$environment"}[15m])
```

RBAC policy sync result：

```promql
sum by (operation, source, result, reason) (
  rate(aegiscore_user_service_rbac_policy_sync_operations_total{service="$service",environment="$environment"}[5m])
)
```

RBAC policy version mismatch：

```promql
sum by (source) (
  rate(aegiscore_user_service_rbac_policy_version_mismatches_total{service="$service",environment="$environment"}[5m])
)
```

Permission route diff：

```promql
aegiscore_user_service_permission_route_diff{service="$service",environment="$environment"}
```

Legend：`{{kind}}`，stat 或 bar gauge，missing/stale 需要明确显示。

### Runtime Dependencies

PostgreSQL 连接池数量：

```promql
aegiscore_postgres_pool_open_connections{service="$service",environment="$environment",resource="user_db"}
aegiscore_postgres_pool_in_use_connections{service="$service",environment="$environment",resource="user_db"}
aegiscore_postgres_pool_idle_connections{service="$service",environment="$environment",resource="user_db"}
aegiscore_postgres_pool_max_open_connections{service="$service",environment="$environment",resource="user_db"}
```

Legend：`open`、`in use`、`idle`、`max`。

PostgreSQL 使用率：

```promql
aegiscore_postgres_pool_in_use_connections{service="$service",environment="$environment",resource="user_db"}
/
clamp_min(aegiscore_postgres_pool_max_open_connections{service="$service",environment="$environment",resource="user_db"}, 1)
```

单位 `percentunit`，threshold 建议 70% warning、90% critical。

PostgreSQL 等待压力拆成两个序列或两个面板：

```promql
rate(aegiscore_postgres_pool_wait_count_total{service="$service",environment="$environment",resource="user_db"}[5m])
rate(aegiscore_postgres_pool_wait_duration_seconds_total{service="$service",environment="$environment",resource="user_db"}[5m])
```

`wait_count` 单位 `ops`，`wait_duration` 单位 `s` 或 `s/s`，不要在 legend 中只显示原始 metric name。

Redis 可用性：

```promql
aegiscore_redis_up{service="$service",environment="$environment",resource="cache_redis"}
```

使用 stat，value mapping：`1 = UP`，`0 = DOWN`。

Redis ping latency：

```promql
aegiscore_redis_ping_duration_seconds{service="$service",environment="$environment",resource="cache_redis"}
```

单位 `s`。

Redis ping failure rate：

```promql
rate(aegiscore_redis_ping_failures_total{service="$service",environment="$environment",resource="cache_redis"}[5m])
```

单位 `ops`。

### Background Jobs

Workerpool 队列状态：

```promql
aegiscore_workerpool_queued{service="$service",environment="$environment",pool="auth_session_purge_pool"}
aegiscore_workerpool_running{service="$service",environment="$environment",pool="auth_session_purge_pool"}
aegiscore_workerpool_waiting{service="$service",environment="$environment",pool="auth_session_purge_pool"}
```

Workerpool 任务事件：

```promql
sum by (event) (
  increase(aegiscore_workerpool_tasks_total{service="$service",environment="$environment",pool="auth_session_purge_pool"}[15m])
)
```

可以用 bar chart 或 table，突出 `rejected`、`failed`、`panicked`。

Scheduler 事件：

```promql
sum by (scheduler_job, event, status, reason) (
  increase(aegiscore_scheduler_jobs_total{service="$service",environment="$environment",scheduler_job=~"$scheduler_job"}[15m])
)
```

Scheduler P95 耗时：

```promql
histogram_quantile(0.95,
  sum by (le, scheduler_job, status) (
    rate(aegiscore_scheduler_job_duration_seconds_bucket{service="$service",environment="$environment",scheduler_job=~"$scheduler_job"}[5m])
  )
)
```

RBAC policy watcher 状态：

```promql
aegiscore_runtime_component_running{service="$service",environment="$environment",resource="rbac_policy_watcher"}
aegiscore_runtime_component_last_error{service="$service",environment="$environment",resource="rbac_policy_watcher"}
```

建议拆为两个 stat，避免 running 与 last error 使用同一个 threshold 语义。

Casbin policy reload：

```promql
sum by (status) (
  increase(aegiscore_casbin_policy_reloads_total{service="$service",environment="$environment"}[15m])
)
aegiscore_casbin_policy_reload_last_success{service="$service",environment="$environment"}
```

`last_success` 用 stat value mapping：`1 = success`，`0 = failure`。

### Go Runtime

保持可选运行时面板：

```promql
go_goroutines{service="$service",environment="$environment"}
go_memstats_heap_alloc_bytes{service="$service",environment="$environment"}
process_resident_memory_bytes{service="$service",environment="$environment"}
go_memstats_gc_cpu_fraction{service="$service",environment="$environment"}
rate(process_cpu_seconds_total{service="$service",environment="$environment"}[5m])
process_open_fds{service="$service",environment="$environment"}
```

内存使用单位 `bytes`，CPU 使用率使用 `percentunit` 或 `cores` 口径时需在标题中说明。

## Visual Style

统一设置建议：

- Timeseries 使用 line、line width 2、fill opacity 8-15、smooth 或 linear 保持全局一致。
- Legend 放在 bottom，calcs 至少展示 `lastNotNull`，需要对比时加 `max`。
- Tooltip 使用 multi，sort descending。
- `noValue` 统一为 `No data`。
- 关键异常面板 threshold：green 正常、orange 警告、red 严重。
- 表格面板开启 column rename，把 `Value` 改成业务语义，例如 `rate`、`count` 或 `latest`。
- 避免使用过多单色调；保持 Grafana 默认中性背景和 classic palette，stat threshold 色只表示状态。

## Synchronization

实现策略：

1. 先更新 `deployments/observability/grafana/user-service-overview.json`。
2. 使用复制或生成脚本同步到 `deployments/compose/grafana/dashboards/user-service-overview.json`。
3. 如现有 `deployments/compose/scripts/generate-grafana-dashboard.sh` 假设旧结构，应同步更新脚本或在任务中明确不再使用它生成该 dashboard。
4. 校验 Compose dashboard 由 canonical dashboard 正确生成：

```bash
make compose-dashboard-check
```

说明：通用 dashboard 使用 `${DS_PROMETHEUS}` 作为可导入 datasource uid，Compose provisioning 的 datasource uid 固定为 `prometheus`，所以两份 JSON 不做原始 `cmp`；以生成脚本的 `--check` 结果作为同步依据。

## Guardrails

- Dashboard 只引用已存在指标。
- PromQL 中不得使用 raw path、query、用户标识、角色标识、权限标识、session 标识、trace/span 标识、Redis key、SQL 或原始错误消息。
- 不在 `common/runtime/observability/metrics` 中加入 dashboard 或部署资产。
- 不修改应用 runtime、feature recorder、alert rules 或 runbook，除非实现时发现 dashboard 文档引用已经失效并需要小范围修正。
