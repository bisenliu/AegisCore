# Helm 部署

本目录承载 AegisCore 服务的 Helm charts。当前可用 chart 位于 `aegiscore-user-services/`，用于模板化 user-service 的 Kubernetes 运行时资源和发布前置 Job。

## Chart

| 路径 | 作用 |
|---|---|
| `aegiscore-user-services/Chart.yaml` | chart 元数据 |
| `aegiscore-user-services/values.yaml` | 生产基线默认值 |
| `aegiscore-user-services/values-local.yaml` | 本地或临时环境覆盖示例 |
| `aegiscore-user-services/templates/` | Deployment、Service、ConfigMap、Job、PDB、HPA、NetworkPolicy 等模板 |

## 发布顺序

chart 会渲染 migration Job、RBAC seed Job 和 HTTP Deployment，但 Helm 本身不保证这些资源按业务顺序等待完成。生产流水线必须显式编排：

1. 准备或更新 `secret.existingSecret` 指向的外部 Secret。
2. 渲染并执行 migration Job，等待成功。
3. 渲染并执行 RBAC seed Job，等待成功。
4. 执行 `helm upgrade --install` 滚动 HTTP Deployment，并在最终 rollout 阶段关闭 Job 渲染。

Deployment 默认不设置 `RUN_MIGRATIONS=true`，普通服务副本不执行 Atlas migration，user-service 运行时镜像不包含 Atlas。migration Job 使用 `migrationJob.image` 指向的独立 Atlas/migration 镜像。

## 验证

```bash
helm lint deployments/helm/aegiscore-user-services
helm template aegiscore-user-services deployments/helm/aegiscore-user-services \
  --values deployments/helm/aegiscore-user-services/values.yaml
```

渲染输出中应能看到：

- `Deployment` 的 `/livez`、`/readyz`、`/startupz` 探针。
- migration `Job` 的 `/atlas migrate apply --config file://migrations/atlas.hcl --env deploy` command 和独立 migration image。
- RBAC seed `Job` 的 `rbac seed --reactivate-system --sync-system-bindings` command。
- `PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`。

生产流水线如果用 `helm template` 分阶段应用 Job，最终执行 `helm upgrade --install` 时应追加：

```bash
--set migrationJob.enabled=false --set rbacSeedJob.enabled=false
```

## 观测资产

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`。当前 chart 不默认封装 ServiceMonitor、PodMonitor 或 Grafana dashboard。
