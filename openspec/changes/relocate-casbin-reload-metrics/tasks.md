## 1. 指标所有权迁移

- [x] 1.1 在 permission feature 中新增 Casbin reload recorder interface、no-op 实现和 Prometheus collector 实现，保持 `aegiscore_casbin_policy_reloads_total` 与 `aegiscore_casbin_policy_reload_last_success` 契约不变。
- [x] 1.2 修改 `permissionMetricsOptions` 和相关 Fx provider，改为提供 permission-owned reload recorder，并继续复用通用 metrics Provider 注册 collector。
- [x] 1.3 修改 Casbin Engine constructor、Engine 字段和调用方，使 Engine 只依赖 permission-owned reload recorder interface。
- [x] 1.4 更新 e2e fixture、单元测试和 mockgen source，全部改用 permission-owned no-op 或 mock recorder。

## 2. common 边界清理

- [x] 2.1 从 `common/runtime/observability/metrics/status.go` 删除 `ReloadMetrics`、`NopReloadMetrics`、`NewCasbinPolicyReloadMetrics`、Casbin reload collector 和 `aegiscore_casbin_*` 指标定义。
- [x] 2.2 保留并验证通用 Provider、label、component status collector 和 runtime metrics 其他测试不受影响。
- [x] 2.3 确认 `common/runtime/observability/metrics` 不再包含 Casbin、permission、role、RBAC 或 user-service 业务 metrics 语义。

## 3. 测试与门禁

- [x] 3.1 将原 common Casbin reload metrics 输出断言迁移到 permission feature 测试，覆盖成功、失败、last success gauge 和 metrics disabled no-op。
- [x] 3.2 增加 `user-service/scripts/architecture-lint.sh` 检查，禁止业务 metrics 语义回流 `common/runtime/observability/metrics`。
- [x] 3.3 更新 `user-service/scripts/architecture-lint-test.sh` fixture，覆盖新门禁的通过和失败场景。
- [x] 3.4 运行 permission 相关包测试和 common runtime metrics 测试，确认迁移后行为不变。

## 4. 验证与收尾

- [x] 4.1 运行 `make user-service-architecture-lint`，确认架构边界门禁通过。
- [x] 4.2 检查本 change 不产生 OpenAPI、Ent、migration、dashboard 或部署观测资产 drift；如有非预期 drift，修正后执行对应检查。
- [x] 4.3 将本次预期代码、测试、OpenSpec 和文档变更加到暂存区。
- [x] 4.4 运行 `make lint`，通过后再运行 `make verify`。
- [x] 4.5 根据实际完成情况把本文件对应 checkbox 更新为 `- [x]`，并确认 `openspec status --change "relocate-casbin-reload-metrics"` 为 apply-ready 或 complete。
