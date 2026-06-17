# Tasks

## 1. Preparation

- [x] 1.1 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、本 change 的 `proposal.md` 和 `design.md`。
- [x] 1.2 确认本 change 使用 `docs/changes/add-observability-alerts-and-dashboards/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 1.3 梳理当前 metrics 名称和 label 约束，确认 dashboard/alert 只使用已有指标。
- [x] 1.4 确认本变更不改变应用代码、不新增业务指标、不修改 health probe 或 tracing provider。

## 2. Deployment Asset Layout

- [x] 2.1 新增 `deployments/observability/README.md`。
- [x] 2.2 新增 `deployments/observability/grafana/`。
- [x] 2.3 新增 `deployments/observability/prometheus/`。
- [x] 2.4 在 README 中说明这些资产是示例/基线，不是完整可生产 Helm chart。
- [x] 2.5 确认没有新增云厂商特定资源。

## 3. Grafana Dashboard

- [x] 3.1 新增 `deployments/observability/grafana/user-service-overview.json`。
- [x] 3.2 添加 datasource、service、environment、route 等低基数变量。
- [x] 3.3 添加 HTTP request rate、5xx ratio、P95/P99 latency 和 in-flight 面板。
- [x] 3.4 添加 auth failure、RBAC policy sync failure 和 route diff 面板。
- [x] 3.5 添加 PostgreSQL `user_db` pool 面板。
- [x] 3.6 添加 Redis `cache_redis` availability、ping latency 和 failure 面板。
- [x] 3.7 添加 auth session purge workerpool queued/running/failed/panicked 面板。
- [x] 3.8 添加 scheduler failed/skipped/lock renew failed 和 duration 面板。
- [x] 3.9 添加 RBAC watcher running/last error 和 Casbin policy reload 面板。
- [x] 3.10 添加 Go runtime/process 面板，并让缺失 runtime series 时 dashboard 可读。
- [x] 3.11 确认 dashboard 不使用 raw path、用户 ID、角色 ID、权限 ID、session ID、trace ID、Redis key、SQL 或日志全文。

## 4. Prometheus Alert Rules

- [x] 4.1 新增 `deployments/observability/prometheus/user-service-alerts.yaml`。
- [x] 4.2 添加 5xx ratio alert，并处理请求总量为 0 的情况。
- [x] 4.3 添加 HTTP P95 latency alert。
- [x] 4.4 添加 readyz failure alert，并说明需要 blackbox exporter、kube probe metrics 或等价外部探测。
- [x] 4.5 添加 PostgreSQL unavailable 或 pool pressure alert。
- [x] 4.6 添加 Redis unavailable alert。
- [x] 4.7 添加 RBAC watcher stopped alert。
- [x] 4.8 添加 Casbin policy reload failed alert。
- [x] 4.9 添加 workerpool failed/panicked alert。
- [x] 4.10 添加 scheduler job failed alert。
- [x] 4.11 每条 alert 添加 `summary`、`description` 和 `runbook_url` annotation。
- [x] 4.12 确认 alert 表达式不依赖高基数 label 或日志全文。

## 5. Runbook Documentation

- [x] 5.1 新增 `docs/observability/user-service-runbook.md` 或等价稳定文档。
- [x] 5.2 记录 metrics endpoint、Prometheus scrape 和 dashboard 导入验证步骤。
- [x] 5.3 为 5xx ratio、latency、readyz failure、PostgreSQL、Redis、RBAC watcher、policy reload、workerpool 和 scheduler alert 编写简短排障步骤。
- [x] 5.4 说明日志关联字段：`trace_id`、`span_id`、`user_id`、`method`、`path`、`status`、`latency_ms`。
- [x] 5.5 说明本地 `exporter: none` 时 tracing 只提供日志关联，不提供 trace UI。
- [x] 5.6 说明部署 Collector 后可通过 OTLP exporter 扩展 trace 可视化，但本变更不提供 Collector 或后端资源。
- [x] 5.7 确认告警 rule 中的 runbook 链接与文档锚点一致。

## 6. Documentation Integration

- [x] 6.1 如有必要，更新 `docs/DEVELOPMENT.md` 的 observability 小节，链接 dashboard/alert/runbook 资产。
- [x] 6.2 如有必要，更新 `docs/ARCHITECTURE.md` 的 deployments 或 observability 描述，说明 dashboard/alert 属于部署资产。
- [x] 6.3 如有必要，更新 `deployments/k8s/README.md` 或 `deployments/helm/README.md`，指向 `deployments/observability/`。
- [x] 6.4 文档继续明确 metrics endpoint 不经过 RBAC，是否暴露由部署网络边界控制。

## 7. Validation

- [x] 7.1 校验 Grafana dashboard JSON：

```bash
jq empty deployments/observability/grafana/user-service-overview.json
```

- [x] 7.2 校验 PrometheusRule YAML：

```bash
promtool check rules deployments/observability/prometheus/user-service-alerts.yaml
```

- [x] 7.3 如果本机没有 `promtool`，使用可用 YAML parser 校验结构，并在结果中说明未运行 `promtool`。
- [x] 7.4 扫描 PromQL 和 JSON，确认未使用高基数或敏感 label。
- [x] 7.5 可选在本地 metrics enabled 环境执行关键 PromQL，确认表达式能返回或安全缺失。
- [x] 7.6 确认没有应用 Go 代码变更。

## 8. Guardrails

- [x] 8.1 不新增云厂商特定资源。
- [x] 8.2 不要求当前仓库提供完整可生产运行的 Helm chart。
- [x] 8.3 不改变应用代码。
- [x] 8.4 不引入新的业务指标。
- [x] 8.5 不把 dashboard、alert 或 runbook 放进 `common`、feature 代码目录或 `internal/shared`。
- [x] 8.6 不新增 `openspec/` 或 `docs/opsx/`。
