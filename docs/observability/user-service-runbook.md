# 用户服务可观测性 Runbook

本文档配合 `deployments/observability/` 下的看板和告警规则使用。它假设用户服务已经在配置的 metrics path 暴露 Prometheus metrics，默认路径为 `/metrics`。

<a id="metrics-endpoint-and-scrape-checks"></a>

## 指标端点与抓取检查

1. 确认 metrics 已启用：

```bash
AEGISCORE_OBSERVABILITY_METRICS_ENABLED=true make run-user-service
```

2. 检查 metrics endpoint：

```bash
curl -fsS http://localhost:8080/metrics | head
```

3. 在 Prometheus 中确认相关 collector 启用后能看到这些指标族：

```promql
http_server_requests_total{service="aegiscore-user-services"}
aegiscore_redis_up{service="aegiscore-user-services",resource="cache_redis"}
aegiscore_postgres_pool_open_connections{service="aegiscore-user-services",resource="user_db"}
aegiscore_runtime_component_running{service="aegiscore-user-services",resource="rbac_policy_watcher"}
```

如果抓取失败，先检查 target 地址、metrics path、服务端口、网络策略，以及用户服务是否确实以 metrics enabled 状态运行。Metrics endpoint 不经过 RBAC；请在部署网络边界、Ingress 策略、service mesh 或等价机制中保护暴露范围。

<a id="http-5xx-ratio"></a>

## HTTP 5xx 比例

看板面板：

- HTTP 请求速率
- HTTP 5xx 比例
- HTTP P95/P99 延迟
- Auth 失败原因
- RBAC policy 同步失败

PromQL：

```promql
sum(rate(http_server_requests_total{service="aegiscore-user-services",status_class="5xx"}[5m]))
/
sum(rate(http_server_requests_total{service="aegiscore-user-services"}[5m]))
```

第一批检查：

- 找出最受影响的 `method` 和 `route` 模板。
- 检查 `status>=500` 的日志，并用 `trace_id` 和 `span_id` 关联同一次请求。
- 查看 PostgreSQL、Redis、RBAC watcher 和 policy reload 面板，判断是否同时存在依赖故障。
- 回看最近的 deployment、migration、RBAC seed 或配置变更。

不要基于 raw path、请求体、用户 ID 或日志全文创建告警。

<a id="http-latency"></a>

## HTTP 延迟

看板面板：

- HTTP P95/P99 延迟
- 进行中的请求数
- PostgreSQL pool
- Redis ping 耗时
- Workerpool 队列和运行任务

PromQL：

```promql
histogram_quantile(
  0.95,
  sum by (le, route) (
    rate(http_server_request_duration_seconds_bucket{service="aegiscore-user-services"}[5m])
  )
)
```

第一批检查：

- 判断延迟是全局升高，还是只集中在特定 route 模板。
- 检查 `aegiscore_postgres_pool_wait_count_total` 是否显示连接池等待。
- 检查 `aegiscore_redis_ping_duration_seconds` 和 `aegiscore_redis_up`。
- 检查进行中的请求数是否出现流量或阻塞尖峰。
- 使用日志中的 `latency_ms`、`method`、`path`、`status`、`trace_id` 和 `span_id` 做请求级关联。

<a id="readiness-failure"></a>

## Readiness 失败

`/readyz` 会检查 PostgreSQL `user_db`、Redis `cache_redis`、Casbin policy 加载状态和 RBAC policy watcher 状态。

PromQL 依赖外部探测指标：

```promql
probe_success{job=~".*readyz.*"} == 0
```

第一批检查：

- 从失败探测所在的同一网络段请求 `/readyz`。
- 检查 PostgreSQL 和 Redis 面板。
- 检查 `aegiscore_runtime_component_running{resource="rbac_policy_watcher"}` 和 `aegiscore_runtime_component_last_error{resource="rbac_policy_watcher"}`。
- 检查启动阶段或最近 policy reload 附近的日志。

仅靠应用 `/metrics` 不能证明 `/readyz` 失败。需要 blackbox exporter、Kubernetes probe metrics 或等价外部探测。

<a id="postgresql-unavailable-or-pool-pressure"></a>

## PostgreSQL 不可用或连接池压力

看板面板：

- PostgreSQL 打开连接数
- PostgreSQL 使用中连接数
- PostgreSQL 空闲连接数
- PostgreSQL 等待速率

PromQL：

```promql
aegiscore_postgres_pool_open_connections{service="aegiscore-user-services",resource="user_db"}
rate(aegiscore_postgres_pool_wait_count_total{service="aegiscore-user-services",resource="user_db"}[5m])
```

第一批检查：

- 查看 `/readyz` 是否报告数据库依赖失败。
- 确认 `postgres.user_db` endpoint、凭据、TLS 和网络路径。
- 检查 migration 或 rollout 时机。
- 如果等待压力较高，继续查看请求延迟和数据库容量。
- 不要把 SQL 文本、DSN 或原始数据库错误放入 label。

<a id="redis-unavailable"></a>

## Redis 不可用

看板面板：

- Redis 可用状态
- Redis ping 耗时
- Redis ping 失败
- RBAC watcher 和 workerpool 面板

PromQL：

```promql
aegiscore_redis_up{service="aegiscore-user-services",resource="cache_redis"}
rate(aegiscore_redis_ping_failures_total{service="aegiscore-user-services",resource="cache_redis"}[5m])
```

第一批检查：

- 检查 `/readyz`。
- 确认 `redis.cache_redis` 地址和网络路径。
- 查看 auth session、RBAC policy watcher 和 scheduler lock 相关信号，因为它们可能依赖 Redis。
- 检查 Redis 连接错误日志。不要把 Redis key 或原始错误写入 metrics label。

<a id="rbac-watcher-stopped"></a>

## RBAC watcher 停止

看板面板：

- RBAC watcher 运行状态
- RBAC watcher 最近错误状态
- RBAC policy version mismatch
- Casbin policy reload 状态

PromQL：

```promql
aegiscore_runtime_component_running{service="aegiscore-user-services",resource="rbac_policy_watcher"}
aegiscore_runtime_component_last_error{service="aegiscore-user-services",resource="rbac_policy_watcher"}
```

第一批检查：

- 检查 Redis availability，因为 watcher 使用 policy version 和 Pub/Sub。
- 检查是否有 `rbac policy refresh subscribe failed`、channel close 或 reload failure 相关日志。
- 如果问题发生在运行副本上执行 `make seed-rbac` 或 `rbac assign-super-admin` 后，请滚动重启副本，或通过正式在线 policy refresh 路径触发刷新。
- 确认 `/readyz` 和 `/startupz` 状态。

<a id="casbin-policy-reload-failed"></a>

## Casbin policy reload 失败

看板面板：

- 按状态统计的 Casbin policy reload
- Casbin policy reload 最近状态
- RBAC policy 同步失败
- PostgreSQL 和 Redis 依赖面板

PromQL：

```promql
increase(aegiscore_casbin_policy_reloads_total{service="aegiscore-user-services",status="failure"}[10m])
aegiscore_casbin_policy_reload_last_status{service="aegiscore-user-services",status="failure"}
```

第一批检查：

- 检查 PostgreSQL `user_db` 健康状态，因为 policy reload 会读取角色、权限和绑定数据。
- 检查最近的 RBAC 管理操作。
- 检查 policy reload 错误日志；如果 reload 由 HTTP 操作触发，用 `trace_id` 关联请求链路。
- 确认 route diff 没有报告异常的 missing 或 stale 权限。

<a id="workerpool-failed-or-panicked"></a>

## Workerpool 失败或 panic

看板面板：

- Auth session purge workerpool failed/panicked 任务
- Workerpool queued/running/free/waiting 状态
- Redis 可用性

PromQL：

```promql
increase(aegiscore_workerpool_tasks_total{service="aegiscore-user-services",pool="auth_session_purge_pool",event=~"failed|panicked"}[10m])
```

第一批检查：

- 检查 Redis availability 和延迟。
- 检查 `logout_all` 流量是否升高。
- 检查 workerpool queue 和 waiting 面板是否显示饱和。
- 检查 detached session purge task 失败日志。不要把 session ID 或 Redis key 放入 alert label。

<a id="scheduler-job-failed"></a>

## Scheduler job 失败

看板面板：

- Scheduler failed/skipped/lock renew failed 事件
- Scheduler job 耗时
- 如果 job 使用分布式锁，查看 Redis availability

PromQL：

```promql
increase(aegiscore_scheduler_jobs_total{service="aegiscore-user-services",event="failed"}[10m])
increase(aegiscore_scheduler_jobs_total{service="aegiscore-user-services",event="lock_renew_failed"}[10m])
```

第一批检查：

- 使用固定低基数 `job` label 定位失败任务。
- 检查 scheduler duration，判断是否有长时间运行的任务。
- 如果 job 使用分布式锁，检查 Redis。
- 检查该 job 运行时段附近的日志。不要把用户 ID、token、Redis key 或原始错误文本加入 metric label。

<a id="tracing-and-log-correlation"></a>

## Tracing 和日志关联

本地默认 tracing 使用 `observability.tracing.exporter: none`。在这种模式下：

- 服务会创建标准 trace 和 span context。
- 日志可以包含 `trace_id` 和 `span_id`。
- Span 不会被导出。
- Jaeger、Tempo 或其他 trace UI 中不会出现这些 trace。

常用日志字段：

- `trace_id`
- `span_id`
- `user_id`
- `client_ip`
- `method`
- `path`
- `status`
- `latency_ms`
- `user_agent`，用于认证失败安全事件

如需后续增加 trace 可视化，先部署 OpenTelemetry Collector 和 trace backend，再配置：

```bash
AEGISCORE_OBSERVABILITY_TRACING_EXPORTER=otlp
AEGISCORE_OBSERVABILITY_TRACING_OTLP_ENDPOINT=<collector-host>:4317
```

本文档不定义 Collector、Tempo、Jaeger 或云厂商资源。
