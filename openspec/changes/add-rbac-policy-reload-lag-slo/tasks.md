## 1. 指标接口与记录器

- [x] 1.1 在 `user-service/internal/features/permission/application/metrics.go` 增加 policy reload lag 记录方法，并重新生成 `metrics_nop_gen.go`。
- [x] 1.2 在 `user-service/internal/features/permission/metrics.go` 注册 `aegiscore_user_service_rbac_policy_reload_lag` gauge，确保不新增业务标签。
- [x] 1.3 更新 permission metrics 单元测试，验证 gauge 导出值、no-op 实现和低基数标签契约。

## 2. Watcher Lag 观测

- [x] 2.1 在 Redis watcher 中增加 lag 计算路径，统一使用 `max(remote_policy_version - local_applied_policy_version, 0)`。
- [x] 2.2 在 `CheckVersion` 成功读取 Redis version 后记录准确 lag，读取失败时只记录 check failure 且不清零 lag。
- [x] 2.3 在 Pub/Sub payload 处理与远端 version 成功应用后更新 lag，不改变 reload、缓存失效或 version tracker 控制流。
- [x] 2.4 更新 watcher 单元测试，覆盖 Pub/Sub 漏消息补偿、reload 成功后 lag 收敛、Redis version 读取失败不伪装为收敛。

## 3. 观测资产与文档

- [x] 3.1 在 Grafana dashboard 源文件新增 RBAC policy reload lag 面板，按实例展示当前 lag。
- [x] 3.2 同步更新 Compose provisioning dashboard，并运行 `make compose-dashboard-check` 验证无 drift。
- [x] 3.3 在 `deployments/observability/prometheus/user-service-alerts.yaml` 新增 `aegiscore_user_service_rbac_policy_reload_lag > 0 for 30s` 告警并指向 runbook。
- [x] 3.4 更新 `docs/observability/user-service-runbook.md`，说明 lag 指标含义、30 秒最终生效 SLO、影响和排障步骤。

## 4. 验证与收尾

- [x] 4.1 运行 permission/RBAC 相关 Go 测试，确认 metrics 和 watcher 行为通过。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认 RBAC metrics 仍留在 permission feature 边界内。
- [x] 4.3 将本次预期代码、观测资产、文档和 OpenSpec 变更加到暂存区。
- [x] 4.4 运行 `make lint` 并修复发现的问题。
- [x] 4.5 运行 `make verify` 并修复发现的问题，确保暂存状态不会阻塞最终 diff 校验。
