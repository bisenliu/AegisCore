# Complete user-service metrics dashboard

## What

补齐用户服务 Grafana dashboard 对当前已导出 Prometheus 指标的覆盖，并统一 `user-service-overview.json` 的面板主题、标题、图例和数据展示方式。

本变更聚焦部署观测资产，不改变应用代码：

- 更新 `deployments/observability/grafana/user-service-overview.json`，覆盖 `common/runtime/observability/metrics` 已导出的通用 runtime 指标和 user-service feature-owned 业务指标。
- 同步更新 `deployments/compose/grafana/dashboards/user-service-overview.json`，保持本地 compose 预置 dashboard 与部署基线一致。
- 补齐当前 dashboard 缺失的 PostgreSQL max-open/连接池使用率、auth token version mismatch、auth session purge submit failure、RBAC policy version mismatch、workerpool 全生命周期事件、scheduler 事件分布和 Casbin reload 成败细节。
- 将单位不同的指标拆成更清晰的面板，避免把 availability、latency、rate 和 counter increase 混在同一视觉尺度中。
- 统一面板标题、legend alias、tooltip、threshold、no data 文案、table 字段和 timeseries 样式，提升日常巡检和故障定位的可读性。

## Why

用户服务已经具备较完整的 metrics 导出能力，但 dashboard 展示还没有完全跟上：

- `common/runtime/observability/metrics/` 已提供 PostgreSQL、Redis、workerpool、scheduler、runtime component 和 Casbin policy reload 指标，其中部分指标没有出现在 dashboard 中。
- Auth 和 permission feature 已导出 token version mismatch、session purge submit failure、RBAC policy version mismatch 和 route diff 指标，但当前面板只覆盖其中一部分。
- 部分现有面板把不同单位的序列放在一起，值域差异明显，排障时不够直观。
- 标题和图例有中英文混用、语义不够稳定的问题，不利于团队形成统一读图习惯。
- `deployments/observability` 与 `deployments/compose` 里存在 dashboard 副本，后续变更需要明确同步策略，避免本地与部署基线漂移。

补齐 dashboard 可以让已有指标真正变成可操作的运行入口，同时保持指标契约、应用代码和部署边界不变。

## Scope

包括：

- 更新 `deployments/observability/grafana/user-service-overview.json`。
- 同步 `deployments/compose/grafana/dashboards/user-service-overview.json`。
- 保留低基数变量：datasource、service、environment、route、scheduler_job 等。
- 补齐以下指标族或派生展示：
  - HTTP RED：request rate、5xx ratio、status class 分布、P95/P99 latency、in-flight requests。
  - Auth：operation success/failure、failure reason、token version mismatch、session purge submit failure。
  - Permission/RBAC：policy sync result、policy version mismatch、route diff missing/stale、Casbin reload status。
  - PostgreSQL：open/in-use/idle/max-open、pool usage ratio、wait count rate、wait duration rate。
  - Redis：up、ping latency、ping failure rate。
  - Workerpool：queued/running/waiting、submitted/rejected/started/completed/failed/panicked。
  - Scheduler：triggered/started/completed/failed/skipped/lock_renew_failed、duration P95、failure/skip reason。
  - Runtime component：RBAC policy watcher running 和 last error。
  - Go runtime/process：goroutines、heap/RSS、GC CPU fraction、process CPU、open FDs。
- 优化 dashboard 展示：
  - 统一 row 分组和面板命名。
  - 为 PromQL target 设置稳定 legend alias。
  - 为 stat 面板设置清晰阈值和 value mappings。
  - 为 table 面板设置列名、排序、单位和小数位。
  - 对 runtime 可选指标保持 no data 可读。
- 如 dashboard 生成脚本需要同步调整，更新 `deployments/compose/scripts/generate-grafana-dashboard.sh` 或相关说明。

不包括：

- 不新增、删除或重命名 Prometheus 指标。
- 不修改 `common/runtime/observability/metrics`、HTTP middleware、feature metrics recorder 或 provider wiring。
- 不改变 alert rules、runbook、Grafana provisioning datasource 或 Prometheus scrape config，除非 implementation 发现必须修正 dashboard 引用路径。
- 不新增云厂商特定资源。
- 不修改 HTTP API、数据库 schema、Ent generated code、Atlas migration、Redis key schema 或应用 Go 代码。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- 两份 dashboard JSON 都是有效 JSON，可通过 `jq empty` 校验。
- `deployments/observability/grafana/user-service-overview.json` 与 `deployments/compose/grafana/dashboards/user-service-overview.json` 内容保持一致，或差异有明确注释和理由。
- Dashboard 覆盖 proposal 中列出的现有指标族，不引用尚未实现的指标名。
- Dashboard 不使用高基数或敏感 label，例如用户 ID、角色 ID、权限 ID、session ID、trace ID、raw path、Redis key、SQL 或原始错误消息。
- 单位不同的核心信号不再挤在同一个难以阅读的面板里；标题、图例和单位能够直接表达数据含义。
- 现有 Grafana datasource/变量机制保持可用，本地 compose 导入路径不被破坏。
- 未修改应用 Go 代码，未新增 `openspec/` 或 `docs/opsx/`。
