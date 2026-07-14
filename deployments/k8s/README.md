# Kubernetes 部署

本目录承载 AegisCore 服务的 Kubernetes YAML。当前提交的生产基线位于 `user-services/`，用于部署运行时名称为 `aegiscore-user-services` 的用户服务。

## 目录

| 路径 | 作用 |
|---|---|
| `user-services/kustomization.yaml` | user-service 原生 YAML 入口 |
| `user-services/deployment.yaml` | HTTP 副本、探针、资源、安全上下文和滚动更新策略 |
| `user-services/rbac-seed-job.yaml` | RBAC 系统数据 seed 发布前置 Job |
| `user-services/secret.example.yaml` | Secret 键名示例；不参与默认 kustomization |

## 发布顺序

生产 Kubernetes 发布必须按以下顺序执行：

1. 创建 namespace、ServiceAccount、配置和外部 Secret。
2. 确认本 release 对应的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行完成。
3. 执行 RBAC seed Job，并等待完成。
4. 创建或滚动更新 user-service Deployment、Service、PDB、HPA 和 NetworkPolicy。

普通 HTTP Deployment 默认不设置 `RUN_MIGRATIONS=true`，运行时镜像不包含 Atlas，不参与数据库 schema 变更。
`kustomization.yaml` 用于发现和验证完整资源集合；生产首次发布不要用一次性 `kubectl apply -k` 跳过 SQL migration 完成确认和 RBAC seed 等待步骤。

user-service 运行时镜像基于 Distroless static nonroot，Kubernetes Deployment 和 RBAC seed Job 的 `runAsUser`、`runAsGroup`、`fsGroup` 均为 `65532`。探针继续使用 kubelet `httpGet` 请求 `/livez`、`/readyz` 和 `/startupz`，不依赖容器内 shell 或 healthcheck 命令。

## Secret 边界

`secret.example.yaml` 只说明必需键名，不应直接用于生产。生产环境应由部署系统、GitOps Secret、密钥管理系统或 CI/CD 注入以下键：

- `AEGISCORE_AUTH_JWT_SECRET`
- `AEGISCORE_RESOURCES_POSTGRES_USER_DB_USERNAME`
- `AEGISCORE_RESOURCES_POSTGRES_USER_DB_PASSWORD`
- `AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_USERNAME`
- `AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_PASSWORD`

ConfigMap 使用 `AEGISCORE_SERVER_HTTP_*`、`AEGISCORE_RESOURCES_*` 最终路径，进程时区使用标准 `TZ`。日志由 stdout/stderr 采集，tracing 启用后固定通过 OTLP 导出。trusted proxy 策略属于 Ingress、gateway 或 service mesh 入口边界，不通过应用配置注入。

pprof 默认不渲染、不暴露。临时诊断应在受控副本中设置 `PPROF_ENABLED=true`、`PPROF_ADDR=127.0.0.1:6060`，再使用 `kubectl port-forward`。

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

连接到目标集群时，也可以执行 server-side dry-run：

```bash
kubectl apply --dry-run=server -k deployments/k8s/user-services
```

## 观测资产

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`。本目录只提供云厂商无关的 Kubernetes 运行时和 RBAC seed 发布前置 Job，不默认提交 ServiceMonitor、PodMonitor 或 Ingress。
