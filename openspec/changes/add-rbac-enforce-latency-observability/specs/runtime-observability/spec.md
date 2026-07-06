## ADDED Requirements

### Requirement: RBAC Enforce 延迟 dashboard

系统 MUST 在 user-service Grafana dashboard 中展示 RBAC Enforce latency histogram 的分位延迟，使 SRE 和开发者能够按授权结果、HTTP 方法和路由模板观察 RBAC 授权判定慢尾延迟。dashboard MUST 直接消费当前稳定的 `aegiscore_user_service_rbac_enforce_duration_seconds` metric family，并 MUST NOT 保留旧指标名、旧 label 或兼容 PromQL。

#### Scenario: 展示 RBAC Enforce P95 和 P99 延迟

- **WHEN** Grafana 加载 user-service overview dashboard
- **THEN** dashboard MUST 包含 RBAC Enforce P95/P99 延迟面板
- **AND** 面板 MUST 使用 `aegiscore_user_service_rbac_enforce_duration_seconds_bucket` 计算 `histogram_quantile(0.95, ...)` 和 `histogram_quantile(0.99, ...)`

#### Scenario: RBAC Enforce 延迟查询保持低基数

- **WHEN** dashboard 查询 RBAC Enforce latency histogram
- **THEN** PromQL MUST 只按 `le`、`method`、`route_template`、`result` 以及固定 `service`、`environment` 过滤条件聚合
- **AND** PromQL MUST NOT 引用用户 ID、角色 ID、权限 ID、会话 ID、token ID、trace/span ID、raw path、IP、SQL、Redis key 或原始错误

#### Scenario: Compose dashboard 同步 RBAC Enforce 延迟面板

- **WHEN** 执行 `make compose-dashboard-generate`
- **THEN** Compose Grafana dashboard MUST 包含与通用 dashboard 相同的 RBAC Enforce P95/P99 延迟面板和 PromQL
- **AND** 除 Prometheus datasource uid 外，Compose dashboard MUST 与通用 dashboard 保持结构一致
