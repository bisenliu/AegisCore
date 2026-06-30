# Kubernetes 部署

本目录承载 AegisCore 服务的 Kubernetes YAML。当前提交的生产基线位于 `user-services/`，用于部署运行时名称为 `aegiscore-user-services` 的用户服务。

## 目录

| 路径 | 作用 |
|---|---|
| `user-services/kustomization.yaml` | user-service 原生 YAML 入口 |
| `user-services/deployment.yaml` | HTTP 副本、探针、资源、安全上下文和滚动更新策略 |
| `user-services/migration-job.yaml` | Atlas migration 发布前置 Job |
| `user-services/rbac-seed-job.yaml` | RBAC 系统数据 seed 发布前置 Job |
| `user-services/secret.example.yaml` | Secret 键名示例；不参与默认 kustomization |

## 发布顺序

生产 Kubernetes 发布必须按以下顺序执行：

1. 创建 namespace、ServiceAccount、配置和外部 Secret。
2. 执行 migration Job，并等待完成。
3. 执行 RBAC seed Job，并等待完成。
4. 创建或滚动更新 user-service Deployment、Service、PDB、HPA 和 NetworkPolicy。

普通 HTTP Deployment 默认不设置 `RUN_MIGRATIONS=true`，运行时镜像不包含 Atlas，不参与 Atlas migration lock 竞争。
migration Job 使用独立 Atlas/migration 镜像，生产发布应确保 migration 镜像与 user-service 镜像来自同一 release。
`kustomization.yaml` 用于发现和验证完整资源集合；生产首次发布不要用一次性 `kubectl apply -k` 跳过 Job 等待步骤。

## Secret 边界

`secret.example.yaml` 只说明必需键名，不应直接用于生产。生产环境应由部署系统、GitOps Secret、密钥管理系统或 CI/CD 注入以下键：

- `DATABASE_URL`
- `AEGISCORE_AUTH_JWT_SECRET`
- `AEGISCORE_POSTGRES_USER_DB_USERNAME`
- `AEGISCORE_POSTGRES_USER_DB_PASSWORD`
- `AEGISCORE_REDIS_CACHE_REDIS_USERNAME`
- `AEGISCORE_REDIS_CACHE_REDIS_PASSWORD`

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

连接到目标集群时，也可以执行 server-side dry-run：

```bash
kubectl apply --dry-run=server -k deployments/k8s/user-services
```

## 观测资产

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`。本目录只提供云厂商无关的 Kubernetes 运行时和发布前置 Job，不默认提交 ServiceMonitor、PodMonitor 或 Ingress。
