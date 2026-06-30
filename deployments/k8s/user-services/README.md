# user-services Kubernetes 清单

本目录提供 user-service 的云厂商无关 Kubernetes 生产基线。运行时清单使用 `aegiscore-user-services:latest` 作为示例镜像，migration Job 使用 `aegiscore-user-services-migration:latest` 作为示例镜像；生产发布应替换为同一 release 的不可变 tag 或 digest。

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
| `migration-job.yaml` | Atlas migration Job |
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

3. 执行 migration Job：

   ```bash
   kubectl apply -f deployments/k8s/user-services/migration-job.yaml
   kubectl wait --for=condition=complete --timeout=15m job/aegiscore-user-services-migrate -n aegiscore
   ```

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

`kustomization.yaml` 聚合完整资源集合，适合验证和审查。生产首次发布应使用上面的分阶段命令，避免 Deployment 与前置 Job 同时创建。

## 探针和运行边界

- liveness probe：`GET /livez`
- readiness probe：`GET /readyz`
- startup probe：`GET /startupz`

Deployment 默认只启动 HTTP 服务，不设置 `RUN_MIGRATIONS=true`，运行时镜像不包含 Atlas。多副本生产发布必须使用独立 Atlas/migration 镜像的 migration Job 或 CI/CD release job。

## 失败诊断和回滚

- migration Job 失败时，停止 rollout，查看 `kubectl logs job/aegiscore-user-services-migrate -n aegiscore`。
- RBAC seed Job 失败时，停止 rollout，查看 `kubectl logs job/aegiscore-user-services-rbac-seed -n aegiscore`。
- Deployment rollout 失败时，查看 Pod 事件和日志，并回滚到上一镜像或上一套 manifest。
- 已成功应用的数据库 migration 不应通过 Deployment 回滚隐式撤销；按 Atlas migration 的独立流程处理。
- 回滚时不要混用新版无 Atlas 的 user-service 运行时镜像和旧版使用运行时镜像执行 migration 的 Job 模板。

如果在已有副本运行时重新执行 RBAC seed，需要滚动重启副本或触发在线 policy refresh，确保授权缓存及时收敛。

## 验证

无可用集群时，先执行离线渲染和 YAML 解析：

```bash
kubectl kustomize deployments/k8s/user-services > /tmp/aegiscore-user-services-k8s.yaml
ruby -e 'require "yaml"; docs = YAML.load_stream(File.read("/tmp/aegiscore-user-services-k8s.yaml")); abort("no docs") if docs.empty?'
```

有可用集群或可用 OpenAPI cache 时，执行 client dry-run：

```bash
kubectl apply --dry-run=client -k deployments/k8s/user-services
```

连接目标集群时可追加 server-side dry-run：

```bash
kubectl apply --dry-run=server -k deployments/k8s/user-services
```
