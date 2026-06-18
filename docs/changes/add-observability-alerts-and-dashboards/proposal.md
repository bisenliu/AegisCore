# Add observability alerts and dashboards

## What

为用户服务已经落地的 logs、metrics 和 tracing 上下文补齐第一版运维 dashboard、Prometheus 告警规则和排障文档入口。

本变更新增部署与文档资产，不改变应用代码：

- 在 `deployments/` 下新增 Prometheus/Grafana 观测资产，优先提供 dashboard JSON、PrometheusRule 示例和 README。
- Dashboard 覆盖 HTTP RED、5xx/error code、P95/P99 latency、PostgreSQL pool、Redis availability、workerpool、scheduler、RBAC watcher、policy reload 和 Go runtime。
- Prometheus 告警覆盖 5xx ratio、P95 latency、readyz failure、PostgreSQL unavailable、Redis unavailable、RBAC watcher stopped、policy reload failed、workerpool panicked/failed 和 scheduler job failed。
- 每条告警提供稳定 runbook 链接或简短排障说明，指向仓库文档中的固定位置。
- 文档说明本地无 Collector 时 tracing 只提供日志关联；部署 Collector 后可扩展 trace 可视化。

## Why

用户服务已经具备 Prometheus metrics endpoint、HTTP RED 指标、runtime dependency 指标、auth/RBAC 业务指标和 OTel tracing context。当前缺口在部署侧：

- 生产排障没有统一 dashboard 入口，依赖人工搜索指标名。
- 告警规则尚未表达关键 SLO/依赖故障/后台任务/RBAC policy 同步风险。
- 运维人员需要明确哪些信号来自 metrics、哪些只能通过日志 trace/span 字段关联。
- 当前 tracing 本地默认 `exporter: none`，需要避免误以为无 Collector 环境会出现 trace UI。

第一版 dashboard 和 alert 资产可以把已有观测能力变成可执行的生产入口，同时保持应用代码和指标契约稳定。

## Scope

包括：

- 在 `deployments/observability/` 或同等部署边界下新增 README。
- 新增 Grafana dashboard JSON，使用现有 Prometheus 指标和低基数 label。
- 新增 PrometheusRule 示例 YAML，适用于 Prometheus Operator 类环境，但不绑定云厂商。
- 新增或更新仓库文档中的稳定 runbook 位置，例如 `docs/OPERATIONS.md` 或 `docs/observability/`，供告警 annotations 链接。
- Dashboard 至少覆盖：
  - HTTP request rate、error rate、duration P95/P99、in-flight requests。
  - 5xx 和稳定错误码趋势。
  - PostgreSQL `user_db` pool open/in-use/idle/wait。
  - Redis `cache_redis` up、ping duration、ping failures。
  - Auth session purge workerpool tasks、running、queued、failed、panicked。
  - Scheduler job failed、skipped、duration。
  - RBAC policy watcher running/last error、Casbin policy reload status。
  - Go runtime/process 指标，例如 goroutines、memory、GC、CPU 或 process open fds。
- 告警至少覆盖：
  - 5xx ratio。
  - HTTP P95 latency。
  - `/readyz` 探针失败。
  - PostgreSQL unavailable 或 pool pressure。
  - Redis unavailable。
  - RBAC watcher stopped。
  - Casbin policy reload failed。
  - Workerpool failed/panicked。
  - Scheduler job failed。
- 文档说明本地验证方法：scrape target、dashboard import、PromQL expression 和 alert rule 校验。

不包括：

- 不新增云厂商特定资源。
- 不要求当前仓库提供完整可生产运行的 Helm chart。
- 不改变应用代码、metrics middleware、collector、tracing provider、health probe 或日志输出。
- 不引入新的业务指标。
- 不修改 HTTP API、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- 新增 dashboard、alert 和 runbook 资产位于 `deployments/` 或相关 docs 位置，不进入 `common` 或 feature 代码目录。
- Dashboard 只使用当前已有指标名和低基数 label，不依赖 raw path、用户 ID、角色 ID、权限 ID、session ID、trace ID、Redis key、SQL 或日志全文。
- PrometheusRule 不依赖高基数 label 或日志全文。
- 每条告警都有 runbook 链接或简短排障说明，且链接指向仓库中的稳定文档位置。
- 文档说明本地无 Collector 时 tracing 只提供日志关联；部署 Collector 后可扩展 trace 可视化。
- 文档说明如何在本地或目标环境验证 scrape、dashboard 和 alert 表达式。
- PrometheusRule YAML 可被 `promtool check rules` 或等价工具校验。
- Grafana dashboard JSON 是有效 JSON，并能被 Grafana 导入或通过 JSON 工具解析。
- 未新增或修改应用 Go 代码。
