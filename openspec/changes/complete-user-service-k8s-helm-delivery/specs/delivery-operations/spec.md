## ADDED Requirements

### Requirement: 用户服务 Kubernetes 生产清单

系统 MUST 为 user-service 提供云厂商无关的 Kubernetes 生产清单，并覆盖运行副本、服务发现、配置引用、Secret 引用、安全上下文、资源配额、探针、滚动更新、PDB、HPA 和 NetworkPolicy。

#### Scenario: 渲染生产清单

- **WHEN** 协作者查看或应用 `deployments/k8s/user-services/` 下的清单
- **THEN** 清单 MUST 包含 user-service 的 `Deployment`、`Service`、配置资源、Secret 引用、`ServiceAccount`、必要 RBAC、`PodDisruptionBudget`、`HorizontalPodAutoscaler` 和 `NetworkPolicy`

#### Scenario: HTTP 副本探针

- **WHEN** user-service Pod 由 Kubernetes Deployment 启动
- **THEN** liveness probe MUST 指向 `GET /livez`，readiness probe MUST 指向 `GET /readyz`，startup probe MUST 指向 `GET /startupz`

#### Scenario: 运行副本不执行 migration

- **WHEN** user-service Deployment 启动普通 HTTP 副本
- **THEN** Pod 环境变量 MUST NOT 设置 `RUN_MIGRATIONS=true`，副本 MUST 只启动 HTTP 服务

#### Scenario: Pod 安全和资源边界

- **WHEN** user-service Pod 被调度
- **THEN** Pod 和容器 securityContext MUST 默认使用非 root、只读根文件系统、禁止特权升级和收敛 Linux capabilities，并且容器 MUST 设置 CPU 与内存 requests/limits

### Requirement: Kubernetes 发布前置作业

系统 MUST 为 user-service Kubernetes 发布提供独立 migration Job 和 RBAC seed Job，并明确它们先于 HTTP Deployment rollout 执行。

#### Scenario: Migration Job

- **WHEN** 发布流水线执行 user-service 数据库迁移
- **THEN** migration Job MUST 使用当前发布镜像执行 `/app/user-service/scripts/migrate-apply.sh`，并 MUST 通过 Secret 或部署系统注入 `DATABASE_URL`

#### Scenario: RBAC seed Job

- **WHEN** migration Job 成功完成后
- **THEN** RBAC seed Job MUST 使用当前发布镜像执行 `rbac seed`，并 MUST 支持 `--reactivate-system` 与 `--sync-system-bindings` 选项

#### Scenario: 发布顺序

- **WHEN** user-service 发布到 Kubernetes 环境
- **THEN** 发布流程 MUST 等待 migration Job 成功，再等待 RBAC seed Job 成功，最后创建或滚动更新 HTTP Deployment

#### Scenario: 前置作业失败

- **WHEN** migration Job 或 RBAC seed Job 失败
- **THEN** 发布流程 MUST 停止 HTTP Deployment rollout，并保留 Job 日志用于诊断

### Requirement: 用户服务 Helm chart

系统 MUST 为 `aegiscore-user-services` 提供 Helm chart，用 values 模板化同等 Kubernetes 交付能力，并保持默认值不包含真实生产 Secret。

#### Scenario: Helm chart 元数据和 values

- **WHEN** 协作者查看 `deployments/helm/aegiscore-user-services/`
- **THEN** chart MUST 包含 `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例，并 MUST 暴露 image、service、config、Secret 引用、resources、probes、autoscaling、PDB、NetworkPolicy、migration Job、RBAC seed Job 和 rollout 配置

#### Scenario: Helm 渲染 Secret 引用

- **WHEN** 协作者执行 `helm template` 渲染 chart
- **THEN** 模板 MUST 通过 `existingSecret` 或等价 values 引用外部 Secret，不得默认渲染真实敏感值

#### Scenario: Helm 渲染发布作业

- **WHEN** Helm values 启用 migration Job 和 RBAC seed Job
- **THEN** chart MUST 渲染使用当前发布镜像的两个独立 Job，并保持 Job command 与 Kubernetes 原生清单一致

#### Scenario: Helm Deployment 默认行为

- **WHEN** Helm chart 渲染 user-service Deployment
- **THEN** Deployment MUST 默认不设置 `RUN_MIGRATIONS=true`，并 MUST 渲染 `/livez`、`/readyz`、`/startupz` 探针和资源 requests/limits

### Requirement: Kubernetes 和 Helm 验证说明

系统 MUST 为 user-service Kubernetes 与 Helm 资产提供可执行的验证说明，覆盖模板渲染、YAML/schema 检查和发布顺序检查。

#### Scenario: 验证原生清单

- **WHEN** 协作者修改 `deployments/k8s/user-services/`
- **THEN** tasks 或 README MUST 指明用于校验 YAML、Kubernetes schema 或 server-side dry-run 的命令

#### Scenario: 验证 Helm chart

- **WHEN** 协作者修改 `deployments/helm/aegiscore-user-services/`
- **THEN** tasks 或 README MUST 指明 `helm lint` 和 `helm template` 的验证命令，并说明如何检查 migration Job、RBAC seed Job 和 Deployment 的关键字段

#### Scenario: 架构文档验证

- **WHEN** 本次部署规格或 OPSX artifacts 变更完成
- **THEN** 协作者 MUST 执行 `make user-service-architecture-lint`，确保中文文档和 OpenSpec 结构约束通过
