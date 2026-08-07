# Helm 部署

本目录承载 AegisCore 服务的 Helm charts。当前可用 chart 位于 `aegiscore-user-service/`，用于模板化 user-service 的 Kubernetes 运行时资源；RBAC seed 发布前置 Job 仅在受控 seed 阶段显式开启渲染。

## Chart

| 路径 | 作用 |
|---|---|
| `aegiscore-user-service/Chart.yaml` | chart 元数据 |
| `aegiscore-user-service/values.yaml` | 生产基线默认值 |
| `aegiscore-user-service/values-local.yaml` | 本地或临时环境覆盖示例 |
| `aegiscore-user-service/templates/` | Deployment、Service、Job、PDB、HPA、NetworkPolicy 等模板 |

## 发布顺序

Helm 默认只渲染最终 runtime manifest，不包含 RBAC seed Job。生产流水线必须显式编排：

1. 准备或更新 Nacos namespace/group/dataId 配置来源。
2. 确认本 release 对应的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行完成。
3. 使用同一不可变 `image.ref`、`--set rbacSeedJob.enabled=true` 和 release 唯一 `rbacSeedJob.nameSuffix` 渲染并执行 RBAC seed Job，等待成功。
4. 执行 `helm upgrade --install` 滚动 HTTP Deployment；最终 rollout 阶段不得开启 Job 渲染。

Deployment 默认不设置 `RUN_MIGRATIONS=true`，普通服务副本不执行 Atlas migration，user-service 运行时镜像不包含 Atlas。chart 不渲染自动执行 Atlas apply 的 migration Job。

chart 默认与 Distroless static nonroot 镜像对齐，`podSecurityContext.runAsUser`、`runAsGroup` 和 `fsGroup` 均为 `65532`。Deployment 保持 kubelet HTTP probes；Compose 场景才使用镜像内原生 `healthcheck` CLI。

运行配置只通过 `AEGISCORE_SERVICE` 和 `AEGISCORE_NACOS_*` 定位 Nacos，再加载 `base.yaml`、`resources.yaml`、`user-service.yaml` 等 dataId。时区使用 `runtime.timezone`。应用日志只写 stdout/stderr，tracing 启用后固定通过 OTLP 导出；trusted proxy 使用 Nacos `server.http.trusted_proxies` 配置真实入口代理 IP/CIDR，入口控制面必须覆盖或重建 forwarded headers。pprof 默认不进入 chart，通过修改 Nacos 中的 `observability.pprof`、loopback 和 `kubectl port-forward` 临时诊断。

## 验证

```bash
helm lint deployments/helm/aegiscore-user-service
helm template aegiscore-user-service deployments/helm/aegiscore-user-service \
  --values deployments/helm/aegiscore-user-service/values.yaml \
  --set-string image.ref=aegiscore-user-service:sha-0000000000000000000000000000000000000000
```

渲染输出中应能看到：

- `Deployment` 的 `/livez`、`/readyz`、`/startupz` 探针。
- 默认渲染结果不包含 RBAC seed `Job`；显式开启 `rbacSeedJob.enabled=true` 时，Job 使用 `rbac seed --reactivate-system --sync-system-bindings` command。
- Deployment 和 RBAC seed Job 的 `runAsUser`、`runAsGroup`、`fsGroup` 都为 `65532`。
- 渲染结果不包含自动执行 `atlas migrate apply` 的 migration Job、command 或 args。
- `PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`。

生产流水线如果用 `helm template` 分阶段应用 RBAC seed Job，seed 阶段应追加：

```bash
--set rbacSeedJob.enabled=true --set-string rbacSeedJob.nameSuffix=rbac-seed-<release-id>
```

最终 runtime 阶段保持默认 `rbacSeedJob.enabled=false`，并在应用前确认 manifest 不包含 `kind: Job`。

## 观测资产

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`。当前 chart 不默认封装 ServiceMonitor、PodMonitor 或 Grafana dashboard。
