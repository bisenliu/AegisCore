# user-services Kubernetes 清单

本目录提供 user-service 的云厂商无关 Kubernetes 生产基线。运行时清单使用 `aegiscore-user-services:latest` 作为示例镜像；生产发布应替换为不可变 tag 或 digest。

## 资源

| 文件 | 作用 |
|---|---|
| `kustomization.yaml` | 聚合默认可应用资源 |
| `namespace.yaml` | `aegiscore` namespace |
| `serviceaccount.yaml` | 运行时 ServiceAccount、空 Role 和 RoleBinding |
| `configmap.yaml` | 非敏感运行时配置 |
| `secret.example.yaml` | Secret 键名示例，不在默认 kustomization 中应用 |
| `deployment.yaml` | HTTP 副本、探针、资源、安全上下文和 rollout 策略 |
| `service.yaml` | ClusterIP Service |
| `rbac-seed-job.yaml` | RBAC seed Job |
| `pdb.yaml` | PodDisruptionBudget |
| `hpa.yaml` | HorizontalPodAutoscaler |
| `networkpolicy.yaml` | 最小 ingress/egress 网络意图 |

## 发布流程

1. 应用前置资源：

   ```bash
   kubectl apply -f deployments/k8s/user-services/namespace.yaml
   kubectl apply -f deployments/k8s/user-services/serviceaccount.yaml
   kubectl apply -f deployments/k8s/user-services/configmap.yaml
   ```

2. 准备 Secret：

   ```bash
   kubectl apply -f deployments/k8s/user-services/secret.example.yaml
   ```

   上述命令只适合本地或临时环境。生产环境应改为由外部 Secret 流程创建同名 Secret，并替换所有示例值。

3. 确认数据库 SQL migration 已执行：

   将已提交的 SQL migration 和权限要求提交到 DBA 工单或受控发布平台，等待目标环境执行完成。若 SQL 包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，生产库可能需要 DBA 权限或前置动作。

4. 执行 RBAC seed Job：

   ```bash
   kubectl apply -f deployments/k8s/user-services/rbac-seed-job.yaml
   kubectl wait --for=condition=complete --timeout=15m job/aegiscore-user-services-rbac-seed -n aegiscore
   ```

5. 应用运行时资源：

   ```bash
   kubectl apply -f deployments/k8s/user-services/service.yaml
   kubectl apply -f deployments/k8s/user-services/deployment.yaml
   kubectl apply -f deployments/k8s/user-services/pdb.yaml
   kubectl apply -f deployments/k8s/user-services/hpa.yaml
   kubectl apply -f deployments/k8s/user-services/networkpolicy.yaml
   kubectl rollout status deployment/aegiscore-user-services -n aegiscore
   ```

`kustomization.yaml` 聚合完整资源集合，适合验证和审查。生产首次发布应使用上面的分阶段命令，避免 Deployment 在数据库 SQL migration 和 RBAC seed 完成前创建。

## 探针和运行边界

- liveness probe：`GET /livez`
- readiness probe：`GET /readyz`
- startup probe：`GET /startupz`

Deployment 默认只启动 HTTP 服务，不设置 `RUN_MIGRATIONS=true`，运行时镜像不包含 Atlas。多副本生产发布必须先确认数据库 SQL migration 已由 DBA 工单或受控发布平台执行完成。

## 失败诊断和回滚

- 数据库 SQL migration 未确认完成时，停止 rollout，查看 DBA 工单、发布平台记录或等价受控执行记录。
- RBAC seed Job 失败时，停止 rollout，查看 `kubectl logs job/aegiscore-user-services-rbac-seed -n aegiscore`。
- Deployment rollout 失败时，查看 Pod 事件和日志，并回滚到上一镜像或上一套 manifest。
- 已成功执行的数据库 SQL migration 不应通过 Deployment 回滚隐式撤销；按 DBA 工单或受控发布平台的数据库回滚流程处理。
- 回滚时不要混用新版 user-service 运行时镜像和旧版包含自动 Atlas apply 的 Job 模板。

如果在已有副本运行时重新执行 RBAC seed，需要滚动重启副本或触发在线 policy refresh，确保授权缓存及时收敛。

## 验证

无可用集群时，先执行离线渲染和 YAML 解析：

```bash
kubectl kustomize deployments/k8s/user-services > /tmp/aegiscore-user-services-k8s.yaml
ruby -e 'require "yaml"; docs = YAML.load_stream(File.read("/tmp/aegiscore-user-services-k8s.yaml")); abort("no docs") if docs.empty?'
grep -q 'atlas migrate apply\|migrate apply' /tmp/aegiscore-user-services-k8s.yaml && exit 1 || true
```

有可用集群或可用 OpenAPI cache 时，执行 client dry-run：

```bash
kubectl apply --dry-run=client -k deployments/k8s/user-services
```

连接目标集群时可追加 server-side dry-run：

```bash
kubectl apply --dry-run=server -k deployments/k8s/user-services
```
