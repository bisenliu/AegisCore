# Tasks

## 1. Preparation

- [x] 1.1 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、本 change 的 `proposal.md` 和 `design.md`。
- [x] 1.2 确认本 change 使用 `docs/changes/complete-user-service-metrics-dashboard/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 1.3 梳理当前 metrics 名称和 label，确认 dashboard 只引用已有指标。
- [x] 1.4 对比两份现有 dashboard，确认 canonical 文件和 compose 副本的同步方式。

## 2. Dashboard Inventory

- [x] 2.1 列出现有 dashboard 已覆盖的指标。
- [x] 2.2 标记缺失指标：PostgreSQL max-open、auth token version mismatch、auth session purge submit failure、RBAC policy version mismatch、workerpool task lifecycle、scheduler event reason、Casbin reload status details。
- [x] 2.3 标记需要拆分的混合单位面板，例如 Redis availability/latency/failure、PostgreSQL wait count/duration、RBAC watcher running/last error。
- [x] 2.4 确认 dashboard 变量仍只使用低基数 label。

## 3. Grafana Dashboard Update

- [x] 3.1 更新 `deployments/observability/grafana/user-service-overview.json`。
- [x] 3.2 保留或修正 datasource、service、environment、route、scheduler_job 变量。
- [x] 3.3 统一 dashboard row 分组：HTTP RED、Auth 与 RBAC、Runtime Dependencies、Background Jobs、Go Runtime、Dashboard Notes。
- [x] 3.4 优化 HTTP 面板标题、legend、单位和阈值。
- [x] 3.5 新增或调整 HTTP status class 分布和错误明细表。
- [x] 3.6 新增 Auth 操作结果、token version mismatch 和 session purge submit failure 面板。
- [x] 3.7 新增 RBAC policy sync result、policy version mismatch 和 route diff missing/stale 展示。
- [x] 3.8 新增 PostgreSQL max-open 和 pool usage ratio 展示。
- [x] 3.9 拆分 PostgreSQL wait count rate 与 wait duration rate 展示，或明确 dual-axis/override。
- [x] 3.10 拆分 Redis up、ping latency 和 ping failure rate 展示。
- [x] 3.11 补齐 workerpool submitted/rejected/started/completed/failed/panicked 事件展示。
- [x] 3.12 补齐 scheduler event/status/reason 展示，并保留 scheduler P95 duration。
- [x] 3.13 将 RBAC policy watcher running 和 last error 拆成状态语义清晰的面板。
- [x] 3.14 优化 Casbin policy reload 成败趋势和 latest status 展示。
- [x] 3.15 保持 Go runtime/process 面板在 include_runtime 关闭时 no data 可读。

## 4. Visual Consistency

- [x] 4.1 统一 timeseries draw style、line width、fill opacity、tooltip、legend placement 和 legend calcs。
- [x] 4.2 为每个 PromQL target 设置稳定 legend alias，避免直接暴露长 metric name。
- [x] 4.3 统一 stat 面板 value mapping 和 threshold。
- [x] 4.4 统一 table 面板列名、排序、单位和 decimals。
- [x] 4.5 统一 `No data` 文案、description 和 panel title 语气。
- [x] 4.6 避免同一面板混杂无法比较的单位；确实需要时使用 field override 或拆面板。

## 5. Compose Dashboard Sync

- [x] 5.1 将 canonical dashboard 同步到 `deployments/compose/grafana/dashboards/user-service-overview.json`。
- [x] 5.2 如 `deployments/compose/scripts/generate-grafana-dashboard.sh` 与新结构冲突，更新脚本或文档说明。
- [x] 5.3 使用 `make compose-dashboard-check` 确认 compose dashboard 是 canonical dashboard 的同步生成产物。

## 6. Validation

- [x] 6.1 校验 canonical dashboard JSON：

```bash
jq empty deployments/observability/grafana/user-service-overview.json
```

- [x] 6.2 校验 compose dashboard JSON：

```bash
jq empty deployments/compose/grafana/dashboards/user-service-overview.json
```

- [x] 6.3 校验 compose dashboard 是否由 canonical dashboard 同步生成：

```bash
make compose-dashboard-check
```

- [x] 6.4 扫描 dashboard PromQL，确认未引用不存在指标。
- [x] 6.5 扫描 dashboard PromQL，确认未使用高基数或敏感 label。
- [x] 6.6 可选启动本地 compose 或 Grafana，导入 dashboard 观察面板渲染；本次未启动本地 Grafana，已通过 JSON 解析和 compose 生成校验覆盖文件级验证。
- [x] 6.7 确认未修改应用 Go 代码。

## 7. Guardrails

- [x] 7.1 不新增、删除或重命名 Prometheus 指标。
- [x] 7.2 不修改 `common/runtime/observability/metrics`、HTTP middleware、feature metrics recorder 或 provider wiring。
- [x] 7.3 不修改 HTTP API、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 7.4 不新增云厂商特定资源。
- [x] 7.5 不新增 `openspec/` 或 `docs/opsx/`。
