# aegiscore-user-services Helm Chart

本 chart 为 user-service 渲染云厂商无关的 Kubernetes 资源，包括 HTTP Deployment、Service、ConfigMap、ServiceAccount、RBAC、RBAC seed Job、PDB、HPA 和 NetworkPolicy。

## Values

| 值 | 作用 |
|---|---|
| `image.repository`、`image.tag` | user-service 发布镜像 |
| `config` | 非敏感 `AEGISCORE_*` 运行时配置 |
| `secret.existingSecret` | 外部 Secret 名称；chart 只引用不渲染真实 Secret |
| `secret.keys.*` | Secret 键名映射 |
| `deployment.terminationGracePeriodSeconds` | Fx Stop 总预算与平台余量对应的 Pod 终止宽限期 |
| `resources` | HTTP 副本 requests/limits |
| `probes` | `/livez`、`/readyz`、`/startupz` 探针配置 |
| `autoscaling` | HPA 配置 |
| `pdb` | PodDisruptionBudget 配置 |
| `networkPolicy` | ingress/egress 网络意图 |
| `rbacSeedJob` | RBAC seed Job 配置 |

默认 `values.yaml` 不包含真实敏感值，也不设置 `RUN_MIGRATIONS=true`。普通 user-service 镜像不包含 Atlas，chart 不渲染自动执行 Atlas apply 的 migration Job。生产环境必须提前创建 `secret.existingSecret` 指向的 Secret，并确保数据库 SQL migration 已通过 DBA 工单或受控发布平台执行完成。

默认 `podSecurityContext.runAsUser`、`runAsGroup` 和 `fsGroup` 均为 `65532`，与 Distroless static nonroot 运行时镜像身份一致。Deployment 和 RBAC seed Job 使用同一数值身份，保持只读根文件系统、禁止提权、capabilities drop 和 RuntimeDefault seccomp；Kubernetes 探针继续由 kubelet 通过 HTTP 访问 `/livez`、`/readyz`、`/startupz`。

默认 `deployment.terminationGracePeriodSeconds` 为 150 秒，其中 120 秒覆盖 Fx `app.Stop()` 对全部 `OnStop` hook 的逆注册顺序串行总预算，30 秒保留给 kubelet 调度、信号传递、受控 `preStop` 和网络抖动。HTTP、workerpool 或 exporter 的局部 timeout 不能替代 Fx 总预算；正常关闭完成后 Pod 会立即退出，不会等待完整 150 秒。环境 values 覆盖该值或新增、延长 `preStop` 时，发布方必须保持 grace 至少覆盖 Fx Stop 总预算和 30 秒平台余量，并同步验证原生 Kubernetes 与 Helm 的默认契约未漂移。

## NetworkPolicy 安全边界

默认 `networkPolicy` 仅允许 `ingress-system` namespace 中带 `aegiscore.io/allow-user-service: "true"` 的受控上游访问 user-service HTTP 端口，并将 PostgreSQL、Redis 和 OTLP Collector egress 分别约束到明确 namespace 与 Pod selector。生产环境必须通过 admission policy 或等价准入控制限制 `aegiscore.io/allow-user-service` 标签只能由受信任 namespace 或受控 workload 使用；仓库原生清单提供了 `deployments/k8s/user-services/admissionpolicy.yaml` 作为参考资产。

如果目标环境使用集群外 PostgreSQL、Redis 或 OTLP Collector，应在环境 values 中使用精确 `ipBlock.cidr` 或等价明确目的地覆盖对应 egress 规则。不得删除 `to` 字段恢复对任意目的地址开放 `5432`、`6379`、`4317` 或 `4318`。

外部 PostgreSQL 覆盖示例：

```yaml
networkPolicy:
  egress:
    - to:
        - ipBlock:
            cidr: 10.20.30.40/32
      ports:
        - protocol: TCP
          port: 5432
```

## Secret 键名

默认 Secret 名称为 `aegiscore-user-services-runtime`，默认键名如下：

- `AEGISCORE_AUTH_JWT_SECRET`
- `AEGISCORE_RESOURCES_POSTGRES_PRIMARY_DB_USERNAME`
- `AEGISCORE_RESOURCES_POSTGRES_PRIMARY_DB_PASSWORD`
- `AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_USERNAME`
- `AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_PASSWORD`

如果集群使用外部 Secret 管理器，应保持这些键名，或通过 `secret.keys` 显式覆盖。

`config` 使用最终 `AEGISCORE_SERVER_*` 与 `AEGISCORE_RESOURCES_*` 路径，Redis 使用统一 `TIMEOUT`，PostgreSQL pool 使用 `POOL_*`；平台时区只使用 `TZ`。日志写 stdout/stderr，tracing 启用后固定通过 OTLP 导出。trusted proxy 由集群入口边界负责，不是 chart 的应用配置。

chart 默认不配置或暴露 pprof。临时排障应通过额外受控环境变量设置 `PPROF_ENABLED=true`、`PPROF_ADDR=127.0.0.1:6060`，并使用 `kubectl port-forward`，不要把诊断端口加入常驻 Service。

## 渲染和验证

```bash
helm lint deployments/helm/aegiscore-user-services
helm template aegiscore-user-services deployments/helm/aegiscore-user-services \
  --values deployments/helm/aegiscore-user-services/values.yaml
```

渲染结果应包含 UID/GID/fsGroup `65532` 和 `terminationGracePeriodSeconds: 150`，且不包含容器内 shell healthcheck、Atlas apply 命令或 migration Job。

本地覆盖示例：

```bash
helm template aegiscore-user-services deployments/helm/aegiscore-user-services \
  --values deployments/helm/aegiscore-user-services/values.yaml \
  --values deployments/helm/aegiscore-user-services/values-local.yaml
```

## 发布流程

生产流水线应按顺序执行：

1. 创建或更新 `secret.existingSecret`。
2. 确认本 release 对应的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行完成；如包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，确认 DBA 权限或前置动作已处理。
3. 执行 RBAC seed Job，等待 Job 成功。
4. 执行 `helm upgrade --install aegiscore-user-services deployments/helm/aegiscore-user-services --values <env-values> --set rbacSeedJob.enabled=false`。
5. 等待 Deployment rollout 完成。

Helm 渲染的 RBAC seed Job 可由 GitOps 或 CI/CD 流水线分阶段应用。不要依赖普通服务副本启动时执行 migration，也不要混用新版 user-service 镜像和旧版自动执行 Atlas apply 的 Job 模板。

## 回滚边界

Deployment rollout 失败时，可回滚 Helm release 或回退镜像 tag。已经成功执行的 SQL migration 不应由 Helm rollback 隐式撤销；数据库回滚必须走 DBA 工单或受控发布平台流程。

如果在已有副本运行时重新执行 RBAC seed，需要滚动重启副本或触发在线 policy refresh，确保授权缓存及时收敛。
