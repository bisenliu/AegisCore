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

运行时 Secret 使用 `AEGISCORE_AUTH_JWT_SECRET`、`AEGISCORE_RESOURCES_POSTGRES_USER_DB_USERNAME`、`AEGISCORE_RESOURCES_POSTGRES_USER_DB_PASSWORD`、`AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_USERNAME` 和 `AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_PASSWORD`。`secret.example.yaml` 的空密码只是键名示例，生产值必须由外部 Secret 管理器注入。

## 探针和运行边界

- liveness probe：`GET /livez`
- readiness probe：`GET /readyz`
- startup probe：`GET /startupz`

Deployment 默认只启动 HTTP 服务，不设置 `RUN_MIGRATIONS=true`，运行时镜像不包含 Atlas。多副本生产发布必须先确认数据库 SQL migration 已由 DBA 工单或受控发布平台执行完成。

非敏感配置使用 `AEGISCORE_SERVER_HTTP_*` 和 `AEGISCORE_RESOURCES_*`，敏感资源凭据使用同路径 Secret key。Redis 只有统一 `TIMEOUT`，PostgreSQL pool 使用 `POOL_*`。时区由平台 `TZ` 控制；日志只写 stdout/stderr；tracing 启用后固定使用 OTLP。应用不接收 trusted proxy 配置，代理信任和 forwarded headers 由 Ingress、gateway 或 service mesh 入口策略负责。

pprof 默认关闭且不由 Service 暴露。临时诊断时只在受控副本设置 `PPROF_ENABLED=true`、`PPROF_ADDR=127.0.0.1:6060`，再通过 `kubectl port-forward` 访问。

运行时镜像基于固定 digest 的 Distroless static nonroot，Deployment 和 RBAC seed Job 的 `runAsUser`、`runAsGroup`、`fsGroup` 均为 `65532`，并保持只读根文件系统、禁止提权、capabilities drop、RuntimeDefault seccomp 和 `/tmp` emptyDir。Kubernetes 探针使用 kubelet HTTP probe，不依赖容器内 shell 或原生 `healthcheck` 命令。

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
grep -q 'runAsUser: 65532' /tmp/aegiscore-user-services-k8s.yaml
grep -q 'runAsGroup: 65532' /tmp/aegiscore-user-services-k8s.yaml
grep -q 'fsGroup: 65532' /tmp/aegiscore-user-services-k8s.yaml
```

有可用集群或可用 OpenAPI cache 时，执行 client dry-run：

```bash
kubectl apply --dry-run=client -k deployments/k8s/user-services
```

连接目标集群时可追加 server-side dry-run：

```bash
kubectl apply --dry-run=server -k deployments/k8s/user-services
```
