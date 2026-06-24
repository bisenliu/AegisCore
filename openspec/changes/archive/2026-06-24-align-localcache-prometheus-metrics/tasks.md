## 1. 指标审计

- [x] 1.1 审计 `common/runtime/observability/metrics/localcache.go` 与 `common/runtime/localcache/types.go`，确认 `aegiscore_localcache_*` metric family、`cache`、`result`、`event` label 和 `Stats` 字段一一对应。
- [x] 1.2 审计 `user-service/internal/providers/metrics.go` 和相关 provider 测试，确认 `auth_token_version`、`rbac_user_roles` 两个缓存实例在 metrics 启用时注册 collector，metrics 禁用时不注册。
- [x] 1.3 审计 `deployments/observability/grafana/user-service-overview.json`、`deployments/observability/prometheus/user-service-alerts.yaml` 和 `deployments/compose/scripts/generate-real-metrics-load.sh`，列出缺失或漂移的 localcache PromQL。

## 2. Collector 与服务注册验证

- [x] 2.1 如发现 collector 漏导字段或 label 语义不完整，更新 `common/runtime/observability/metrics/localcache.go`，只使用当前稳定 metric family，不添加兼容 alias。
- [x] 2.2 补齐或调整 `common/runtime/observability/metrics/localcache_test.go`，用结构化 metric family 验证 requests、loads、singleflight、writes、evictions 和 capacity。
- [x] 2.3 补齐或调整 `user-service/internal/providers/health_test.go`、`routes_test.go` 或 metrics provider 相关测试，验证两个 user-service localcache collector 的服务端 `/metrics` 暴露。

## 3. Grafana 与 Prometheus 资产

- [x] 3.1 更新 `deployments/observability/grafana/user-service-overview.json`，新增或调整 localcache 面板，覆盖 hit/miss、loader success/error、singleflight shared/double-check、set dropped/rejected、evictions 和 capacity。
- [x] 3.2 更新 `deployments/observability/prometheus/user-service-alerts.yaml`，补充 localcache loader error、write-side drop/reject 和 eviction pressure 等可行动告警，并指向稳定 runbook。
- [x] 3.3 运行 `make compose-dashboard-generate`，同步 `deployments/compose/grafana/dashboards/user-service-overview.json` 生成产物。

## 4. 脚本与文档

- [x] 4.1 更新 `deployments/compose/scripts/generate-real-metrics-load.sh`，把 localcache metric family 加入服务端 presence check 和 Prometheus sample query。
- [x] 4.2 更新 `docs/observability/user-service-runbook.md` 或 `deployments/observability/README.md`，说明 localcache 指标含义、常见异常和排障顺序。
- [x] 4.3 确认所有 PromQL 只引用当前 `aegiscore_localcache_*` 指标、固定低基数 label 和当前缓存名，不保留旧 metric 或旧 label 兼容查询。

## 5. 验证

- [x] 5.1 运行 `go test ./common/runtime/observability/metrics ./user-service/internal/providers`，确认 collector 和 user-service metrics 注册测试通过。
- [x] 5.2 运行 `make compose-dashboard-check`，确认通用 dashboard 与 Compose 生成产物无 drift。
- [x] 5.3 本机存在 `promtool` 时运行 `promtool check rules deployments/observability/prometheus/user-service-alerts.yaml`，确认 Prometheus rules 可加载。
- [x] 5.4 运行 `openspec status --change align-localcache-prometheus-metrics`，确认 artifacts 与 apply-ready 状态正常。
