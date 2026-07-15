## Why

user-service 默认 Fx `runtime.lifecycle.stop_timeout` 为 120 秒，但原生 Kubernetes 与 Helm 的 `terminationGracePeriodSeconds` 仅为 35 秒，Pod 在 Fx 逆序串行执行全部 `OnStop` hook 的总预算耗尽前就可能被强制 `SIGKILL`。需要统一应用与部署的终止预算契约并以自动校验防止漂移，保障滚动发布、缩容、驱逐和内部故障退出都能完成有限时优雅关闭。

## What Changes

- 盘点 user-service 默认配置中 Fx 总停止预算，以及 HTTP、auth session purge workerpool、watcher、tracing、datastore、Redis 和 logger 等生命周期组件的关闭预算与逆序执行关系。
- 将原生 Kubernetes manifest 与 Helm 默认 `terminationGracePeriodSeconds` 统一调整为 150 秒，使其严格大于 120 秒 Fx Stop 总预算，并明确记录 30 秒平台余量用于进程调度、`preStop` 和网络抖动。
- 增加可由 CI 或架构检查调用的配置一致性测试，同时解析 `user-service/configs/config.yaml`、原生 Kubernetes deployment 和 Helm values；当部署 grace 不满足“Fx Stop 总预算 + 平台余量”或两个部署默认值漂移时稳定失败。
- 更新部署注释和相关交付说明，明确组件级 hook timeout 不能替代 Fx 全部逆序串行 `OnStop` hook 的总预算，也不得通过并行执行 hook 绕开 Fx 停止语义。
- 将已验证的 `fix-user-service-internal-shutdown` 归档作为实施前置条件；本 change 不代替或隐式完成其归档。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`: 规定 Kubernetes 与 Helm 默认终止宽限期必须严格覆盖 Fx Stop 总预算和已记录的平台余量，并由跨配置自动校验阻止默认值漂移。
- `runtime-observability`: 明确 user-service 关闭链路的总预算与组件预算边界，以及滚动发布、缩容、驱逐和内部故障退出时完成可观测优雅关闭的行为。

## Impact

- 受影响配置和部署资产：`user-service/configs/config.yaml`、`deployments/k8s/user-services/deployment.yaml`、`deployments/helm/aegiscore-user-services/values.yaml` 及相关部署说明。
- 受影响测试和交付入口：新增或扩展部署配置一致性测试，并接入现有 CI 或 `make user-service-architecture-lint` 可达的检查路径。
- 受影响规格：`delivery-operations` 与 `runtime-observability` 的关闭预算和发布行为要求。
- Kubernetes/Helm 默认 Pod 最长终止等待从 35 秒增加到 150 秒；正常快速关闭不必等待完整宽限期，但卡住的 Pod 会更晚被强制终止。
- 不改变业务 API、OpenAPI、认证/RBAC 语义、数据库 schema、Atlas migration、依赖版本或 Fx `OnStop` 逆序串行语义，也不新增 runtime migration 行为。
