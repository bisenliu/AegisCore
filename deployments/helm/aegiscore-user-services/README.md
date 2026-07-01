# aegiscore-user-services Helm Chart

本 chart 为 user-service 渲染云厂商无关的 Kubernetes 资源，包括 HTTP Deployment、Service、ConfigMap、ServiceAccount、RBAC、RBAC seed Job、PDB、HPA 和 NetworkPolicy。

## Values

| 值 | 作用 |
|---|---|
| `image.repository`、`image.tag` | user-service 发布镜像 |
| `config` | 非敏感 `AEGISCORE_*` 运行时配置 |
| `secret.existingSecret` | 外部 Secret 名称；chart 只引用不渲染真实 Secret |
| `secret.keys.*` | Secret 键名映射 |
| `resources` | HTTP 副本 requests/limits |
| `probes` | `/livez`、`/readyz`、`/startupz` 探针配置 |
| `autoscaling` | HPA 配置 |
| `pdb` | PodDisruptionBudget 配置 |
| `networkPolicy` | ingress/egress 网络意图 |
| `rbacSeedJob` | RBAC seed Job 配置 |

默认 `values.yaml` 不包含真实敏感值，也不设置 `RUN_MIGRATIONS=true`。普通 user-service 镜像不包含 Atlas，chart 不渲染自动执行 Atlas apply 的 migration Job。生产环境必须提前创建 `secret.existingSecret` 指向的 Secret，并确保数据库 SQL migration 已通过 DBA 工单或受控发布平台执行完成。

## Secret 键名

默认 Secret 名称为 `aegiscore-user-services-runtime`，默认键名如下：

- `AEGISCORE_AUTH_JWT_SECRET`
- `AEGISCORE_POSTGRES_USER_DB_USERNAME`
- `AEGISCORE_POSTGRES_USER_DB_PASSWORD`
- `AEGISCORE_REDIS_CACHE_REDIS_USERNAME`
- `AEGISCORE_REDIS_CACHE_REDIS_PASSWORD`

如果集群使用外部 Secret 管理器，应保持这些键名，或通过 `secret.keys` 显式覆盖。

## 渲染和验证

```bash
helm lint deployments/helm/aegiscore-user-services
helm template aegiscore-user-services deployments/helm/aegiscore-user-services \
  --values deployments/helm/aegiscore-user-services/values.yaml
```

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
