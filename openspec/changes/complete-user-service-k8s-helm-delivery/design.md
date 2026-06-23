## Context

`deployments/k8s/user-services/` 和 `deployments/helm/aegiscore-user-services/` 当前只有占位 README。仓库已有 Docker 镜像构建入口、Compose 本地发布顺序、Atlas migration 脚本、RBAC CLI、健康探针和独立观测资产，因此 Kubernetes 与 Helm 资产应延续这些既有边界。

本变更影响 `deployments/` 和 OpenSpec 文档，不改变 Go 业务代码、HTTP API、OpenAPI 生成物、Ent schema 或 Atlas migration 内容。生产发布顺序必须保持为 migration Job 成功、RBAC seed Job 成功、HTTP Deployment rollout。

## Goals / Non-Goals

**Goals:**

- 为 user-service 提供可直接审查的 Kubernetes 原生清单，覆盖工作负载、服务发现、配置、Secret 引用、发布 Job、安全上下文、资源限制、探针、PDB、HPA、NetworkPolicy 和 rollout 策略。
- 为 `aegiscore-user-services` 提供 Helm chart，支持通过 values 渲染同等 Kubernetes 能力，并能复用当前发布镜像执行 migration 与 RBAC seed。
- 明确 Secret、配置、发布顺序、验证命令和回滚注意事项，避免普通服务副本默认执行 migration。
- 保持云厂商无关，不引入特定 Ingress Controller、ServiceMonitor、ExternalSecret 或托管数据库资源作为默认依赖。

**Non-Goals:**

- 不新增或修改 user-service 的业务 API、鉴权逻辑、RBAC 策略语义或数据库 schema。
- 不把 Prometheus/Grafana 观测资产封装进 Helm chart；现有 `deployments/observability/` 继续独立维护。
- 不在 Kubernetes 清单中提交真实密钥、生产域名或云厂商专有资源。
- 不要求 Helm hook 作为唯一发布机制；chart 需要支持普通 Job 渲染，具体平台可在流水线中编排执行顺序。

## Decisions

### Decision: 使用独立 Job 表达 migration 和 RBAC seed

Kubernetes 清单和 Helm chart 都提供独立 `Job`：migration Job 使用当前发布镜像执行 `/app/user-service/scripts/migrate-apply.sh`，RBAC seed Job 使用 `/app/user-service/bin/user-services rbac --config /app/user-service/configs/config.yaml seed --reactivate-system --sync-system-bindings`。

理由：这与 Compose 的 one-shot 服务和既有 README 约束一致，能避免多副本 Deployment 在启动时争抢 Atlas migration lock。备选方案是在 Deployment entrypoint 中设置 `RUN_MIGRATIONS=true`，但该模式只适合简单部署或兼容场景，不作为生产默认。

### Decision: Secret 通过引用注入，不在 chart 或清单内生成真实 Secret

资产提供 `Secret` 引用边界和示例键名，例如 `DATABASE_URL`、`AEGISCORE_AUTH_JWT_SECRET`、PostgreSQL 配置、Redis 地址和按需的管理员引导参数。默认模板只引用已有 Secret 或使用示例 Secret manifest 占位，不提交真实敏感值。

理由：生产 Secret 生命周期通常由部署平台、密钥管理系统或 GitOps 外部流程控制。备选方案是在 Helm values 中内联敏感值并渲染 Secret，这会增加泄漏风险，不作为默认能力。

### Decision: Deployment 默认只启动 HTTP 服务

Deployment 复用容器默认 CMD 启动 `serve`，显式不设置 `RUN_MIGRATIONS=true`。Pod 暴露 `/livez`、`/readyz`、`/startupz` 三类探针，使用 resources、securityContext、rollingUpdate、terminationGracePeriod 和 Pod anti-affinity 或 topology spread 约束提升 rollout 稳定性。

理由：运行时副本应只承载 HTTP 服务职责。备选方案是让服务副本负责发布前置任务，会放大 rollout 失败和重复执行风险。

### Decision: Helm chart 与原生 YAML 保持同一语义

原生 YAML 提供清晰、可直接阅读的基线；Helm chart 提供同一资源集合的模板化版本。两者共享命名、label、Secret key、探针路径、Job command 和默认资源策略。Helm values 暴露最小必要配置，不把业务配置拆成过细模板逻辑。

理由：原生 YAML 方便审查和非 Helm 环境复用，Helm chart 方便环境化发布。备选方案是只维护 Helm chart，但会降低无 Helm 平台的可读性和落地速度。

### Decision: 默认资源保持云厂商无关

默认不创建 Ingress、ServiceMonitor、PodMonitor、ExternalSecret、Certificate、云负载均衡注解或托管数据库资源。需要这些能力时，后续 change 再明确平台约束。

理由：当前仓库的部署资产定位是通用 Kubernetes 和 Helm。备选方案是一次性封装完整生产平台，但会过早绑定云厂商和集群插件。

## 风险和权衡

- 原生 YAML 与 Helm template 可能出现 drift。缓解方式：tasks 中要求同时渲染 Helm、审查关键字段，并在 README 中说明两套资产的同步边界。
- Job 顺序在纯 Kubernetes YAML 中无法自动表达跨阶段应用的串行依赖。缓解方式：README 明确等待 Job 完成的发布流程；Helm values 保留 Job 启用边界，流水线负责顺序编排。
- 默认 NetworkPolicy 在未安装 CNI policy enforcement 的集群中不生效。缓解方式：文档说明其为最小网络意图，验证以 manifest 渲染和策略字段为准，不声称替代集群安全治理。
- 资源 requests/limits 不适合所有环境。缓解方式：提供保守默认值和 values 覆盖入口，生产环境必须按实际容量调整。
- RBAC seed Job 与运行中副本的 policy cache 同步存在时序差异。缓解方式：发布顺序要求 seed 在 rollout 前完成；如果对运行中副本执行 seed，README 说明需要滚动重启或触发在线 policy refresh。

## Migration Plan

1. 新增 Kubernetes 原生清单和 Helm chart 文件，保持 `RUN_MIGRATIONS` 默认关闭。
2. 新增 README 发布流程：创建或引用 Secret，运行 migration Job，等待成功，运行 RBAC seed Job，等待成功，再 rollout Deployment。
3. 使用 `helm template` 渲染 chart，并对原生 YAML 与 Helm 输出执行 YAML/schema lint 或 dry-run 检查。
4. 发布失败时，先查看 migration Job 和 seed Job 日志；migration 失败不得继续 rollout。Deployment rollout 失败时回滚到上一镜像或上一 Helm release，数据库 migration 回滚按已提交 Atlas migration 的独立流程处理。

## Open Questions

- 是否在实现阶段引入仓库级 `make user-service-helm-template` 或 `make user-service-k8s-lint` 目标，取决于本地工具链可用性和是否需要固定 CI 入口。
- 是否提供 Ingress 或 ServiceMonitor 示例留给后续平台化 change 决定；本次默认不包含。
