# Helm 部署

本目录承载 AegisCore 服务的 Helm charts。当前可用 chart 位于 `aegiscore-user-services/`，用于模板化 user-service 的 Kubernetes 运行时资源和 RBAC seed 发布前置 Job。

## Chart

| 路径 | 作用 |
|---|---|
| `aegiscore-user-services/Chart.yaml` | chart 元数据 |
| `aegiscore-user-services/values.yaml` | 生产基线默认值 |
| `aegiscore-user-services/values-local.yaml` | 本地或临时环境覆盖示例 |
| `aegiscore-user-services/templates/` | Deployment、Service、ConfigMap、Job、PDB、HPA、NetworkPolicy 等模板 |

## 发布顺序

chart 会渲染 RBAC seed Job 和 HTTP Deployment，但 Helm 本身不保证这些资源按业务顺序等待完成。生产流水线必须显式编排：

1. 准备或更新 `secret.existingSecret` 指向的外部 Secret。
2. 确认本 release 对应的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行完成。
3. 渲染并执行 RBAC seed Job，等待成功。
4. 执行 `helm upgrade --install` 滚动 HTTP Deployment，并在最终 rollout 阶段关闭 Job 渲染。

Deployment 默认不设置 `RUN_MIGRATIONS=true`，普通服务副本不执行 Atlas migration，user-service 运行时镜像不包含 Atlas。chart 不渲染自动执行 Atlas apply 的 migration Job。

chart 默认与 Distroless static nonroot 镜像对齐，`podSecurityContext.runAsUser`、`runAsGroup` 和 `fsGroup` 均为 `65532`。Deployment 保持 kubelet HTTP probes；Compose 场景才使用镜像内原生 `healthcheck` CLI。

运行配置使用 `AEGISCORE_SERVER_*` 和服务声明的 `AEGISCORE_RESOURCES_*` 路径，时区使用平台 `TZ`。应用日志只写 stdout/stderr，tracing 启用后固定通过 OTLP 导出；trusted proxy 属于入口控制面。pprof 默认不进入 chart，通过 loopback 和 `kubectl port-forward` 临时诊断。

## 验证

```bash
helm lint deployments/helm/aegiscore-user-services
helm template aegiscore-user-services deployments/helm/aegiscore-user-services \
  --values deployments/helm/aegiscore-user-services/values.yaml
```

渲染输出中应能看到：

- `Deployment` 的 `/livez`、`/readyz`、`/startupz` 探针。
- RBAC seed `Job` 的 `rbac seed --reactivate-system --sync-system-bindings` command。
- Deployment 和 RBAC seed Job 的 `runAsUser`、`runAsGroup`、`fsGroup` 都为 `65532`。
- 渲染结果不包含自动执行 `atlas migrate apply` 的 migration Job、command 或 args。
- `PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`。

生产流水线如果用 `helm template` 分阶段应用 RBAC seed Job，最终执行 `helm upgrade --install` 时应追加：

```bash
--set rbacSeedJob.enabled=false
```

## 观测资产

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`。当前 chart 不默认封装 ServiceMonitor、PodMonitor 或 Grafana dashboard。
