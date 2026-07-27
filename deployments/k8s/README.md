# Kubernetes 部署

本目录承载 AegisCore 服务的 Kubernetes YAML。当前提交的生产基线位于 `user-service/`，用于部署运行时名称为 `aegiscore-user-service` 的用户服务。

## 目录

| 路径 | 作用 |
|---|---|
| `user-service/kustomization.yaml` | user-service 原生 YAML 入口 |
| `user-service/deployment.yaml` | HTTP 副本、探针、资源、安全上下文和滚动更新策略 |
| `user-service/rbac-seed-job.yaml` | RBAC 系统数据 seed 发布前置 Job |

## 发布顺序

生产 Kubernetes 发布必须按以下顺序执行：

1. 创建 namespace、ServiceAccount，并准备 Nacos namespace/group/dataId。
2. 确认本 release 对应的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行完成。
3. 执行 RBAC seed Job，并等待完成。
4. 创建或滚动更新 user-service Deployment、Service、PDB、HPA 和 NetworkPolicy。

普通 HTTP Deployment 默认不设置 `RUN_MIGRATIONS=true`，运行时镜像不包含 Atlas，不参与数据库 schema 变更。
`kustomization.yaml` 用于发现和验证完整资源集合；生产首次发布不要用一次性 `kubectl apply -k` 跳过 SQL migration 完成确认和 RBAC seed 等待步骤。

user-service 运行时镜像基于 Distroless static nonroot，Kubernetes Deployment 和 RBAC seed Job 的 `runAsUser`、`runAsGroup`、`fsGroup` 均为 `65532`。探针继续使用 kubelet `httpGet` 请求 `/livez`、`/readyz` 和 `/startupz`，不依赖容器内 shell 或 healthcheck 命令。

## 终止预算

user-service Deployment 默认 `terminationGracePeriodSeconds` 为 150 秒，用于覆盖应用默认 120 秒 Fx `runtime.lifecycle.stop_timeout` 和 30 秒平台余量。Fx Stop 是全部 `OnStop` hook 按逆注册顺序串行执行的进程级总预算；HTTP 25 秒、auth session purge workerpool 30 秒或 exporter I/O timeout 等局部预算不能替代该总预算。

应用完成请求排空、后台任务停止、tracing flush、datastore 关闭和 logger sync 后会立即退出，不会主动等待完整 150 秒。当前清单没有 `preStop`；后续新增或延长 `preStop` 时必须把执行上界计入 30 秒平台余量，余量不足时同步提高原生清单、Helm 默认值、配置一致性测试和说明。

## 配置边界

user-service 每个进程只通过 `AEGISCORE_SERVICE` 和 `AEGISCORE_NACOS_*` 定位 Nacos 配置来源。生产环境应在 Nacos 中维护 `base.yaml`、`resources.yaml`、`user-service.yaml` 等 dataId，这些 YAML 合成后必须同时包含运行时配置、资源地址和凭据，例如：

- `auth.jwt.secret`
- `resources.postgres.primary_db.username`
- `resources.postgres.primary_db.password`
- `resources.redis.cache_redis.mode`，枚举为 `cluster` 或 `standalone`
- `resources.redis.cache_redis.addrs`，`cluster` 模式使用，可只包含一个阿里云 Redis 集群 seed endpoint
- `resources.redis.cache_redis.addr`，`standalone` 模式使用；Redis DB 固定为 0 号库且不暴露配置项
- 可选 `resources.redis.cache_redis.username/password`
- 可选 `resources.redis.cache_redis.cluster.max_redirects`

超级管理员 bootstrap 临时密码只通过 `ADMIN_BOOTSTRAP_PASSWORD` 环境变量提供，不写入 Nacos 配置。

Nacos 合成配置是唯一运行配置来源，进程时区使用 `runtime.timezone`。日志由 stdout/stderr 采集，tracing 启用后固定通过 OTLP 导出。trusted proxy 策略属于 Ingress、gateway 或 service mesh 入口边界，不通过应用配置注入。

pprof 默认不渲染、不暴露。临时诊断应修改 Nacos 配置中的 `observability.pprof`，再使用 `kubectl port-forward`。

## 验证

无可用集群时，先执行离线渲染和 YAML 解析：

```bash
kubectl kustomize deployments/k8s/user-service > /tmp/aegiscore-user-service-k8s.yaml
ruby -e 'require "yaml"; docs = YAML.load_stream(File.read("/tmp/aegiscore-user-service-k8s.yaml")); abort("no docs") if docs.empty?'
grep -q 'atlas migrate apply\|migrate apply' /tmp/aegiscore-user-service-k8s.yaml && exit 1 || true
grep -q 'runAsUser: 65532' /tmp/aegiscore-user-service-k8s.yaml
grep -q 'runAsGroup: 65532' /tmp/aegiscore-user-service-k8s.yaml
grep -q 'fsGroup: 65532' /tmp/aegiscore-user-service-k8s.yaml
grep -q 'terminationGracePeriodSeconds: 150' /tmp/aegiscore-user-service-k8s.yaml
```

有可用集群或可用 OpenAPI cache 时，执行 client dry-run：

```bash
kubectl apply --dry-run=client -k deployments/k8s/user-service
```

连接到目标集群时，也可以执行 server-side dry-run：

```bash
kubectl apply --dry-run=server -k deployments/k8s/user-service
```

## 观测资产

Prometheus/Grafana dashboard、alert rule 示例和 runbook 链接位于 `deployments/observability/`。本目录只提供云厂商无关的 Kubernetes 运行时和 RBAC seed 发布前置 Job，不默认提交 ServiceMonitor、PodMonitor 或 Ingress。
