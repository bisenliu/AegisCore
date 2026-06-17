# Design

## Overview

本变更只增加部署和运维文档资产：

```text
deployments/observability/
  README.md
  grafana/user-service-overview.json
  prometheus/user-service-alerts.yaml

docs/observability/ 或 docs/OPERATIONS.md
  -> alert runbook and local verification guide
```

核心原则：

- 只消费已有 metrics、logs 和 tracing context，不改变应用代码。
- Dashboard 和 alert 资产归 `deployments/`，排障说明归 `docs/`。
- PromQL 只使用低基数 label：`service`、`environment`、`method`、`route`、`status_class`、`code`、`resource`、`pool`、`job`、`event`、`status`、`reason`、`operation`、`result`、`source`、`kind`。
- 不使用用户 ID、角色 ID、权限 ID、session ID、trace ID、raw path、Redis key、SQL、错误消息全文或日志全文。
- 本地 tracing 默认只通过日志中的 `trace_id`、`span_id` 关联请求；trace UI 需要部署 Collector 和后端后再扩展。

## Current State

已有观测信号：

- 用户服务启用 `observability.metrics.enabled: true` 时暴露配置化 metrics endpoint，默认 `/metrics`。
- `common/http/middleware.HTTPServerMetrics` 导出：
  - `http_server_requests_total`
  - `http_server_request_duration_seconds`
  - `http_server_in_flight_requests`
- Runtime dependency 指标包括：
  - PostgreSQL pool：`aegiscore_postgres_pool_*`
  - Redis ping：`aegiscore_redis_up`、`aegiscore_redis_ping_duration_seconds`、`aegiscore_redis_ping_failures_total`
  - Workerpool：`aegiscore_workerpool_*`
  - Scheduler：`aegiscore_scheduler_jobs_total`、`aegiscore_scheduler_job_duration_seconds`
  - Runtime component：`aegiscore_runtime_component_running`、`aegiscore_runtime_component_last_error`
  - Casbin policy reload：`aegiscore_casbin_policy_reloads_total`、`aegiscore_casbin_policy_reload_last_status`
- Feature-owned 业务指标包括：
  - Auth：`aegiscore_user_service_auth_operations_total`、`aegiscore_user_service_auth_token_version_mismatches_total`、`aegiscore_user_service_auth_session_purge_submit_failures_total`
  - Permission/RBAC：`aegiscore_user_service_rbac_policy_sync_operations_total`、`aegiscore_user_service_rbac_policy_version_mismatches_total`、`aegiscore_user_service_permission_route_diff`
- Go runtime/process 指标在 `observability.metrics.include_runtime: true` 时注册。
- Tracing runtime 支持本地 OTel SDK provider；`exporter: none` 生成 trace/span context 但不导出 span。

## Asset Layout

建议新增：

```text
deployments/observability/
  README.md
  grafana/
    user-service-overview.json
  prometheus/
    user-service-alerts.yaml

docs/observability/
  user-service-runbook.md
```

说明：

- `deployments/observability/README.md` 说明资产用途、导入方式、本地验证和目标环境接入注意事项。
- `grafana/user-service-overview.json` 使用 Grafana dashboard JSON，数据源通过变量配置，例如 `${DS_PROMETHEUS}` 或 datasource UID 变量。
- `prometheus/user-service-alerts.yaml` 使用 PrometheusRule 示例，便于 Kubernetes/Prometheus Operator 环境直接参考；非 Operator 环境可复制 `groups` 到普通 rules 文件。
- `docs/observability/user-service-runbook.md` 是告警 annotations 的稳定链接目标，包含每类告警的排障步骤。

不放入：

- `common/runtime/observability/metrics`，因为 dashboard/alert 是部署资产，不是 runtime primitive。
- `user-service/internal/features/*`，因为不改变业务或 feature 逻辑。
- `deployments/helm` chart templates，除非后续单独变更要求 chart 化。

## Dashboard Design

Dashboard 目标是第一屏看到用户服务运行状态，后续面板支持定位依赖和后台任务问题。

建议 dashboard 变量：

| Variable | Query / Values | Purpose |
|---|---|---|
| `service` | `label_values(http_server_requests_total, service)` | 默认选择用户服务 |
| `environment` | `label_values(http_server_requests_total{service="$service"}, environment)` | 区分环境 |
| `route` | `label_values(http_server_requests_total{service="$service",environment="$environment"}, route)` | 可选路由过滤 |
| `resource` | `label_values(aegiscore_redis_up{service="$service",environment="$environment"}, resource)` | 依赖资源过滤 |

变量查询需允许在缺少 HTTP traffic 时手动输入或默认 `aegiscore-user-services` / 当前配置中的服务名。

### HTTP RED

面板：

- Request rate：

```promql
sum by (method, route) (
  rate(http_server_requests_total{service="$service",environment="$environment"}[5m])
)
```

- 5xx ratio：

```promql
sum(rate(http_server_requests_total{service="$service",environment="$environment",status_class="5xx"}[5m]))
/
sum(rate(http_server_requests_total{service="$service",environment="$environment"}[5m]))
```

- P95 / P99 latency：

```promql
histogram_quantile(0.95,
  sum by (le, method, route) (
    rate(http_server_request_duration_seconds_bucket{service="$service",environment="$environment"}[5m])
  )
)
```

P99 使用 `0.99`。

- In-flight requests：

```promql
sum(http_server_in_flight_requests{service="$service",environment="$environment"})
```

### Error Code And Auth/RBAC

如果当前 HTTP RED 尚未带 `code` label，错误码面板优先使用 feature-owned auth/RBAC result 指标，并在 README 标注 HTTP error code 面板依赖未来稳定 `code` label 或上游记录规则。

面板：

- Auth failures by reason：

```promql
sum by (operation, reason) (
  rate(aegiscore_user_service_auth_operations_total{service="$service",environment="$environment",result="failure"}[5m])
)
```

- RBAC policy sync failures：

```promql
sum by (operation, source, reason) (
  rate(aegiscore_user_service_rbac_policy_sync_operations_total{service="$service",environment="$environment",result="failure"}[5m])
)
```

- Route diff latest：

```promql
aegiscore_user_service_permission_route_diff{service="$service",environment="$environment"}
```

### Runtime Dependencies

PostgreSQL:

```promql
aegiscore_postgres_pool_open_connections{service="$service",environment="$environment",resource="user_db"}
aegiscore_postgres_pool_in_use_connections{service="$service",environment="$environment",resource="user_db"}
aegiscore_postgres_pool_idle_connections{service="$service",environment="$environment",resource="user_db"}
rate(aegiscore_postgres_pool_wait_count_total{service="$service",environment="$environment",resource="user_db"}[5m])
```

Redis:

```promql
aegiscore_redis_up{service="$service",environment="$environment",resource="cache_redis"}
aegiscore_redis_ping_duration_seconds{service="$service",environment="$environment",resource="cache_redis"}
rate(aegiscore_redis_ping_failures_total{service="$service",environment="$environment",resource="cache_redis"}[5m])
```

Workerpool:

```promql
aegiscore_workerpool_queued{service="$service",environment="$environment",pool="auth_session_purge_pool"}
aegiscore_workerpool_running{service="$service",environment="$environment",pool="auth_session_purge_pool"}
increase(aegiscore_workerpool_tasks_total{service="$service",environment="$environment",pool="auth_session_purge_pool",event=~"failed|panicked"}[15m])
```

Scheduler:

```promql
sum by (job, event) (
  increase(aegiscore_scheduler_jobs_total{service="$service",environment="$environment",event=~"failed|skipped|lock_renew_failed"}[15m])
)
```

RBAC watcher and policy reload:

```promql
aegiscore_runtime_component_running{service="$service",environment="$environment",resource="rbac_policy_watcher"}
aegiscore_runtime_component_last_error{service="$service",environment="$environment",resource="rbac_policy_watcher"}
increase(aegiscore_casbin_policy_reloads_total{service="$service",environment="$environment",status="failure"}[15m])
aegiscore_casbin_policy_reload_last_status{service="$service",environment="$environment",status="failure"}
```

Go runtime:

- `go_goroutines`
- `go_memstats_heap_alloc_bytes`
- `go_memstats_gc_cpu_fraction`
- `process_cpu_seconds_total`
- `process_resident_memory_bytes`
- `process_open_fds` if available on the platform

Runtime panels should tolerate absent series because `include_runtime` can be disabled.

## Alert Design

PrometheusRule 使用一个 group，例如 `aegiscore-user-service.rules`。Labels 建议：

```yaml
labels:
  service: aegiscore-user-services
  severity: warning|critical
```

Annotations：

- `summary`：一句话说明。
- `description`：包含当前表达式语义和第一步检查。
- `runbook_url`：指向 `docs/observability/user-service-runbook.md#...`。

### Required Alerts

5xx ratio:

```promql
(
  sum(rate(http_server_requests_total{service="aegiscore-user-services",status_class="5xx"}[5m]))
  /
  sum(rate(http_server_requests_total{service="aegiscore-user-services"}[5m]))
) > 0.05
```

实现时需处理分母为 0 的情况，可使用 `clamp_min(..., 1)` 或记录规则。

P95 latency:

```promql
histogram_quantile(0.95,
  sum by (le) (
    rate(http_server_request_duration_seconds_bucket{service="aegiscore-user-services"}[5m])
  )
) > 1
```

阈值第一版可保守设为 1s warning、3s critical，文档说明需按真实 SLO 调整。

Readyz failure:

- 如果目标环境采集 blackbox/probe 指标，优先使用 `probe_success{job=~".*readyz.*"} == 0`。
- 如果只 scrape `/metrics`，不能从应用指标直接证明 `/readyz` 失败；PrometheusRule 示例应把该 alert 标为需要接入 blackbox exporter 或 kube probe metrics。

PostgreSQL unavailable / pressure:

当前没有 `postgres_up` 指标。第一版可用 pool pressure 告警：

```promql
aegiscore_postgres_pool_open_connections{service="aegiscore-user-services",resource="user_db"} == 0
```

并增加 wait pressure：

```promql
rate(aegiscore_postgres_pool_wait_count_total{service="aegiscore-user-services",resource="user_db"}[5m]) > 0
```

文档需说明真正 unavailable 通常会同时体现在 `/readyz` failure 和日志中。

Redis unavailable:

```promql
aegiscore_redis_up{service="aegiscore-user-services",resource="cache_redis"} == 0
```

RBAC watcher stopped:

```promql
aegiscore_runtime_component_running{service="aegiscore-user-services",resource="rbac_policy_watcher"} == 0
```

Policy reload failed:

```promql
increase(aegiscore_casbin_policy_reloads_total{service="aegiscore-user-services",status="failure"}[10m]) > 0
```

Workerpool panicked/failed:

```promql
increase(aegiscore_workerpool_tasks_total{service="aegiscore-user-services",pool="auth_session_purge_pool",event=~"failed|panicked"}[10m]) > 0
```

Scheduler job failed:

```promql
increase(aegiscore_scheduler_jobs_total{service="aegiscore-user-services",event="failed"}[10m]) > 0
```

可选补充：

- Redis ping latency high。
- PostgreSQL pool wait pressure sustained。
- RBAC policy version mismatch sustained。
- Route diff missing/stale non-zero。

## Runbook Design

Runbook 需要短、可执行、稳定链接。建议章节：

- Metrics endpoint and scrape checks。
- HTTP 5xx ratio。
- HTTP latency。
- Readiness failure。
- PostgreSQL unavailable or pool pressure。
- Redis unavailable。
- RBAC watcher stopped。
- Casbin policy reload failed。
- Workerpool failed or panicked。
- Scheduler job failed。
- Tracing and log correlation。

每个章节包含：

- 看哪些 dashboard 面板。
- 查询哪些 PromQL。
- 查哪些日志字段：`trace_id`、`span_id`、`user_id`、`method`、`path`、`status`、`latency_ms`。
- 第一批处理建议，例如检查 deployment rollout、Redis/PostgreSQL availability、RBAC seed/reload 操作、workerpool queue pressure。
- 不应做的事，例如不要通过日志全文或高基数 label 建 alert。

Tracing 章节明确：

- 本地默认 `observability.tracing.exporter: none` 不会导出 span。
- 这种模式仍可让日志附带有效 `trace_id` / `span_id`，用于关联同一次请求。
- 需要 trace UI 时，部署 OTel Collector 和后端，并将 `observability.tracing.exporter=otlp`、`observability.tracing.otlp_endpoint` 指向 Collector。
- 本变更不提供 Collector、Tempo、Jaeger 或云厂商资源。

## Validation

本变更无需运行 Go 测试，但需要验证资产格式：

```bash
jq empty deployments/observability/grafana/user-service-overview.json
promtool check rules deployments/observability/prometheus/user-service-alerts.yaml
```

如果本机没有 `promtool`，可至少用 YAML parser 验证结构，并在最终说明未运行 `promtool`。

本地或目标环境验证：

1. 启用 `observability.metrics.enabled: true` 并启动用户服务。
2. 确认 Prometheus target 能 scrape metrics path。
3. 在 Prometheus expression browser 执行 README 中列出的关键 PromQL。
4. 导入 Grafana dashboard，确认变量能选中 `service` / `environment`。
5. 加载 alert rule，使用 `promtool test rules` 或临时降低阈值验证触发路径。

## Risks / Trade-offs

- `readyz` failure alert 需要 blackbox exporter、kube probe 指标或额外 scrape job；应用自身 metrics 不能完整替代 readiness probe。
- PostgreSQL 当前没有专门 `up` 指标；pool 指标和 readyz/logs 组合可以覆盖第一版排障，但严格 unavailable 告警依赖 readiness 或未来 DB ping 指标。
- Dashboard JSON 里的 datasource UID 在不同 Grafana 实例可能不同，因此应使用变量或默认 Prometheus datasource。
- 第一版阈值是保守默认，需要目标环境根据真实流量和 SLO 调整。
- Go runtime/process 指标可被配置关闭，相关面板需要对缺失 series 友好。
