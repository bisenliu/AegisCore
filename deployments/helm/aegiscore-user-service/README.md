# aegiscore-user-service Helm Chart

本 chart 为 user-service 渲染云厂商无关的 Kubernetes 资源，包括 HTTP Deployment、Service、ServiceAccount、RBAC、PDB、HPA 和 NetworkPolicy。RBAC seed Job 仅在受控 seed 阶段显式开启渲染。运行配置由 `AEGISCORE_*` 环境变量定位 Nacos，再按 dataId 加载分层 YAML。

## Values

| 值 | 作用 |
|---|---|
| `image.ref` | user-service 不可变发布镜像，生产必须使用 digest 或 `sha-<commit>` tag |
| `nacos` | Nacos Service、Kubernetes namespace、配置 namespace/group 与扩展环境变量 |
| `deployment.terminationGracePeriodSeconds` | Fx Stop 总预算与平台余量对应的 Pod 终止宽限期 |
| `resources` | HTTP 副本 requests/limits |
| `probes` | `/livez`、`/readyz`、`/startupz` 探针配置 |
| `autoscaling` | HPA 配置 |
| `pdb` | PodDisruptionBudget 配置 |
| `networkPolicy` | ingress 与 Nacos 之外的 additional egress 网络意图 |
| `rbacSeedJob` | RBAC seed Job 配置，默认关闭以保证最终 runtime manifest 不包含 Job |

默认 `values.yaml` 不包含真实敏感值，也不设置 `RUN_MIGRATIONS=true`。普通 user-service 镜像不包含 Atlas，chart 不渲染自动执行 Atlas apply 的 migration Job。生产环境必须提前在 Nacos 写入 `base.yaml`、`resources.yaml` 和 `user-service.yaml` 等 dataId，并确保数据库 SQL migration 已通过 DBA 工单或受控发布平台执行完成。生产发布必须显式传入 `image.ref`，且该值必须是当前 release 的不可变镜像引用，不得使用 `latest`。

`image.ref` 同时用于 Deployment 和显式开启的 RBAC seed Job。CI/CD 必须先构建并扫描同一 user-service 镜像工件，再将 digest 或 `sha-<commit>` tag 传给 Helm；不得扫描一个镜像、发布另一个镜像。生产 seed 阶段必须覆盖 `rbacSeedJob.nameSuffix` 为 release 唯一值，避免固定名 Job 在升级时触发不可变 Pod template 冲突。

默认 `podSecurityContext.runAsUser`、`runAsGroup` 和 `fsGroup` 均为 `65532`，与 Distroless static nonroot 运行时镜像身份一致。Deployment 和 RBAC seed Job 使用同一数值身份，保持只读根文件系统、禁止提权、capabilities drop 和 RuntimeDefault seccomp；Kubernetes 探针继续由 kubelet 通过 HTTP 访问 `/livez`、`/readyz`、`/startupz`。

默认 `deployment.terminationGracePeriodSeconds` 为 150 秒，其中 120 秒覆盖 Fx `app.Stop()` 对全部 `OnStop` hook 的逆注册顺序串行总预算，30 秒保留给 kubelet 调度、信号传递、受控 `preStop` 和网络抖动。HTTP、workerpool 或 exporter 的局部 timeout 不能替代 Fx 总预算；正常关闭完成后 Pod 会立即退出，不会等待完整 150 秒。环境 values 覆盖该值或新增、延长 `preStop` 时，发布方必须保持 grace 至少覆盖 Fx Stop 总预算和 30 秒平台余量，并同步验证原生 Kubernetes 与 Helm 的默认契约未漂移。

## NetworkPolicy 安全边界

默认 `networkPolicy` 仅允许 `ingress-system` namespace 中带 `aegiscore.io/allow-user-service: "true"` 的受控上游访问 user-service HTTP 端口。Nacos egress 直接从 `nacos.server.namespace`、`nacos.server.port` 和 `nacos.server.podSelector` 生成，保证 Service DNS 与网络策略使用同一组定位信息；PostgreSQL、Redis 和 OTLP Collector egress 分别约束到明确 namespace 与 Pod selector。生产环境必须通过 admission policy 或等价准入控制限制 `aegiscore.io/allow-user-service` 标签只能由受信任 namespace 或受控 workload 使用；仓库原生清单提供了 `deployments/k8s/user-service/admissionpolicy.yaml` 作为参考资产。

如果目标环境使用集群外 PostgreSQL、Redis 或 OTLP Collector，应在环境 values 的 `networkPolicy.additionalEgress` 中使用精确 `ipBlock.cidr` 或等价明确目的地覆盖对应规则。不得删除 `to` 字段恢复对任意目的地址开放 `5432`、`6379`、`4317` 或 `4318`。

外部 PostgreSQL 覆盖示例：

```yaml
networkPolicy:
  additionalEgress:
    - to:
        - ipBlock:
            cidr: 10.20.30.40/32
      ports:
        - protocol: TCP
          port: 5432
```

## Nacos 配置来源

默认 `nacos` 结构化 values 同时渲染 Deployment、显式开启的 RBAC seed Job 的来源选择变量和 Nacos NetworkPolicy egress：

- `AEGISCORE_SERVICE=user-service`
- `AEGISCORE_NACOS_ADDR=nacos.prod.svc.cluster.local:8848`
- `AEGISCORE_NACOS_NAMESPACE=prod`
- `AEGISCORE_NACOS_GROUP=AEGISCORE`

默认 dataId 顺序为 `base.yaml`、`resources.yaml`、`user-service.yaml`，后面的 YAML 覆盖前面的同名 scalar，map 递归合并，list 整体替换。敏感字段第一阶段可以放在 Nacos，但 `config render` 默认脱敏 `auth.jwt.secret`、`resources.redis.*.password` 和 `resources.postgres.*.password`；生产建议后续升级到 Kubernetes Secret、Vault 或 KMS。

user-service 使用 Nacos v3 Client HTTP API 读取配置；默认网络策略只需允许访问 `8848`。

`nacos.server.namespace` 同时参与集群内 Service DNS 和 NetworkPolicy namespace selector，`nacos.configNamespace` 则表示 Nacos 自身的配置 namespace ID，两者语义不同，环境 values 应分别显式维护。认证凭据或 timeout 可通过 `nacos.extraEnv` 追加，并优先使用 Secret 引用提供敏感值。

超级管理员 bootstrap 临时密码只通过 `ADMIN_BOOTSTRAP_PASSWORD` 环境变量提供，不写入 Nacos 配置。

进程时区使用 `runtime.timezone`。日志写 stdout/stderr，tracing 启用后固定通过 OTLP 导出。trusted proxy 通过 Nacos `server.http.trusted_proxies` 显式配置真实入口代理 IP/CIDR；chart 的 NetworkPolicy 只限制网络来源，入口 Ingress、gateway 或 service mesh 仍必须覆盖或重建 forwarded headers。

chart 默认不配置或暴露 pprof。临时排障应修改 Nacos 中的 `observability.pprof`，并使用 `kubectl port-forward`，不要把诊断端口加入常驻 Service。

## 渲染和验证

```bash
helm lint deployments/helm/aegiscore-user-service \
  --set-string image.ref=aegiscore-user-service:sha-local
helm template aegiscore-user-service deployments/helm/aegiscore-user-service \
  --values deployments/helm/aegiscore-user-service/values.yaml \
  --set-string image.ref=aegiscore-user-service:sha-local
```

默认渲染结果应包含 UID/GID/fsGroup `65532`、`terminationGracePeriodSeconds: 150` 和 `AEGISCORE_NACOS_*` 环境变量，且不包含 RBAC seed Job、容器内 shell healthcheck、Atlas apply 命令、`--config` 参数或 migration Job。seed 阶段可追加 `--set rbacSeedJob.enabled=true --set-string rbacSeedJob.nameSuffix=rbac-seed-<release-id>` 单独渲染 Job。

本地覆盖示例：

```bash
helm template aegiscore-user-service deployments/helm/aegiscore-user-service \
  --values deployments/helm/aegiscore-user-service/values.yaml \
  --values deployments/helm/aegiscore-user-service/values-local.yaml
```

## 发布流程

生产流水线应按顺序执行：

1. 创建或更新目标 Nacos namespace/group/dataId。
2. 构建、推送、扫描并记录当前 release 的 user-service 不可变镜像引用。生产基线使用 `registry.example.com/aegiscore/user-service@sha256:<64hex>`；如保留 commit tag，只允许 `registry.example.com/aegiscore/user-service:sha-<40hex>`，并要求 registry 对该 tag 启用 immutability。
3. 确认本 release 对应的已提交 SQL migration 已通过 DBA 工单或受控发布平台执行完成；如包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，确认 DBA 权限或前置动作已处理。
4. 使用同一个 `image.ref` 和 release 唯一 `rbacSeedJob.nameSuffix` 执行 RBAC seed Job，等待 Job 成功。
5. 执行 `helm upgrade --install aegiscore-user-service deployments/helm/aegiscore-user-service --values <env-values> --set-string image.ref=<immutable-ref>`，保持默认 `rbacSeedJob.enabled=false`。
6. 等待 Deployment rollout 完成。

Helm 渲染的 RBAC seed Job 可由 GitOps 或 CI/CD 流水线分阶段应用。任一 migration 确认或 seed 阶段失败时，不得应用新版 Deployment。最终 runtime manifest 必须不包含 seed Job。不要依赖普通服务副本启动时执行 migration，也不要混用新版 user-service 镜像和旧版自动执行 Atlas apply 的 Job 模板。

## 回滚边界

Deployment rollout 失败时，可回滚 Helm release 或回退到上一版已记录的不可变 `image.ref`。回滚不得改回 `latest`，也不得依赖 registry 中当前 tag 指向。已经成功执行的 SQL migration 不应由 Helm rollback 隐式撤销；数据库回滚必须走 DBA 工单或受控发布平台流程。

如果在已有副本运行时重新执行 RBAC seed，需要滚动重启副本或触发在线 policy refresh，确保授权缓存及时收敛。
